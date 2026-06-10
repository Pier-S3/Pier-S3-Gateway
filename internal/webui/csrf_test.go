package webui

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// csrfHandler builds an auth-protected handler that records whether the inner
// handler ran, so tests can assert a request was blocked before reaching it.
func csrfHandler(t *testing.T) (http.Handler, *bool) {
	t.Helper()
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "u", "groups": []interface{}{"reports-rw"}}}
	ran := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ran = true
		w.WriteHeader(http.StatusOK)
	})
	return AuthMiddleware(verifier)(inner), &ran
}

// withAccessCookie sets the access_token cookie a browser session would carry.
func withAccessCookie(r *http.Request) *http.Request {
	r.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: "valid.jwt.token"})
	return r
}

func TestCSRFCookieAuth(t *testing.T) {
	_ = slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("cross-site cookie PUT is blocked", func(t *testing.T) {
		h, ran := csrfHandler(t)
		req := withAccessCookie(httptest.NewRequest("PUT", "/api/v1/buckets/reports/objects/k", nil))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, *ran, "handler must not run for a blocked CSRF request")
	})

	t.Run("cross-origin Origin mismatch is blocked", func(t *testing.T) {
		h, ran := csrfHandler(t)
		req := withAccessCookie(httptest.NewRequest("DELETE", "/api/v1/buckets/reports/objects/k", nil))
		req.Host = "ui.example.com"
		req.Header.Set("Origin", "https://evil.example.org")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, *ran)
	})

	t.Run("same-origin cookie PUT is allowed", func(t *testing.T) {
		h, ran := csrfHandler(t)
		req := withAccessCookie(httptest.NewRequest("PUT", "/api/v1/buckets/reports/objects/k", nil))
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, *ran)
	})

	t.Run("bearer-auth cross-site is exempt", func(t *testing.T) {
		// The SPA uses a Bearer header; a cross-origin page cannot read the token
		// to set it, so header auth is not subject to the CSRF block.
		h, ran := csrfHandler(t)
		req := httptest.NewRequest("PUT", "/api/v1/buckets/reports/objects/k", nil)
		req.Header.Set("Authorization", "Bearer valid.jwt.token")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, *ran)
	})

	t.Run("cookie GET is never blocked", func(t *testing.T) {
		h, ran := csrfHandler(t)
		req := withAccessCookie(httptest.NewRequest("GET", "/api/v1/buckets", nil))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, *ran)
	})
}
