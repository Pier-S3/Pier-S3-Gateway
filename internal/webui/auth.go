package webui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"pier-s3/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is a custom type for context keys.
type contextKey string

const (
	claimsKey   contextKey = "claims"
	usernameKey contextKey = "username"
	groupsKey   contextKey = "groups"
)

// AuthMiddleware extracts and verifies the Bearer token using the default
// claim mapping (Keycloak-compatible). See AuthMiddlewareWithMapper for
// provider-neutral claim configuration.
func AuthMiddleware(verifier auth.TokenVerifier) func(http.Handler) http.Handler {
	return AuthMiddlewareWithMapper(verifier, auth.ClaimMapper{})
}

// AuthMiddlewareWithMapper extracts and verifies the Bearer token from the
// Authorization header or HttpOnly cookie, resolving identity and groups via
// the supplied claim mapper. On success, stores claims, username, and groups in
// the request context.
func AuthMiddlewareWithMapper(verifier auth.TokenVerifier, mapper auth.ClaimMapper) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}

			claims, err := verifier.VerifyToken(tokenString)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_token", "token verification failed")
				return
			}

			username := mapper.User(claims)
			groups := mapper.Groups(claims)

			ctx := r.Context()
			ctx = context.WithValue(ctx, claimsKey, claims)
			ctx = context.WithValue(ctx, usernameKey, username)
			ctx = context.WithValue(ctx, groupsKey, groups)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken gets the bearer token from the Authorization header, falling
// back to the access_token HttpOnly cookie set by the OIDC callback handler
// (see oidc_handlers.go). The Authorization header takes precedence.
func extractToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// Fall back to the access_token cookie (browser sessions).
	cookie, err := r.Cookie(accessTokenCookie)
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

// GetUsername extracts username from request context (set by AuthMiddleware).
func GetUsername(r *http.Request) string {
	if v, ok := r.Context().Value(usernameKey).(string); ok {
		return v
	}
	return ""
}

// GetGroups extracts groups from request context (set by AuthMiddleware).
func GetGroups(r *http.Request) []string {
	if v, ok := r.Context().Value(groupsKey).([]string); ok {
		return v
	}
	return nil
}

// GetClaims extracts JWT claims from request context (set by AuthMiddleware).
func GetClaims(r *http.Request) jwt.MapClaims {
	if v, ok := r.Context().Value(claimsKey).(jwt.MapClaims); ok {
		return v
	}
	return nil
}

// HandleAuthMe returns the current user's info.
// GET /api/v1/auth/me
func HandleAuthMe(w http.ResponseWriter, r *http.Request) {
	username := GetUsername(r)
	groups := GetGroups(r)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"username": username,
		"groups":   groups,
	})
}

// HandleLogout handles user logout by clearing cookies and returning the end_session URL.
// POST /api/v1/auth/logout
func HandleLogout(oidcCfg *auth.OIDCConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
			return
		}
		auth.ClearRefreshTokenCookie(w)
		// Clear access_token cookie too.
		clearCookie(w, accessTokenCookie)

		logoutURL := ""
		if oidcCfg != nil {
			logoutURL = oidcCfg.EndSessionURL
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"logout_url": logoutURL,
		})
	}
}

// writeJSON writes a JSON response. Encoding errors are logged-by-side-effect
// only: the status and headers are already committed by the time encoding runs,
// so there is nothing meaningful to send to the client. We surface them through
// the default logger to avoid silently swallowing the error.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Default().Error("failed to encode JSON response", "error", err.Error())
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}
