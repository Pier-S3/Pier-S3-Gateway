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
	ListenS3Addr         string
	ListenUIAddr         string
	LogLevel             string
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
