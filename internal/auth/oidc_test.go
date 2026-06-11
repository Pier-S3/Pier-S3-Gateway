package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOIDCConfig tests endpoint derivation, including trailing-slash trimming.
func TestNewOIDCConfig(t *testing.T) {
	tests := []struct {
		name        string
		keycloakURL string
		realm       string
		wantAuth    string
		wantToken   string
		wantEndSess string
		wantJWKS    string
	}{
		{
			name:        "no trailing slash",
			keycloakURL: "https://kc.example.com",
			realm:       "click",
			wantAuth:    "https://kc.example.com/realms/click/protocol/openid-connect/auth",
			wantToken:   "https://kc.example.com/realms/click/protocol/openid-connect/token",
			wantEndSess: "https://kc.example.com/realms/click/protocol/openid-connect/logout",
			wantJWKS:    "https://kc.example.com/realms/click/protocol/openid-connect/certs",
		},
		{
			name:        "single trailing slash trimmed",
			keycloakURL: "https://kc.example.com/",
			realm:       "click",
			wantAuth:    "https://kc.example.com/realms/click/protocol/openid-connect/auth",
			wantToken:   "https://kc.example.com/realms/click/protocol/openid-connect/token",
			wantEndSess: "https://kc.example.com/realms/click/protocol/openid-connect/logout",
			wantJWKS:    "https://kc.example.com/realms/click/protocol/openid-connect/certs",
		},
		{
			name:        "multiple trailing slashes trimmed",
			keycloakURL: "https://kc.example.com///",
			realm:       "test",
			wantAuth:    "https://kc.example.com/realms/test/protocol/openid-connect/auth",
			wantToken:   "https://kc.example.com/realms/test/protocol/openid-connect/token",
			wantEndSess: "https://kc.example.com/realms/test/protocol/openid-connect/logout",
			wantJWKS:    "https://kc.example.com/realms/test/protocol/openid-connect/certs",
		},
		{
			name:        "url with path prefix",
			keycloakURL: "https://example.com/auth",
			realm:       "myrealm",
			wantAuth:    "https://example.com/auth/realms/myrealm/protocol/openid-connect/auth",
			wantToken:   "https://example.com/auth/realms/myrealm/protocol/openid-connect/token",
			wantEndSess: "https://example.com/auth/realms/myrealm/protocol/openid-connect/logout",
			wantJWKS:    "https://example.com/auth/realms/myrealm/protocol/openid-connect/certs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewOIDCConfig(tt.keycloakURL, tt.realm, "client-id", "secret", "https://app/callback")
			require.NotNil(t, cfg)
			assert.Equal(t, tt.keycloakURL, cfg.KeycloakURL)
			assert.Equal(t, tt.realm, cfg.Realm)
			assert.Equal(t, "client-id", cfg.ClientID)
			assert.Equal(t, "secret", cfg.ClientSecret)
			assert.Equal(t, "https://app/callback", cfg.RedirectURI)
			assert.Equal(t, tt.wantAuth, cfg.AuthURL)
			assert.Equal(t, tt.wantToken, cfg.TokenURL)
			assert.Equal(t, tt.wantEndSess, cfg.EndSessionURL)
			assert.Equal(t, tt.wantJWKS, cfg.JWKSURL)
		})
	}
}

// tokenEndpointStub returns a test server that records the last form values it
// received and replies with the supplied status/body.
func tokenEndpointStub(t *testing.T, status int, body string, captured *url.Values) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		if captured != nil {
			*captured = r.PostForm
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestExchangeCodeSuccess(t *testing.T) {
	var form url.Values
	srv := tokenEndpointStub(t, http.StatusOK,
		`{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":300,"id_token":"idt"}`, &form)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "shh", "https://app/cb")
	cfg.TokenURL = srv.URL

	resp, err := cfg.ExchangeCode("the-code", "the-verifier")
	require.NoError(t, err)
	assert.Equal(t, "at", resp.AccessToken)
	assert.Equal(t, "rt", resp.RefreshToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, 300, resp.ExpiresIn)
	assert.Equal(t, "idt", resp.IDToken)

	// Verify the request payload.
	assert.Equal(t, "authorization_code", form.Get("grant_type"))
	assert.Equal(t, "the-code", form.Get("code"))
	assert.Equal(t, "https://app/cb", form.Get("redirect_uri"))
	assert.Equal(t, "client-id", form.Get("client_id"))
	assert.Equal(t, "the-verifier", form.Get("code_verifier"))
	assert.Equal(t, "shh", form.Get("client_secret"))
}

