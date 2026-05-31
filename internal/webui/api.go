package webui

import (
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"pier-s3/internal/acl"
	"pier-s3/internal/auth"
	"pier-s3/internal/proxy"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// API holds dependencies for the Web UI API handlers.
type API struct {
	backend s3Backend
	logger  *slog.Logger
}

// NewAPI creates a new API handler with dependencies.
//
// A nil s3Client (or a client without a backend) is tolerated: object
// handlers will respond with 503 Service Unavailable rather than panic.
func NewAPI(s3Client *proxy.S3Client, logger *slog.Logger) *API {
	return &API{
		backend: newS3Backend(s3Client),
		logger:  logger,
	}
}

// newAPIWithBackend builds an API around an arbitrary s3Backend. Used by tests
// to inject a mock backend.
func newAPIWithBackend(backend s3Backend, logger *slog.Logger) *API {
	return &API{
		backend: backend,
		logger:  logger,
	}
}

// backendReady reports whether an S3 backend is configured. When it is not,
// it writes a 503 error response and returns false so the caller can stop.
func (a *API) backendReady(w http.ResponseWriter) bool {
	if a.backend == nil {
		writeError(w, http.StatusServiceUnavailable, "backend_unavailable", "storage backend is not configured")
		return false
	}
	return true
}

// BucketResponse represents a bucket in the API response.
type BucketResponse struct {
	Name        string          `json:"name"`
	Permissions PermissionFlags `json:"permissions"`
	ObjectCount int64           `json:"object_count,omitempty"`
	SizeBytes   int64           `json:"size_bytes,omitempty"`
}

// PermissionFlags represents the per-bucket access a user holds.
type PermissionFlags struct {
	Read   bool `json:"read"`
	Write  bool `json:"write"`
	Delete bool `json:"delete"`
}

// ObjectResponse represents an object in the API response.
type ObjectResponse struct {
	Key          string    `json:"key"`
	SizeBytes    int64     `json:"size_bytes"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
	IsPrefix     bool      `json:"is_prefix"`
}

// ListObjectsResponse is the response for listing objects.
type ListObjectsResponse struct {
	Objects       []ObjectResponse `json:"objects"`
	Prefixes      []string         `json:"prefixes"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	Truncated     bool             `json:"truncated"`
}

// HandleListBuckets handles GET /api/v1/buckets.
//
// It returns the explicitly-named buckets a user can access (from their
// groups), merged - when the user holds a wildcard ("*-<policy>") group - with
// every bucket reported by the storage backend, so a wildcard grantee actually
// sees the buckets it can reach. Named-group permissions take precedence over
// the wildcard for the same bucket (they are unioned).
func (a *API) HandleListBuckets(w http.ResponseWriter, r *http.Request) {
	groups := GetGroups(r)

	// Start from explicitly-named buckets, keyed by name for merging.
	perms := make(map[string]*acl.BucketPermission)
	order := []string{}
	for _, bp := range acl.EffectiveBuckets(groups) {
		cp := bp
		perms[bp.Name] = &cp
		order = append(order, bp.Name)
	}

	// If the user has any wildcard grant, expand it across the buckets the
	// backend actually has and union the wildcard rights onto each.
	if wild, ok := acl.WildcardGrant(groups); ok && a.backend != nil {
		out, err := a.backend.ListBuckets(r.Context())
		if err != nil {
			a.logger.Error("failed to list buckets for wildcard grant", "error", err.Error())
			writeError(w, http.StatusBadGateway, "upstream_error", "failed to list buckets")
			return
		}
		for _, b := range out.Buckets {
			if b.Name == nil {
				continue
			}
			name := *b.Name
			if _, exists := perms[name]; !exists {
				perms[name] = &acl.BucketPermission{Name: name}
				order = append(order, name)
			}
			perms[name].Read = perms[name].Read || wild.Read
			perms[name].Write = perms[name].Write || wild.Write
			perms[name].Delete = perms[name].Delete || wild.Delete
		}
	}

	sort.Strings(order)
	buckets := make([]BucketResponse, 0, len(order))
	for _, name := range order {
		bp := perms[name]
		buckets = append(buckets, BucketResponse{
			Name: bp.Name,
			Permissions: PermissionFlags{
				Read:   bp.Read,
				Write:  bp.Write,
				Delete: bp.Delete,
			},
		})
	}

	writeJSON(w, http.StatusOK, buckets)
}

// HandleListObjects handles GET /api/v1/buckets/{bucket}/objects
func (a *API) HandleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := bucketFromRequest(r)
	if bucket == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket name required")
		return
	}

	groups := GetGroups(r)
	if !acl.CheckAccess(groups, bucket, "GET") {
		writeError(w, http.StatusForbidden, "forbidden", "no read access to bucket")
		return
	}

	if !a.backendReady(w) {
		return
	}

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	if delimiter == "" {
		delimiter = "/"
	}
	pageToken := r.URL.Query().Get("page_token")
	maxKeysStr := r.URL.Query().Get("max_keys")
	maxKeys := int32(1000)
	if maxKeysStr != "" {
		if mk, err := strconv.ParseInt(maxKeysStr, 10, 32); err == nil && mk > 0 && mk <= 1000 {
			maxKeys = int32(mk)
		}
	}

	result, err := a.backend.ListObjects(r.Context(), bucket, prefix, delimiter, pageToken, maxKeys)
	if err != nil {
		a.logger.Error("failed to list objects", "bucket", bucket, "error", err.Error())
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to list objects")
		return
	}

	var objects []ObjectResponse
	for _, obj := range result.Contents {
		etag := ""
		if obj.ETag != nil {
			etag = strings.Trim(*obj.ETag, "\"")
		}
		objects = append(objects, ObjectResponse{
			Key:          deref(obj.Key),
			SizeBytes:    derefInt64(obj.Size),
			LastModified: derefTime(obj.LastModified),
			ETag:         etag,
			IsPrefix:     false,
		})
	}

	var prefixes []string
	for _, p := range result.CommonPrefixes {
		if p.Prefix != nil {
			prefixes = append(prefixes, *p.Prefix)
		}
	}

	nextPageToken := ""
	if result.NextContinuationToken != nil {
		nextPageToken = *result.NextContinuationToken
	}

	resp := ListObjectsResponse{
		Objects:       objects,
		Prefixes:      prefixes,
		NextPageToken: nextPageToken,
		Truncated:     derefBool(result.IsTruncated),
	}
	if resp.Objects == nil {
		resp.Objects = []ObjectResponse{}
	}
	if resp.Prefixes == nil {
		resp.Prefixes = []string{}
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleGetObject handles GET /api/v1/buckets/{bucket}/objects/{key+} - streaming download
func (a *API) HandleGetObject(w http.ResponseWriter, r *http.Request) {
	bucket, key := bucketAndKeyFromRequest(r)
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key required")
		return
	}

	groups := GetGroups(r)
	if !acl.CheckAccess(groups, bucket, "GET") {
		writeError(w, http.StatusForbidden, "forbidden", "no read access")
		return
	}

	if !a.backendReady(w) {
		return
	}

	result, err := a.backend.GetObject(r.Context(), bucket, key)
	if err != nil {
		a.logger.Error("failed to get object", "bucket", bucket, "key", key, "error", err.Error())
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to get object")
		return
	}
	defer result.Body.Close()

	// Security hardening: a direct hit on this URL must never execute active
	// content in our origin (which would enable OIDC token theft). Always send
	// nosniff + a fully sandboxed CSP, and downgrade active/dangerous content
	// types (HTML/XHTML/SVG/XML, ...) to an inert octet-stream attachment.
	// Body bytes are never altered - only headers/disposition.
	w.Header().Set("X-Content-Type-Options", headerXCTO)
	w.Header().Set("Content-Security-Policy", headerCSP)

	disp := neutralizeContentType(deref(result.ContentType))
	w.Header().Set("Content-Type", disp.ContentType)
	if disp.Inline {
		w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(key)+`"`)
	} else {
		w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(key)+`"`)
	}

	if result.ContentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*result.ContentLength, 10))
	}
	if result.ETag != nil {
		w.Header().Set("ETag", *result.ETag)
	}
	if result.LastModified != nil {
		w.Header().Set("Last-Modified", result.LastModified.Format(http.TimeFormat))
	}

	// Stream body. Headers are already committed, so on a copy error we can
	// only log it - the client will observe a truncated/aborted response.
	if _, err := io.Copy(w, result.Body); err != nil {
		a.logger.Error("failed to stream object body", "bucket", bucket, "key", key, "error", err.Error())
	}
}

