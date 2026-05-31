package webui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"pier-s3/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestHandleLoginRedirectsWithPKCE(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/callback")
	req := httptest.NewRequest("GET", "/api/v1/auth/login", nil)
	w := httptest.NewRecorder()

	HandleLogin(cfg)(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	require.NotEmpty(t, loc)

	u, err := url.Parse(loc)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "client-x", q.Get("client_id"))
	assert.Equal(t, "https://app/callback", q.Get("redirect_uri"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotEmpty(t, q.Get("code_challenge"))
	state := q.Get("state")
	require.NotEmpty(t, state)

	resp := w.Result()
	verifierCookie := findCookie(resp, pkceVerifierCookie)
	stateCookie := findCookie(resp, oauthStateCookie)
	require.NotNil(t, verifierCookie)
	require.NotNil(t, stateCookie)
	assert.True(t, verifierCookie.HttpOnly)
	assert.Equal(t, state, stateCookie.Value, "state cookie must match state param")

	// Verify the challenge corresponds to the stored verifier.
	sum := sha256.Sum256([]byte(verifierCookie.Value))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), q.Get("code_challenge"))
}

func TestHandleLoginNilConfig(t *testing.T) {
	w := httptest.NewRecorder()
	HandleLogin(nil)(w, httptest.NewRequest("GET", "/api/v1/auth/login", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleCallbackSuccess(t *testing.T) {
	// Fake Keycloak token endpoint.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "the-code", r.Form.Get("code"))
		assert.Equal(t, "the-verifier", r.Form.Get("code_verifier"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "AT-123",
			"refresh_token": "RT-456",
			"token_type":    "Bearer",
			"expires_in":    300,
		})
	}))
	defer tokenSrv.Close()

	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	cfg.TokenURL = tokenSrv.URL

	req := httptest.NewRequest("GET", "/api/v1/auth/callback?code=the-code&state=st-1", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "st-1"})
	req.AddCookie(&http.Cookie{Name: pkceVerifierCookie, Value: "the-verifier"})
	w := httptest.NewRecorder()

	HandleCallback(cfg, testLogger())(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))

	resp := w.Result()
	at := findCookie(resp, accessTokenCookie)
	rt := findCookie(resp, "refresh_token")
	require.NotNil(t, at)
	assert.Equal(t, "AT-123", at.Value)
	assert.True(t, at.HttpOnly)
	require.NotNil(t, rt)
	assert.Equal(t, "RT-456", rt.Value)

	// Transient cookies cleared.
	if c := findCookie(resp, pkceVerifierCookie); c != nil {
		assert.True(t, c.MaxAge < 0)
	}
	if c := findCookie(resp, oauthStateCookie); c != nil {
		assert.True(t, c.MaxAge < 0)
	}
}

func TestHandleCallbackStateMismatch(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	req := httptest.NewRequest("GET", "/api/v1/auth/callback?code=c&state=evil", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "expected"})
	req.AddCookie(&http.Cookie{Name: pkceVerifierCookie, Value: "v"})
	w := httptest.NewRecorder()
	HandleCallback(cfg, testLogger())(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "invalid_state", resp["error"])
}

func TestHandleCallbackMissingState(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	req := httptest.NewRequest("GET", "/api/v1/auth/callback?code=c", nil)
	w := httptest.NewRecorder()
	HandleCallback(cfg, testLogger())(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCallbackMissingVerifierCookie(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	req := httptest.NewRequest("GET", "/api/v1/auth/callback?code=c&state=s", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "s"})
	w := httptest.NewRecorder()
	HandleCallback(cfg, testLogger())(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCallbackProviderError(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	req := httptest.NewRequest("GET", "/api/v1/auth/callback?error=access_denied", nil)
	w := httptest.NewRecorder()
	HandleCallback(cfg, testLogger())(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "access_denied", resp["message"])
}

func TestHandleCallbackExchangeFailure(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tokenSrv.Close()

	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	cfg.TokenURL = tokenSrv.URL
	req := httptest.NewRequest("GET", "/api/v1/auth/callback?code=c&state=s", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "s"})
	req.AddCookie(&http.Cookie{Name: pkceVerifierCookie, Value: "v"})
	w := httptest.NewRecorder()
	HandleCallback(cfg, testLogger())(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandleCallbackNilConfig(t *testing.T) {
	w := httptest.NewRecorder()
	HandleCallback(nil, testLogger())(w, httptest.NewRequest("GET", "/api/v1/auth/callback", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleLogout(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	HandleLogout(cfg)(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, cfg.EndSessionURL, resp["logout_url"])

	result := w.Result()
	at := findCookie(result, accessTokenCookie)
	rt := findCookie(result, "refresh_token")
	require.NotNil(t, at)
	require.NotNil(t, rt)
	assert.True(t, at.MaxAge < 0, "access_token cookie cleared")
	assert.True(t, rt.MaxAge < 0, "refresh_token cookie cleared")
}

func TestHandleLogoutWrongMethod(t *testing.T) {
	cfg := auth.NewOIDCConfig("https://kc.example", "click", "client-x", "", "https://app/cb")
	req := httptest.NewRequest("GET", "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	HandleLogout(cfg)(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleLogoutNilConfig(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	HandleLogout(nil)(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "", resp["logout_url"])
}

func TestRandomURLSafeUnique(t *testing.T) {
	a, err := randomURLSafe(32)
	require.NoError(t, err)
	b, err := randomURLSafe(32)
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.NotEmpty(t, a)
}

func TestSetAccessTokenCookieNegativeMaxAge(t *testing.T) {
	w := httptest.NewRecorder()
	setAccessTokenCookie(w, "tok", -10)
	c := findCookie(w.Result(), accessTokenCookie)
	require.NotNil(t, c)
	assert.Equal(t, 0, c.MaxAge, "negative maxAge normalized to session cookie")
}
