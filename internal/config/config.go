// Package config loads and validates runtime configuration for the
// S3 proxy gateway from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config holds all runtime configuration for the gateway. All values are
// sourced from environment variables via Load.
type Config struct {
	S3Endpoint           string
	S3AccessKey          string
	S3SecretKey          string
	S3Region             string
	KeycloakURL          string
	KeycloakRealm        string
	KeycloakClientID     string
	KeycloakClientSecret string
	KeycloakJWKSURL      string
	// Provider-neutral OIDC settings. When set they take precedence over the
	// KEYCLOAK_* equivalents, letting any compliant OIDC IdP be configured
	// without code changes. Empty values fall back to the KEYCLOAK_* fields so
	// existing Keycloak deployments keep working unchanged. See ResolveOIDC.
	OIDCIssuer        string
	OIDCJWKSURL       string
	OIDCAudience      string
	OIDCClientID      string
	OIDCUsernameClaim string
	OIDCGroupsClaim   string
	OIDCDiscoveryURL  string
	ListenS3Addr      string
	ListenUIAddr      string
	LogLevel          string
}

// Default values applied by Load when an environment variable is unset or
// empty. Exported so callers and tests reference the canonical defaults
// rather than duplicating string literals.
const (
	DefaultS3Endpoint   = "http://seaweedfs:8333"
	DefaultS3Region     = "us-east-1"
	DefaultListenS3Addr = ":8080"
	DefaultListenUIAddr = ":8081"
	DefaultLogLevel     = "info"
)

// Load reads configuration from the environment and applies defaults.
//
// Load does not itself reject incomplete configuration; callers MUST invoke
// (*Config).Validate before using the result so missing required values fail
// fast with a clear, aggregated error. Load is kept side-effect-free and
// non-failing so it can be used in tests and tooling that only need defaults.
func Load() *Config {
	return &Config{
		S3Endpoint:           getEnv("S3_ENDPOINT", DefaultS3Endpoint),
		S3AccessKey:          getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:          getEnv("S3_SECRET_KEY", ""),
		S3Region:             getEnv("S3_REGION", DefaultS3Region),
		KeycloakURL:          getEnv("KEYCLOAK_URL", ""),
		KeycloakRealm:        getEnv("KEYCLOAK_REALM", ""),
		KeycloakClientID:     getEnv("KEYCLOAK_CLIENT_ID", ""),
		KeycloakClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		KeycloakJWKSURL:      getEnv("KEYCLOAK_JWKS_URL", ""),
		OIDCIssuer:           getEnv("OIDC_ISSUER", ""),
		OIDCJWKSURL:          getEnv("OIDC_JWKS_URL", ""),
		OIDCAudience:         getEnv("OIDC_AUDIENCE", ""),
		OIDCClientID:         getEnv("OIDC_CLIENT_ID", ""),
		OIDCUsernameClaim:    getEnv("OIDC_USERNAME_CLAIM", ""),
		OIDCGroupsClaim:      getEnv("OIDC_GROUPS_CLAIM", ""),
		OIDCDiscoveryURL:     getEnv("OIDC_DISCOVERY_URL", ""),
		ListenS3Addr:         getEnv("LISTEN_S3_ADDR", DefaultListenS3Addr),
		ListenUIAddr:         getEnv("LISTEN_UI_ADDR", DefaultListenUIAddr),
		LogLevel:             getEnv("LOG_LEVEL", DefaultLogLevel),
	}
}

// LoadAndValidate reads configuration from the environment, applies defaults,
// and validates required fields. A non-nil error means the configuration is
// unusable and the process should not start. This is the recommended entry
// point for production startup.
func LoadAndValidate() (*Config, error) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// requiredField pairs an environment variable name with the config value it
// populates, for validation error reporting.
type requiredField struct {
	envVar string
	value  string
}