func TestExchangeCodeOmitsEmptySecret(t *testing.T) {
	var form url.Values
	srv := tokenEndpointStub(t, http.StatusOK, `{"access_token":"at"}`, &form)

	cfg := NewOIDCConfig("https://kc", "click", "public-client", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	_, err := cfg.ExchangeCode("code", "verifier")
	require.NoError(t, err)
	_, present := form["client_secret"]
	assert.False(t, present, "client_secret must be omitted when empty")
}

func TestExchangeCodeNon200(t *testing.T) {
	srv := tokenEndpointStub(t, http.StatusBadRequest, `{"error":"invalid_grant"}`, nil)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	resp, err := cfg.ExchangeCode("bad", "verifier")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid_grant", "OAuth error code should be propagated")
}

// TestExchangeCodeErrorBodyNotLeaked verifies that only the parsed OAuth error
// code reaches the error string: the raw body (error_description, HTML
// diagnostics) ends up in logs and must not be propagated verbatim.
func TestExchangeCodeErrorBodyNotLeaked(t *testing.T) {
	srv := tokenEndpointStub(t, http.StatusBadRequest,
		`{"error":"invalid_grant","error_description":"sensitive-internal-detail"}`, nil)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	_, err := cfg.ExchangeCode("bad", "verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
	assert.NotContains(t, err.Error(), "sensitive-internal-detail")
}

// TestExchangeCodeNonJSONErrorBody verifies a non-JSON error body (e.g. an
// HTML error page) degrades to error="unknown" without leaking the body.
func TestExchangeCodeNonJSONErrorBody(t *testing.T) {
	srv := tokenEndpointStub(t, http.StatusBadGateway, `<html>stack trace here</html>`, nil)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	_, err := cfg.ExchangeCode("code", "verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	assert.Contains(t, err.Error(), "unknown")
	assert.NotContains(t, err.Error(), "stack trace")
}

func TestExchangeCodeBadJSON(t *testing.T) {
	srv := tokenEndpointStub(t, http.StatusOK, `{not-json`, nil)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	resp, err := cfg.ExchangeCode("code", "verifier")
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestExchangeCodeTransportError(t *testing.T) {
	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = "http://127.0.0.1:0/unreachable"

	resp, err := cfg.ExchangeCode("code", "verifier")
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestRefreshTokensSuccess(t *testing.T) {
	var form url.Values
	srv := tokenEndpointStub(t, http.StatusOK,
		`{"access_token":"new-at","refresh_token":"new-rt","expires_in":600}`, &form)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "shh", "https://app/cb")
	cfg.TokenURL = srv.URL

	resp, err := cfg.RefreshTokens("old-rt")
	require.NoError(t, err)
	assert.Equal(t, "new-at", resp.AccessToken)
	assert.Equal(t, "new-rt", resp.RefreshToken)
	assert.Equal(t, 600, resp.ExpiresIn)

	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "old-rt", form.Get("refresh_token"))
	assert.Equal(t, "client-id", form.Get("client_id"))
	assert.Equal(t, "shh", form.Get("client_secret"))
}

func TestRefreshTokensOmitsEmptySecret(t *testing.T) {
	var form url.Values
	srv := tokenEndpointStub(t, http.StatusOK, `{"access_token":"at"}`, &form)

	cfg := NewOIDCConfig("https://kc", "click", "public-client", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	_, err := cfg.RefreshTokens("rt")
	require.NoError(t, err)
	_, present := form["client_secret"]
	assert.False(t, present, "client_secret must be omitted when empty")
}

func TestRefreshTokensNon200(t *testing.T) {
	srv := tokenEndpointStub(t, http.StatusUnauthorized, `{"error":"invalid_grant"}`, nil)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	resp, err := cfg.RefreshTokens("expired-rt")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestRefreshTokensBadJSON(t *testing.T) {
	srv := tokenEndpointStub(t, http.StatusOK, `}{`, nil)

	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = srv.URL

	resp, err := cfg.RefreshTokens("rt")
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestRefreshTokensTransportError(t *testing.T) {
	cfg := NewOIDCConfig("https://kc", "click", "client-id", "", "https://app/cb")
	cfg.TokenURL = "http://127.0.0.1:0/unreachable"

	resp, err := cfg.RefreshTokens("rt")
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestSetRefreshTokenCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	SetRefreshTokenCookie(rec, "my-refresh-token", 3600)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "refresh_token", c.Name)
	assert.Equal(t, "my-refresh-token", c.Value)
	assert.Equal(t, CookiePath, c.Path)
	assert.True(t, c.HttpOnly, "cookie must be HttpOnly")
	assert.True(t, c.Secure, "cookie must be Secure")
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, 3600, c.MaxAge)
}

func TestClearRefreshTokenCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearRefreshTokenCookie(rec)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "refresh_token", c.Name)
	assert.Equal(t, "", c.Value)
	assert.Equal(t, CookiePath, c.Path)
	assert.True(t, c.HttpOnly)
	assert.True(t, c.Secure)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, -1, c.MaxAge, "MaxAge must be -1 to clear the cookie")
}
