package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticHandlerServesIndex(t *testing.T) {
	h := StaticHandler("https://keycloak.example.com/realms/master")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<html", "root should serve index.html")
}

func TestStaticHandlerSPAFallback(t *testing.T) {
	h := StaticHandler("https://keycloak.example.com/realms/master")

	// A client-side route that is not a real file should fall back to index.html.
	req := httptest.NewRequest("GET", "/buckets/some/deep/route", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<html", "unknown route should fall back to index.html")
}

func TestStaticHandlerRejectsAPIRoutes(t *testing.T) {
	h := StaticHandler("https://keycloak.example.com/realms/master")

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "API routes must not be served by the static handler")
}

func TestStaticHandlerSecurityHeaders(t *testing.T) {
	h := StaticHandler("https://keycloak.example.com/realms/master")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "object-src 'none'")
	// The Keycloak ORIGIN (no path) must be reachable for the OIDC token
	// endpoint and the silent-renew iframe.
	assert.Contains(t, csp, "connect-src 'self' https://keycloak.example.com")
	assert.Contains(t, csp, "frame-src blob: https://keycloak.example.com")
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestBuildAppCSPOmitsUnparseableKeycloak(t *testing.T) {
	csp := buildAppCSP("")
	assert.Contains(t, csp, "connect-src 'self'")
	assert.NotContains(t, csp, "connect-src 'self' ")
}
