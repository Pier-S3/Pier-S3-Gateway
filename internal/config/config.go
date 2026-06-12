// Package config loads and validates runtime configuration for the
// S3 proxy gateway from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
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
	// OIDCAllowInsecure permits http:// (non-TLS) OIDC issuer/JWKS/discovery
	// URLs. It exists ONLY for local development against a plaintext IdP (e.g.
	// the docker-compose Keycloak). In production these URLs must be https so an
	// attacker cannot MITM the discovery document or JWKS to swap the trust
	// anchor. Sourced from OIDC_ALLOW_INSECURE ("true"/"1").
	OIDCAllowInsecure bool
	// OIDCRedirectURI is the explicit OAuth2 redirect_uri sent on the
	// server-side login flow. Set it (and allowlist the exact value at the IdP)
	// so a redirect-target regression elsewhere can never widen where tokens
	// are delivered. Sourced from OIDC_REDIRECT_URI.
	OIDCRedirectURI string
	// BucketQuotasRaw is the raw BUCKET_QUOTAS value: a comma-separated list
	// of <bucket>=<size> pairs (e.g. "logs=10GB,backups=2TB,*=100GB"). "*"
	// sets a default for buckets without an explicit entry. Display-only
	// metadata for the UI; parse with ParseBucketQuotas.
	BucketQuotasRaw string
	ListenS3Addr    string
	ListenUIAddr    string
	// ListenHealthAddr is a dedicated listener for /_health and /_ready. It is
	// separate from the S3/UI listeners so the kubelet (or compose healthcheck)
	// can probe the process while the Service/Ingress never expose the
	// endpoints publicly. Sourced from LISTEN_HEALTH_ADDR.
	ListenHealthAddr string
	LogLevel         string
}

// Default values applied by Load when an environment variable is unset or
// empty. Exported so callers and tests reference the canonical defaults
// rather than duplicating string literals.
const (
	DefaultS3Endpoint       = "http://seaweedfs:8333"
	DefaultS3Region         = "us-east-1"
	DefaultListenS3Addr     = ":8080"
	DefaultListenUIAddr     = ":8081"
	DefaultListenHealthAddr = ":8082"
	DefaultLogLevel         = "info"
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
		OIDCAllowInsecure:    isTruthy(getEnv("OIDC_ALLOW_INSECURE", "")),
		OIDCRedirectURI:      getEnv("OIDC_REDIRECT_URI", ""),
		BucketQuotasRaw:      getEnv("BUCKET_QUOTAS", ""),
		ListenS3Addr:         getEnv("LISTEN_S3_ADDR", DefaultListenS3Addr),
		ListenUIAddr:         getEnv("LISTEN_UI_ADDR", DefaultListenUIAddr),
		ListenHealthAddr:     getEnv("LISTEN_HEALTH_ADDR", DefaultListenHealthAddr),
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
		// Validate the discovery URL's scheme BEFORE fetching: discovery defines
		// the trust anchor (issuer + jwks_uri), so it must come over TLS unless
		// insecure mode is explicitly enabled for dev.
		if err := requireSecureURL("OIDC discovery URL", c.OIDCDiscoveryURL, c.OIDCAllowInsecure); err != nil {
			return OIDCSettings{}, err
		}
		disc, err := discoverOIDC(c.OIDCDiscoveryURL)
		if err != nil {
			return OIDCSettings{}, fmt.Errorf("oidc discovery: %w", err)
		}
		// Cross-check: when an explicit issuer is configured alongside
		// discovery, the discovered issuer must match it. A mismatch means the
		// discovery URL points at a different (possibly attacker-controlled)
		// IdP than the one tokens are validated against - fail closed rather
		// than silently prefer one of the two trust anchors.
		if s.Issuer != "" && disc.Issuer != "" &&
			strings.TrimRight(s.Issuer, "/") != strings.TrimRight(disc.Issuer, "/") {
			return OIDCSettings{}, fmt.Errorf(
				"oidc discovery: discovered issuer %q does not match configured issuer %q", disc.Issuer, s.Issuer)
		}
		if s.Issuer == "" {
			s.Issuer = disc.Issuer
		}
		if s.JWKSURL == "" {
			s.JWKSURL = disc.JWKSURL
		}
	}

	// Enforce TLS on the resolved issuer and JWKS endpoints. A plaintext JWKS
	// fetch is MITM-able into serving attacker keys, which would let forged
	// tokens verify. Dev against a plaintext IdP must opt in via
	// OIDC_ALLOW_INSECURE.
	if err := c.validateOIDCURLs(s); err != nil {
		return OIDCSettings{}, err
	}

	return s, nil
}

