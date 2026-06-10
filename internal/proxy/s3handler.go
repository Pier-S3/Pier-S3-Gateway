package proxy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"pier-s3/internal/acl"
	"pier-s3/internal/auth"
)

// proxyForwarder forwards an authorized request to the S3 backend and streams
// the response back. *PassThrough satisfies it; tests substitute a fake.
type proxyForwarder interface {
	Forward(w http.ResponseWriter, r *http.Request)
}

// S3Handler handles proxied S3 requests with authentication and authorization.
type S3Handler struct {
	verifier  auth.TokenVerifier
	backend   S3Backend
	forwarder proxyForwarder
	mapper    auth.ClaimMapper
	logger    *slog.Logger
}

// NewS3Handler creates a new S3 proxy handler with the default claim mapping
// (Keycloak-compatible). See NewS3HandlerWithMapper for provider-neutral claim
// configuration.
//
// s3Client may be nil; callers that only need the auth/ACL gating (e.g. unit
// tests) can pass nil, in which case the bucket-listing and pass-through paths
// return a 502 rather than panicking.
func NewS3Handler(verifier auth.TokenVerifier, s3Client *S3Client, logger *slog.Logger) *S3Handler {
	return NewS3HandlerWithMapper(verifier, s3Client, auth.ClaimMapper{}, logger)
}

// NewS3HandlerWithMapper is like NewS3Handler but uses the supplied claim mapper
// to resolve identity and groups from token claims, supporting any OIDC IdP.
func NewS3HandlerWithMapper(verifier auth.TokenVerifier, s3Client *S3Client, mapper auth.ClaimMapper, logger *slog.Logger) *S3Handler {
	h := &S3Handler{
		verifier: verifier,
		mapper:   mapper,
		logger:   logger,
	}
	// Only wire the backend/forwarder when a real client is supplied. A typed
	// nil *S3Client stored in an interface would be non-nil, defeating the nil
	// guard, so keep the interface fields nil instead.
	if s3Client != nil {
		h.backend = s3Client
		h.forwarder = NewPassThrough(s3Client, logger)
	}
	return h
}

// newS3HandlerWithDeps builds a handler from explicit collaborators. Used by
// tests to inject fakes for the backend and forwarder.
func newS3HandlerWithDeps(verifier auth.TokenVerifier, backend S3Backend, forwarder proxyForwarder, logger *slog.Logger) *S3Handler {
	return &S3Handler{
		verifier:  verifier,
		backend:   backend,
		forwarder: forwarder,
		logger:    logger,
	}
}

// ServeHTTP handles incoming S3 proxy requests.
func (h *S3Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenString := extractBearerToken(r)
	if tokenString == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid authorization header")
		return
	}

	// Verify token
	claims, err := h.verifier.VerifyToken(tokenString)
	if err != nil {
		h.logger.Warn("token verification failed", "error", err.Error())
		writeJSONError(w, http.StatusUnauthorized, "invalid_token", "token verification failed")
		return
	}

	username := h.mapper.User(claims)
	groups := h.mapper.Groups(claims)

	// Parse bucket from path: /<bucket>/... or /<bucket>
	bucket := pathBucket(r.URL.Path)
	if bucket == "" {
		// Root path: GET / → list buckets (filtered by ACL)
		if r.Method == http.MethodGet {
			h.handleListBuckets(w, r, groups)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "bad_request", "bucket name required")
		return
	}

	// Check for admin operations (always blocked)
	if acl.IsAdminOperation(r.Method, r.URL.RequestURI()) {
		h.logger.Warn("admin operation blocked",
			"user", username,
			"method", r.Method,
			"path", r.URL.Path,
		)
		writeJSONError(w, http.StatusForbidden, "operation_not_permitted", "admin operations are not allowed")
		return
	}

	if !acl.CheckAccess(groups, bucket, r.Method) {
		// Log the decision for audit, but not the caller's full group set: that
		// is their complete authorization profile and does not belong in
		// access-denial logs that may ship to lower-trust aggregators.
		h.logger.Warn("access denied",
			"user", username,
			"bucket", bucket,
			"method", r.Method,
			"group_count", len(groups),
		)
		writeJSONError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return
	}

	h.logger.Info("proxying request",
		"user", username,
		"method", r.Method,
		"bucket", bucket,
		"path", r.URL.Path,
	)

	if h.forwarder == nil {
		h.logger.Error("pass-through requested but no S3 backend configured")
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "S3 backend is not configured")
		return
	}

	// Stream the request to the backend and the response back to the client.
	h.forwarder.Forward(w, r)
}

// handleListBuckets handles GET / by listing all buckets and filtering by ACL.
func (h *S3Handler) handleListBuckets(w http.ResponseWriter, r *http.Request, groups []string) {
	if h.backend == nil {
		h.logger.Error("list buckets requested but no S3 backend configured")
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "S3 backend is not configured")
		return
	}

	ctx := r.Context()

	result, err := h.backend.ListBuckets(ctx)
	if err != nil {
		h.logger.Error("failed to list buckets", "error", err.Error())
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "failed to list buckets from storage")
		return
	}

	// Filter buckets by effective permissions. A user whose grant is the
	// wildcard ("*-<policy>") holds no explicitly-named buckets, so it must be
	// expanded against the backend's actual bucket set - otherwise a wildcard
	// grantee (e.g. an auditor with "*-ro") would see an empty list here even
	// though CheckAccess correctly authorizes them per bucket.
	effectiveMap := make(map[string]bool)
	for _, bp := range acl.EffectiveBuckets(groups) {
		effectiveMap[bp.Name] = true
	}
	_, hasWildcard := acl.WildcardGrant(groups)

	// Build filtered XML response (simplified as JSON for the proxy)
	type BucketInfo struct {
		Name string `json:"name"`
	}
	filtered := make([]BucketInfo, 0, len(result.Buckets))
	for _, b := range result.Buckets {
		if b.Name == nil {
			continue
		}
		if hasWildcard || effectiveMap[*b.Name] {
			filtered = append(filtered, BucketInfo{Name: *b.Name})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(filtered); err != nil {
		h.logger.Error("failed to encode bucket list response", "error", err.Error())
	}
}

// pathBucket extracts the bucket name from an S3-style URL path
// (/<bucket>/... or /<bucket>). Returns "" for the root path.
func pathBucket(path string) string {
	path = strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}

// extractBearerToken extracts the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// writeJSONError writes a JSON error response. nosniff is set so a browser
// navigating directly to a proxy URL cannot MIME-sniff the error body into
// active content.
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

func init() {
	// Ensure S3Handler implements http.Handler
	var _ http.Handler = (*S3Handler)(nil)
}
