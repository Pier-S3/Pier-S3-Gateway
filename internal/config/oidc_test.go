package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveOIDCPrecedence verifies OIDC_* over KEYCLOAK_* layering without
// discovery.
func TestResolveOIDCPrecedence(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want OIDCSettings
	}{
		{
			name: "keycloak fallbacks only",
			cfg: Config{
				KeycloakURL:      "https://kc.example.com",
				KeycloakRealm:    "click",
				KeycloakClientID: "s3-proxy",
				KeycloakJWKSURL:  "https://kc.example.com/realms/click/protocol/openid-connect/certs",
			},
			want: OIDCSettings{
				ClientID: "s3-proxy",
				Audience: "s3-proxy",
				Issuer:   "https://kc.example.com/realms/click",
				JWKSURL:  "https://kc.example.com/realms/click/protocol/openid-connect/certs",
			},
		},
		{
			name: "oidc values win over keycloak",
			cfg: Config{
				KeycloakURL:       "https://kc.example.com",
				KeycloakRealm:     "click",
				KeycloakClientID:  "kc-client",
				KeycloakJWKSURL:   "https://kc/jwks",
				OIDCIssuer:        "https://idp.example.com/",
				OIDCJWKSURL:       "https://idp.example.com/jwks",
				OIDCAudience:      "my-aud",
				OIDCClientID:      "oidc-client",
				OIDCUsernameClaim: "email",
				OIDCGroupsClaim:   "roles",
			},
			want: OIDCSettings{
				ClientID:      "oidc-client",
				Audience:      "my-aud",
				Issuer:        "https://idp.example.com/",
				JWKSURL:       "https://idp.example.com/jwks",
				UsernameClaim: "email",
				GroupsClaim:   "roles",
			},
		},
		{
			name: "audience falls back to resolved client id",
			cfg: Config{
				OIDCIssuer:   "https://idp/",
				OIDCJWKSURL:  "https://idp/jwks",
				OIDCClientID: "the-client",
			},
			want: OIDCSettings{
				ClientID: "the-client",
				Audience: "the-client",
				Issuer:   "https://idp/",
				JWKSURL:  "https://idp/jwks",
			},
		},
		{
			name: "trailing slash trimmed when deriving keycloak issuer",
			cfg: Config{
				KeycloakURL:      "https://kc.example.com/",
				KeycloakRealm:    "test",
				KeycloakClientID: "c",
				KeycloakJWKSURL:  "https://kc/jwks",
			},
			want: OIDCSettings{
				ClientID: "c",
				Audience: "c",
				Issuer:   "https://kc.example.com/realms/test",
				JWKSURL:  "https://kc/jwks",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.ResolveOIDC()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// discoveryServer returns a test server serving an OIDC discovery document at
// the well-known path.
func discoveryServer(t *testing.T, issuer, jwks string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wellKnownPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + issuer + `","jwks_uri":"` + jwks + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveOIDCDiscoveryFillsMissing(t *testing.T) {
	srv := discoveryServer(t, "https://disc.example.com", "https://disc.example.com/jwks")

	cfg := Config{
		OIDCClientID:     "client",
		OIDCDiscoveryURL: srv.URL, // base URL; well-known path is appended
		// httptest serves over http; allow it for this mechanics test.
		OIDCAllowInsecure: true,
	}
	got, err := cfg.ResolveOIDC()
	require.NoError(t, err)
	assert.Equal(t, "https://disc.example.com", got.Issuer)
	assert.Equal(t, "https://disc.example.com/jwks", got.JWKSURL)
	assert.Equal(t, "client", got.ClientID)
	assert.Equal(t, "client", got.Audience)
}

func TestResolveOIDCExplicitWinsOverDiscovery(t *testing.T) {
	srv := discoveryServer(t, "https://disc.example.com", "https://disc.example.com/jwks")

	cfg := Config{
		OIDCIssuer:       "https://explicit.example.com",
		OIDCJWKSURL:      "https://explicit.example.com/jwks",
		OIDCClientID:     "client",
		OIDCDiscoveryURL: srv.URL,
	}
	got, err := cfg.ResolveOIDC()
	require.NoError(t, err)
	assert.Equal(t, "https://explicit.example.com", got.Issuer)
	assert.Equal(t, "https://explicit.example.com/jwks", got.JWKSURL)
}

func TestResolveOIDCDiscoveryNotCalledWhenComplete(t *testing.T) {
	// Point discovery at an unreachable URL; it must not be contacted because
	// both issuer and jwks are already set, so resolution still succeeds.
	cfg := Config{
		OIDCIssuer:       "https://idp",
		OIDCJWKSURL:      "https://idp/jwks",
		OIDCClientID:     "client",
		OIDCDiscoveryURL: "http://127.0.0.1:0/unreachable",
	}
	got, err := cfg.ResolveOIDC()
	require.NoError(t, err)
	assert.Equal(t, "https://idp", got.Issuer)
}

func TestResolveOIDCDiscoveryErrorPropagates(t *testing.T) {
	cfg := Config{
		OIDCClientID:      "client",
		OIDCDiscoveryURL:  "http://127.0.0.1:0/unreachable",
		OIDCAllowInsecure: true, // so the http scheme check passes and the fetch is attempted
	}
	_, err := cfg.ResolveOIDC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc discovery")
}

// TestResolveOIDCRejectsInsecureURLs verifies that http issuer/JWKS/discovery
// URLs are rejected by default (TLS required) and permitted only when
// OIDCAllowInsecure is set.
func TestResolveOIDCRejectsInsecureURLs(t *testing.T) {
	t.Run("insecure issuer/jwks rejected by default", func(t *testing.T) {
		cfg := Config{
			OIDCClientID: "client",
			OIDCIssuer:   "http://idp.example.com",
			OIDCJWKSURL:  "http://idp.example.com/jwks",
		}
		_, err := cfg.ResolveOIDC()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insecure")
	})

	t.Run("insecure permitted with allow flag", func(t *testing.T) {
		cfg := Config{
			OIDCClientID:      "client",
			OIDCIssuer:        "http://idp.example.com",
			OIDCJWKSURL:       "http://idp.example.com/jwks",
			OIDCAllowInsecure: true,
		}
		got, err := cfg.ResolveOIDC()
		require.NoError(t, err)
		assert.Equal(t, "http://idp.example.com", got.Issuer)
	})

	t.Run("non-http scheme always rejected", func(t *testing.T) {
		cfg := Config{
			OIDCClientID:      "client",
			OIDCIssuer:        "https://idp.example.com",
			OIDCJWKSURL:       "file:///etc/passwd",
			OIDCAllowInsecure: true,
		}
		_, err := cfg.ResolveOIDC()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scheme")
	})

	t.Run("insecure discovery URL rejected by default", func(t *testing.T) {
		cfg := Config{
			OIDCClientID:     "client",
			OIDCDiscoveryURL: "http://disc.example.com",
		}
		_, err := cfg.ResolveOIDC()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OIDC discovery URL")
	})
}

func TestDiscoverOIDCFullWellKnownURL(t *testing.T) {
	srv := discoveryServer(t, "https://disc", "https://disc/jwks")

	doc, err := discoverOIDC(srv.URL + wellKnownPath)
	require.NoError(t, err)
	assert.Equal(t, "https://disc", doc.Issuer)
	assert.Equal(t, "https://disc/jwks", doc.JWKSURL)
}

func TestDiscoverOIDCNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := discoverOIDC(srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDiscoverOIDCBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer srv.Close()

	_, err := discoverOIDC(srv.URL)
	require.Error(t, err)
}

func TestDiscoverOIDCEmptyDocument(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := discoverOIDC(srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither issuer nor jwks_uri")
}