// HandleGetObjectMeta handles GET /api/v1/buckets/{bucket}/objects/{key+}/meta
func (a *API) HandleGetObjectMeta(w http.ResponseWriter, r *http.Request) {
	bucket, key := bucketAndKeyMetaFromRequest(r)
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key required")
		return
	}

	groups := GetGroups(r)
	if !acl.CheckAccess(groups, bucket, "GET") {
		writeError(w, http.StatusForbidden, "forbidden", "no read access")
		return
	}

	if !a.backendReady(w) {
		return
	}

	result, err := a.backend.HeadObject(r.Context(), bucket, key)
	if err != nil {
		a.logger.Error("failed to head object", "bucket", bucket, "key", key, "error", err.Error())
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to get object metadata")
		return
	}

	meta := map[string]interface{}{
		"key":            key,
		"content_type":   deref(result.ContentType),
		"content_length": derefInt64(result.ContentLength),
		"etag":           strings.Trim(deref(result.ETag), "\""),
		"last_modified":  derefTime(result.LastModified),
	}

	// Harden the metadata response too: nosniff prevents the JSON envelope
	// from being reinterpreted, and the sandboxed CSP neutralizes any active
	// content should the endpoint ever be navigated to directly.
	w.Header().Set("X-Content-Type-Options", headerXCTO)
	w.Header().Set("Content-Security-Policy", headerCSP)

	writeJSON(w, http.StatusOK, meta)
}

