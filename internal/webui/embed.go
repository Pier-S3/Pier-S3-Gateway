package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed static
var staticFS embed.FS

// buildAppCSP returns the Content-Security-Policy for the SPA's own documents
// and assets. It is intentionally strict (defense-in-depth: the OIDC access
// token lives in the page, so an injected script would mean token theft), but
// must allow the few cross-origin things the app legitimately needs:
//   - the Keycloak origin for the OIDC token endpoint (connect-src) and the
//     silent-renew iframe (frame-src),
//   - Google Fonts (style + font),
//   - blob:/data: for in-app file previews (img/media/pdf-iframe).
//
// keycloakOrigin is the scheme://host[:port] of the Keycloak server, derived
// from KEYCLOAK_URL. If it cannot be parsed it is simply omitted (the policy
// stays valid, only cross-origin Keycloak calls would be blocked).
func buildAppCSP(keycloakURL string) string {
	kc := originOf(keycloakURL)

	connect := "'self'"
	frame := "blob:"
	if kc != "" {
		connect += " " + kc
		frame += " " + kc
	}

	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		// antd (CSS-in-JS) injects <style> tags, so inline styles are required.
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"img-src 'self' blob: data:",
		"media-src 'self' blob:",
		"frame-src " + frame,
		"connect-src " + connect,
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}, "; ")
}

// originOf returns the scheme://host[:port] of a URL, or "" if it cannot be
// parsed into an absolute origin.
func originOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// StaticHandler serves the embedded frontend files with SPA fallback.
// All paths not starting with /api/ are served from the embedded static
// directory; unknown paths fall back to index.html for client-side routing.
//
// Every response carries hardening headers (CSP/nosniff/frame/referrer) so the
// application shell that holds the OIDC token is defended in depth.
//
// The fs.Sub call cannot fail in practice because "static" is a constant valid
// path embedded at build time; if it ever does (e.g. the embed directive is
// removed), the returned handler serves a 500 instead so callers that already
// hold an *http.Server can shut down cleanly rather than crash the process.
func StaticHandler(keycloakURL string) http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "static assets unavailable", http.StatusInternalServerError)
		})
	}

	csp := buildAppCSP(keycloakURL)
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip API routes
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Security headers for the app shell and its assets.
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Try to serve the file directly
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// SPA fallback: serve index.html for all non-API, non-file routes
		if _, err := fs.Stat(sub, path); err != nil {
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
