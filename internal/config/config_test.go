package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allRequiredEnv lists the required env vars and valid placeholder values used
// to build an otherwise-complete environment for tests.
var allRequiredEnv = map[string]string{
	"S3_ACCESS_KEY":          "akey",
	"S3_SECRET_KEY":          "skey",
	"KEYCLOAK_URL":           "https://kc.example.com",
	"KEYCLOAK_REALM":         "realm",
	"KEYCLOAK_CLIENT_ID":     "client",
	"KEYCLOAK_CLIENT_SECRET": "secret",
	"KEYCLOAK_JWKS_URL":      "https://kc.example.com/jwks",
}

// optionalEnv lists the env vars that have defaults.
var optionalEnv = []string{
	"S3_ENDPOINT", "S3_REGION", "LISTEN_S3_ADDR", "LISTEN_UI_ADDR", "LOG_LEVEL",
}

// clearAllEnv unsets every config-related env var so each test starts from a
// known-empty environment. t.Setenv registers cleanup automatically.
func clearAllEnv(t *testing.T) {
	t.Helper()
	for k := range allRequiredEnv {
		t.Setenv(k, "")
	}
	for _, k := range optionalEnv {
		t.Setenv(k, "")
	}
}

// setRequiredEnv populates every required env var with a valid placeholder.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	for k, v := range allRequiredEnv {
		t.Setenv(k, v)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		setValue *string // nil means do not set (unset); otherwise set to this value
		fallback string
		want     string
	}{
		{name: "returns env value when set", setValue: strPtr("custom"), fallback: "fb", want: "custom"},
		{name: "returns fallback when unset", setValue: nil, fallback: "fb", want: "fb"},
		{name: "returns fallback when empty string", setValue: strPtr(""), fallback: "fb", want: "fb"},
		{name: "empty fallback with unset returns empty", setValue: nil, fallback: "", want: ""},
	}

	const key = "CONFIG_TEST_GETENV_KEY"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue == nil {
				// Ensure unset; t.Setenv to "" then rely on the case being "unset".
				// os.Unsetenv is needed to truly distinguish unset from empty, but
				// getEnv treats both identically, so setting "" exercises the same path.
				t.Setenv(key, "")
			} else {
				t.Setenv(key, *tt.setValue)
			}
			got := getEnv(key, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	clearAllEnv(t)

	cfg := Load()
	require.NotNil(t, cfg)

	// Optional fields fall back to documented defaults.
	assert.Equal(t, DefaultS3Endpoint, cfg.S3Endpoint)
	assert.Equal(t, "http://seaweedfs:8333", cfg.S3Endpoint)
	assert.Equal(t, DefaultS3Region, cfg.S3Region)
	assert.Equal(t, "us-east-1", cfg.S3Region)
	assert.Equal(t, DefaultListenS3Addr, cfg.ListenS3Addr)
	assert.Equal(t, ":8080", cfg.ListenS3Addr)
	assert.Equal(t, DefaultListenUIAddr, cfg.ListenUIAddr)
	assert.Equal(t, ":8081", cfg.ListenUIAddr)
	assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
	assert.Equal(t, "info", cfg.LogLevel)

	// Required fields with no env are empty (Load does not fail on its own).
	assert.Empty(t, cfg.S3AccessKey)
	assert.Empty(t, cfg.S3SecretKey)
	assert.Empty(t, cfg.KeycloakURL)
	assert.Empty(t, cfg.KeycloakRealm)
	assert.Empty(t, cfg.KeycloakClientID)
	assert.Empty(t, cfg.KeycloakClientSecret)
	assert.Empty(t, cfg.KeycloakJWKSURL)
}