// HandlePutObject handles PUT /api/v1/buckets/{bucket}/objects/{key+} - streaming upload
func (a *API) HandlePutObject(w http.ResponseWriter, r *http.Request) {
	bucket, key := bucketAndKeyFromRequest(r)
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key required")
		return
	}

	groups := GetGroups(r)
	if !acl.CheckAccess(groups, bucket, "PUT") {
		writeError(w, http.StatusForbidden, "write_not_allowed", "no write access to bucket")
		return
	}

	if !a.backendReady(w) {
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var contentLength *int64
	if cl := r.Header.Get("Content-Length"); cl != "" {
		if v, err := strconv.ParseInt(cl, 10, 64); err == nil {
			contentLength = &v
		}
	}

	input := &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        r.Body,
		ContentType: &contentType,
	}
	if contentLength != nil {
		input.ContentLength = contentLength
	}

	_, err := a.backend.PutObject(r.Context(), input)
	if err != nil {
		a.logger.Error("failed to put object", "bucket", bucket, "key", key, "error", err.Error())
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to upload object")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "uploaded",
		"key":    key,
	})
}

// HandleDeleteObject handles DELETE /api/v1/buckets/{bucket}/objects/{key+}
func (a *API) HandleDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket, key := bucketAndKeyFromRequest(r)
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key required")
		return
	}

	groups := GetGroups(r)
	if !acl.CheckAccess(groups, bucket, "DELETE") {
		writeError(w, http.StatusForbidden, "write_not_allowed", "no delete access to bucket")
		return
	}

	if !a.backendReady(w) {
		return
	}

	_, err := a.backend.DeleteObject(r.Context(), bucket, key)
	if err != nil {
		a.logger.Error("failed to delete object", "bucket", bucket, "key", key, "error", err.Error())
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to delete object")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"key":    key,
	})
}

