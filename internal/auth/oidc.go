package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxTokenResponseBytes caps how many bytes are read from a Keycloak token
// endpoint response. Legitimate responses are a few KiB; 1 MiB leaves ample
// headroom while preventing memory exhaustion from a hostile/oversized body.
const maxTokenResponseBytes = 1 << 20 // 1 MiB

// CookiePath scopes the auth cookies (access_token / refresh_token and the
// transient login cookies) to the API prefix so the browser does not attach
// them to static-asset requests. Every Set-Cookie and the matching clear MUST
// use this same path, otherwise the clear creates a second cookie instead of
// deleting the original.
const CookiePath = "/api/"

// OIDCConfig holds configuration for OIDC Authorization Code + PKCE flow.
type OIDCConfig struct {
	KeycloakURL  string
	Realm        string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	// Derived endpoints
	IssuerURL     string
	AuthURL       string
	TokenURL      string
	EndSessionURL string
	JWKSURL       string
}

// NewOIDCConfig creates OIDC configuration from Keycloak parameters.
func NewOIDCConfig(keycloakURL, realm, clientID, clientSecret, redirectURI string) *OIDCConfig {
	issuer := IssuerURL(keycloakURL, realm)
	base := issuer + "/protocol/openid-connect"
	return &OIDCConfig{
		KeycloakURL:   keycloakURL,
		Realm:         realm,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		RedirectURI:   redirectURI,
		IssuerURL:     issuer,
		AuthURL:       base + "/auth",
		TokenURL:      base + "/token",
		EndSessionURL: base + "/logout",
		JWKSURL:       base + "/certs",
	}
}

// IssuerURL returns the OIDC issuer ("iss" claim) value for a Keycloak realm,
// i.e. <keycloakURL>/realms/<realm>. This is the value tokens must declare and
// the value JWKSVerifier validates against.
func IssuerURL(keycloakURL, realm string) string {
	return fmt.Sprintf("%s/realms/%s", strings.TrimRight(keycloakURL, "/"), realm)
}

// TokenResponse represents the response from the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *OIDCConfig) ExchangeCode(code, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURI},
		"client_id":     {c.ClientID},
		"code_verifier": {codeVerifier},
	}
	if c.ClientSecret != "" {
		data.Set("client_secret", c.ClientSecret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(c.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	// Bound the response size: a legitimate token response is a few KiB, so
	// 1 MiB is generous. The cap protects against memory exhaustion if the
	// token endpoint is compromised, misconfigured, or SSRF'd to a hostile host.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d (error=%q)", resp.StatusCode, oauthErrorCode(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &tokenResp, nil
}

// RefreshTokens refreshes an access token using a refresh token.
func (c *OIDCConfig) RefreshTokens(refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.ClientID},
	}
	if c.ClientSecret != "" {
		data.Set("client_secret", c.ClientSecret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(c.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh returned %d (error=%q)", resp.StatusCode, oauthErrorCode(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing refresh response: %w", err)
	}

	return &tokenResp, nil
}

// oauthErrorCode extracts the standard OAuth2 "error" code from a token
// endpoint error body. The raw body is never propagated: these errors end up
// in logs, and IdP error bodies can echo request parameters or carry verbose
// diagnostics that do not belong there.
func oauthErrorCode(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err != nil || e.Error == "" {
		return "unknown"
	}
	return e.Error
}

// SetRefreshTokenCookie sets an HttpOnly cookie with the refresh token.
func SetRefreshTokenCookie(w http.ResponseWriter, refreshToken string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     CookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// ClearRefreshTokenCookie clears the refresh token cookie.
func ClearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     CookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