// Validate ensures every required configuration field is non-empty. It
// reports all missing fields at once so operators can fix configuration in a
// single pass rather than discovering missing values one at a time.
func (c *Config) Validate() error {
	// KEYCLOAK_CLIENT_SECRET is intentionally NOT required: the OIDC layer
	// (internal/auth/oidc.go) implements Authorization Code + PKCE and only
	// sends client_secret when it is non-empty, so a public Keycloak client
	// works without one. Operators using a confidential client simply set the
	// variable; leaving it empty must not block startup.
	required := []requiredField{
		{"S3_ACCESS_KEY", c.S3AccessKey},
		{"S3_SECRET_KEY", c.S3SecretKey},
		{"KEYCLOAK_URL", c.KeycloakURL},
		{"KEYCLOAK_REALM", c.KeycloakRealm},
		{"KEYCLOAK_CLIENT_ID", c.KeycloakClientID},
		{"KEYCLOAK_JWKS_URL", c.KeycloakJWKSURL},
	}

	var missing []string
	for _, f := range required {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.envVar)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("config: missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// OIDCSettings is the fully-resolved OIDC verification configuration, after
// applying OIDC_* over KEYCLOAK_* precedence (and, when configured, discovery).
type OIDCSettings struct {
	// Issuer is the expected "iss" claim value.
	Issuer string
	// JWKSURL is the JWKS endpoint used to fetch signing keys.
	JWKSURL string
	// Audience is the expected "aud" claim value.
	Audience string
	// ClientID identifies this client at the IdP token endpoints.
	ClientID string
	// UsernameClaim / GroupsClaim are optional dotted claim paths. Empty means
	// "use the default extraction" (see auth.ClaimMapper).
	UsernameClaim string
	GroupsClaim   string
}

// ResolveOIDC computes the effective OIDC settings by layering provider-neutral
// OIDC_* values over the KEYCLOAK_* fallbacks:
//
//   - ClientID:   OIDC_CLIENT_ID,  else KEYCLOAK_CLIENT_ID
//   - Audience:   OIDC_AUDIENCE,   else the resolved ClientID
//   - Issuer:     OIDC_ISSUER,     else <KEYCLOAK_URL>/realms/<KEYCLOAK_REALM>
//   - JWKSURL:    OIDC_JWKS_URL,   else KEYCLOAK_JWKS_URL
//   - Username/Groups claim: OIDC_USERNAME_CLAIM / OIDC_GROUPS_CLAIM (may be empty)
//
// Discovery (OIDC_DISCOVERY_URL), when set, fills any Issuer/JWKSURL still empty
// after the above; explicit values always win over discovered ones.
func (c *Config) ResolveOIDC() (OIDCSettings, error) {
	clientID := firstNonEmpty(c.OIDCClientID, c.KeycloakClientID)
	s := OIDCSettings{
		ClientID:      clientID,
		Audience:      firstNonEmpty(c.OIDCAudience, clientID),
		Issuer:        firstNonEmpty(c.OIDCIssuer, deriveKeycloakIssuer(c.KeycloakURL, c.KeycloakRealm)),
		JWKSURL:       firstNonEmpty(c.OIDCJWKSURL, c.KeycloakJWKSURL),
		UsernameClaim: c.OIDCUsernameClaim,
		GroupsClaim:   c.OIDCGroupsClaim,
	}

	if c.OIDCDiscoveryURL != "" && (s.Issuer == "" || s.JWKSURL == "") {
		disc, err := discoverOIDC(c.OIDCDiscoveryURL)
		if err != nil {
			return OIDCSettings{}, fmt.Errorf("oidc discovery: %w", err)
		}
		if s.Issuer == "" {
			s.Issuer = disc.Issuer
		}
		if s.JWKSURL == "" {
			s.JWKSURL = disc.JWKSURL
		}
	}

	return s, nil
}

// firstNonEmpty returns the first argument that is non-empty after trimming.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// deriveKeycloakIssuer builds the Keycloak realm issuer URL, or "" when either
// input is empty.
func deriveKeycloakIssuer(keycloakURL, realm string) string {
	if strings.TrimSpace(keycloakURL) == "" || strings.TrimSpace(realm) == "" {
		return ""
	}
	return strings.TrimRight(keycloakURL, "/") + "/realms/" + realm
}

// SlogLevel parses the configured LogLevel string into a slog.Level.
// Recognized values (case-insensitive): "debug", "info", "warn"/"warning",
// "error". Any unrecognized value falls back to slog.LevelInfo so a
// misconfigured level never silences logging entirely.
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// getEnv returns the value of the environment variable named key, or fallback
// when the variable is unset or set to the empty string. Empty is treated as
// unset so a blank export does not override a sensible default.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