// RegisterRoutes registers all API routes on the given mux.
func (a *API) RegisterRoutes(mux *http.ServeMux, verifier auth.TokenVerifier, oidcCfg *auth.OIDCConfig) {
	authMw := AuthMiddleware(verifier)

	// Throttle the unauthenticated OIDC endpoints per client IP to blunt
	// brute-force / token-exchange abuse: 20 requests/minute with a small
	// burst is far above any legitimate interactive login rate.
	authRL := rateLimitMiddleware(newRateLimiter(20, 10))

	// Auth endpoints
	mux.Handle("GET /api/v1/auth/login", authRL(HandleLogin(oidcCfg)))
	mux.Handle("GET /api/v1/auth/callback", authRL(HandleCallback(oidcCfg, a.logger)))
	mux.Handle("GET /api/v1/auth/me", authMw(http.HandlerFunc(HandleAuthMe)))
	mux.Handle("POST /api/v1/auth/logout", authRL(HandleLogout(oidcCfg)))

	// Bucket endpoints
	mux.Handle("GET /api/v1/buckets", authMw(http.HandlerFunc(a.HandleListBuckets)))

	// Object endpoints - Go 1.22 method-based routing with a {key...} wildcard.
	// The wildcard must be the final segment, so object metadata is exposed
	// under a distinct /meta/ prefix rather than an /objects/.../meta suffix
	// (the latter is an invalid pattern and would also misroute keys that
	// literally end in "/meta").
	mux.Handle("GET /api/v1/buckets/{bucket}/meta/{key...}", authMw(http.HandlerFunc(a.HandleGetObjectMeta)))
	mux.Handle("GET /api/v1/buckets/{bucket}/objects/{key...}", authMw(http.HandlerFunc(a.HandleGetObject)))
	mux.Handle("PUT /api/v1/buckets/{bucket}/objects/{key...}", authMw(http.HandlerFunc(a.HandlePutObject)))
	mux.Handle("DELETE /api/v1/buckets/{bucket}/objects/{key...}", authMw(http.HandlerFunc(a.HandleDeleteObject)))
	mux.Handle("GET /api/v1/buckets/{bucket}/objects", authMw(http.HandlerFunc(a.HandleListObjects)))
}

// Helper functions for path parsing.
//
// When requests are routed through the Go 1.22 ServeMux patterns registered in
// RegisterRoutes, the {bucket} and {key...} wildcards are available via
// r.PathValue and are preferred because they are unambiguous (a key that itself
// ends in "/meta" is not misrouted, and "/objects/" inside a key is handled by
// the router). When PathValue is empty - e.g. a handler invoked directly in a
// unit test without the mux - we fall back to manual URL-path parsing.

// bucketFromRequest returns the bucket name for object-list requests.
func bucketFromRequest(r *http.Request) string {
	if b := r.PathValue("bucket"); b != "" {
		return b
	}
	return extractPathParam(r.URL.Path, "buckets")
}

// bucketAndKeyFromRequest returns the bucket and object key.
func bucketAndKeyFromRequest(r *http.Request) (string, string) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")
	if bucket != "" && key != "" {
		return bucket, key
	}
	return extractBucketAndKey(r.URL.Path)
}

// bucketAndKeyMetaFromRequest returns the bucket and key for the /meta route.
func bucketAndKeyMetaFromRequest(r *http.Request) (string, string) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")
	if bucket != "" && key != "" {
		return bucket, key
	}
	return extractBucketAndKeyMeta(r.URL.Path)
}

// extractPathParam extracts a path parameter value after the given segment.
// E.g., extractPathParam("/api/v1/buckets/my-bucket/objects", "buckets") → "my-bucket"
func extractPathParam(path, segment string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == segment && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// extractBucketAndKey extracts bucket and key from path like /api/v1/buckets/{bucket}/objects/{key+}
func extractBucketAndKey(path string) (string, string) {
	// Path: /api/v1/buckets/{bucket}/objects/{key...}
	const prefix = "/api/v1/buckets/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	rest := path[len(prefix):]
	parts := strings.SplitN(rest, "/objects/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// extractBucketAndKeyMeta extracts bucket and key from a metadata path of the
// form /api/v1/buckets/{bucket}/meta/{key...}.
func extractBucketAndKeyMeta(path string) (string, string) {
	const prefix = "/api/v1/buckets/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	rest := path[len(prefix):]
	parts := strings.SplitN(rest, "/meta/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// Pointer dereference helpers
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