// validateOIDCURLs rejects non-TLS (or non-http(s)) issuer/JWKS URLs unless
// OIDC_ALLOW_INSECURE is set. Empty values are skipped (the required-field check
// in Validate handles missing JWKS); an empty issuer only disables the issuer
// binding, which is a separate, already-documented choice.
func (c *Config) validateOIDCURLs(s OIDCSettings) error {
	var errs []string
	for _, f := range []struct{ label, raw string }{
		{"OIDC issuer", s.Issuer},
		{"OIDC JWKS URL", s.JWKSURL},
	} {
		if err := requireSecureURL(f.label, f.raw, c.OIDCAllowInsecure); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("insecure OIDC configuration: %s (set OIDC_ALLOW_INSECURE=true only for local dev)", strings.Join(errs, "; "))
	}
	return nil
}

// requireSecureURL parses raw and requires an https scheme. http is allowed only
// when allowInsecure is true; any other scheme (or an unparseable URL) is always
// rejected. An empty raw value is treated as "not set" and passes.
func requireSecureURL(label, raw string, allowInsecure bool) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL (%q): %v", label, raw, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		if allowInsecure {
			return nil
		}
		return fmt.Errorf("%s uses insecure http (%q)", label, raw)
	default:
		return fmt.Errorf("%s has unsupported scheme %q (want https)", label, u.Scheme)
	}
}

// quotaUnits maps a size suffix to its byte multiplier (binary, 1024-based;
// the IEC spellings KiB/MiB/... are accepted as aliases).
var quotaUnits = map[string]int64{
	"":    1,
	"B":   1,
	"KB":  1 << 10,
	"KIB": 1 << 10,
	"MB":  1 << 20,
	"MIB": 1 << 20,
	"GB":  1 << 30,
	"GIB": 1 << 30,
	"TB":  1 << 40,
	"TIB": 1 << 40,
}

// ParseBucketQuotas parses BUCKET_QUOTAS ("name=10GB,other=512MB,*=1TB") into
// a bucket -> bytes map. An empty value yields an empty map. Malformed pairs
// are an error so a typo fails startup instead of silently dropping a quota.
func (c *Config) ParseBucketQuotas() (map[string]int64, error) {
	quotas := make(map[string]int64)
	raw := strings.TrimSpace(c.BucketQuotasRaw)
	if raw == "" {
		return quotas, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, sizeStr, ok := strings.Cut(pair, "=")
		name, sizeStr = strings.TrimSpace(name), strings.TrimSpace(sizeStr)
		if !ok || name == "" || sizeStr == "" {
			return nil, fmt.Errorf("BUCKET_QUOTAS: malformed entry %q (want <bucket>=<size>)", pair)
		}
		bytes, err := parseByteSize(sizeStr)
		if err != nil {
			return nil, fmt.Errorf("BUCKET_QUOTAS: entry %q: %w", pair, err)
		}
		quotas[name] = bytes
	}
	return quotas, nil
}

// parseByteSize parses "512", "10GB", "1.5TiB" (case-insensitive) into bytes.
func parseByteSize(s string) (int64, error) {
	upper := strings.ToUpper(strings.TrimSpace(s))
	num := strings.TrimRight(upper, "BKMGIT")
	unit := strings.TrimSpace(upper[len(num):])
	mult, ok := quotaUnits[unit]
	if !ok {
		return 0, fmt.Errorf("unknown size unit %q", unit)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return int64(value * float64(mult)), nil
}

// isTruthy reports whether an env value means "on" ("true"/"1"/"yes",
// case-insensitive).
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
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
