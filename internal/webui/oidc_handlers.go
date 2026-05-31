package webui

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"

	"pier-s3/internal/auth"
)

const (
	// accessTokenCookie holds the Keycloak access token. It is the token the
	// API's AuthMiddleware reads when no Authorization header is present, so
	// browser sessions authenticate via this HttpOnly cookie.
	accessTokenCookie = "access_token"
	// pkceVerifierCookie temporarily stores the PKCE code_verifier between the
	// /login redirect and the /callback exchange.
	pkceVerifierCookie = "pkce_verifier"
	// oauthStateCookie temporarily stores the CSRF state value.
	oauthStateCookie = "oauth_state"
)

// HandleLogin starts the OIDC Authorization Code + PKCE flow.
//
// It generates a PKCE verifier and CSRF state, stores them in short-lived
// HttpOnly cookies, and redirects the browser to the Keycloak authorization
// endpoint. GET /api/v1/auth/login
func HandleLogin(oidcCfg *auth.OIDCConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if oidcCfg == nil {
			writeError(w, http.StatusServiceUnavailable, "oidc_unavailable", "OIDC is not configured")
			return
		}

		verifier, err := randomURLSafe(32)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to start login")
			return
		}
		state, err := randomURLSafe(16)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to start login")
			return
		}

		setTransientCookie(w, pkceVerifierCookie, verifier)
		setTransientCookie(w, oauthStateCookie, state)

		challenge := pkceChallenge(verifier)
		q := url.Values{
			"response_type":         {"code"},
			"client_id":             {oidcCfg.ClientID},
			"redirect_uri":          {oidcCfg.RedirectURI},
			"scope":                 {"openid profile"},
			"state":                 {state},
			"code_challenge":        {challenge},
			"code_challenge_method": {"S256"},
		}

		http.Redirect(w, r, oidcCfg.AuthURL+"?"+q.Encode(), http.StatusFound)
	}
}

// HandleCallback completes the OIDC flow: it validates state, exchanges the
// authorization code for tokens, and stores them in HttpOnly cookies so that
// subsequent API calls authenticate via the access_token cookie.
// GET /api/v1/auth/callback
func HandleCallback(oidcCfg *auth.OIDCConfig, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if oidcCfg == nil {
			writeError(w, http.StatusServiceUnavailable, "oidc_unavailable", "OIDC is not configured")
			return
		}

		// Surface provider-side errors (e.g. access_denied).
		if errCode := r.URL.Query().Get("error"); errCode != "" {
			writeError(w, http.StatusUnauthorized, "oidc_error", errCode)
			return
		}

		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" || state == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "missing code or state")
			return
		}

		stateCookie, err := r.Cookie(oauthStateCookie)
		// Constant-time comparison of the CSRF state token avoids leaking
		// timing information about how much of the value matched.
		if err != nil || stateCookie.Value == "" ||
			subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
			writeError(w, http.StatusBadRequest, "invalid_state", "state mismatch")
			return
		}

		verifierCookie, err := r.Cookie(pkceVerifierCookie)
		if err != nil || verifierCookie.Value == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "missing PKCE verifier")
			return
		}

		tokens, err := oidcCfg.ExchangeCode(code, verifierCookie.Value)
		if err != nil {
			if logger != nil {
				logger.Error("token exchange failed", "error", err.Error())
			}
			writeError(w, http.StatusBadGateway, "token_exchange_failed", "failed to exchange authorization code")
			return
		}

		// Clear the one-time transient cookies.
		clearCookie(w, pkceVerifierCookie)
		clearCookie(w, oauthStateCookie)

		// Persist the access token for the API and the refresh token for renewal.
		setAccessTokenCookie(w, tokens.AccessToken, tokens.ExpiresIn)
		if tokens.RefreshToken != "" {
			auth.SetRefreshTokenCookie(w, tokens.RefreshToken, 0)
		}

		// Land the user back on the SPA root.
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// setAccessTokenCookie stores the access token in an HttpOnly cookie.
// maxAge <= 0 results in a session cookie.
func setAccessTokenCookie(w http.ResponseWriter, token string, maxAge int) {
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// setTransientCookie stores a short-lived value used during the login round-trip.
func setTransientCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
	})
}

// clearCookie expires a cookie immediately.
func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// randomURLSafe returns a URL-safe, base64-raw-encoded random string built from
// n bytes of cryptographically secure randomness.
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge derives the S256 PKCE code_challenge from a verifier.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