func TestLoadFromEnv(t *testing.T) {
	clearAllEnv(t)
	setRequiredEnv(t)
	t.Setenv("S3_ENDPOINT", "http://localhost:9000")
	t.Setenv("S3_REGION", "eu-west-1")
	t.Setenv("LISTEN_S3_ADDR", ":9090")
	t.Setenv("LISTEN_UI_ADDR", ":9091")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := Load()
	require.NotNil(t, cfg)

	assert.Equal(t, "http://localhost:9000", cfg.S3Endpoint)
	assert.Equal(t, "akey", cfg.S3AccessKey)
	assert.Equal(t, "skey", cfg.S3SecretKey)
	assert.Equal(t, "eu-west-1", cfg.S3Region)
	assert.Equal(t, "https://kc.example.com", cfg.KeycloakURL)
	assert.Equal(t, "realm", cfg.KeycloakRealm)
	assert.Equal(t, "client", cfg.KeycloakClientID)
	assert.Equal(t, "secret", cfg.KeycloakClientSecret)
	assert.Equal(t, "https://kc.example.com/jwks", cfg.KeycloakJWKSURL)
	assert.Equal(t, ":9090", cfg.ListenS3Addr)
	assert.Equal(t, ":9091", cfg.ListenUIAddr)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestValidate(t *testing.T) {
	base := func() *Config {
		return &Config{
			S3AccessKey:          "a",
			S3SecretKey:          "s",
			KeycloakURL:          "u",
			KeycloakRealm:        "r",
			KeycloakClientID:     "id",
			KeycloakClientSecret: "sec",
			KeycloakJWKSURL:      "jwks",
		}
	}

	tests := []struct {
		name        string
		mutate      func(*Config)
		wantErr     bool
		errContains []string
	}{
		{
			name:    "all required present",
			mutate:  func(*Config) {},
			wantErr: false,
		},
		{
			name:    "empty client secret is allowed (public client + PKCE)",
			mutate:  func(c *Config) { c.KeycloakClientSecret = "" },
			wantErr: false,
		},
		{
			name:        "missing access key",
			mutate:      func(c *Config) { c.S3AccessKey = "" },
			wantErr:     true,
			errContains: []string{"S3_ACCESS_KEY"},
		},
		{
			name:        "whitespace-only secret treated as missing",
			mutate:      func(c *Config) { c.S3SecretKey = "   " },
			wantErr:     true,
			errContains: []string{"S3_SECRET_KEY"},
		},
		{
			name: "multiple missing reported together",
			mutate: func(c *Config) {
				c.KeycloakURL = ""
				c.KeycloakJWKSURL = ""
			},
			wantErr:     true,
			errContains: []string{"KEYCLOAK_URL", "KEYCLOAK_JWKS_URL"},
		},
		{
			name: "all missing",
			mutate: func(c *Config) {
				*c = Config{}
			},
			wantErr: true,
			errContains: []string{
				"S3_ACCESS_KEY", "S3_SECRET_KEY", "KEYCLOAK_URL", "KEYCLOAK_REALM",
				"KEYCLOAK_CLIENT_ID", "KEYCLOAK_JWKS_URL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mutate(c)
			err := c.Validate()
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, sub := range tt.errContains {
				assert.Contains(t, err.Error(), sub)
			}
		})
	}
}

func TestLoadAndValidate(t *testing.T) {
	t.Run("succeeds with full env", func(t *testing.T) {
		clearAllEnv(t)
		setRequiredEnv(t)

		cfg, err := LoadAndValidate()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "akey", cfg.S3AccessKey)
	})

	t.Run("fails when required missing", func(t *testing.T) {
		clearAllEnv(t)

		cfg, err := LoadAndValidate()
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "S3_ACCESS_KEY")
	})
}

func TestSlogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		want     slog.Level
	}{
		{name: "debug", logLevel: "debug", want: slog.LevelDebug},
		{name: "info", logLevel: "info", want: slog.LevelInfo},
		{name: "warn", logLevel: "warn", want: slog.LevelWarn},
		{name: "warning alias", logLevel: "warning", want: slog.LevelWarn},
		{name: "error", logLevel: "error", want: slog.LevelError},
		{name: "uppercase", logLevel: "DEBUG", want: slog.LevelDebug},
		{name: "mixed case with spaces", logLevel: "  Error ", want: slog.LevelError},
		{name: "empty falls back to info", logLevel: "", want: slog.LevelInfo},
		{name: "unknown falls back to info", logLevel: "verbose", want: slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{LogLevel: tt.logLevel}
			assert.Equal(t, tt.want, c.SlogLevel())
		})
	}
}

func strPtr(s string) *string { return &s }
