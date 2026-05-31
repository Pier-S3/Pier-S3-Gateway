package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

// JWKS initialization retry tuning. These are vars (not consts) so tests can
// lower the window to keep failure-path cases fast. The defaults let the
// gateway survive a Keycloak that is still starting up (a common race in
// docker-compose / Kubernetes) instead of crash-looping on first boot.
var (
	// jwksInitMaxWait bounds total time spent retrying the initial JWKS fetch.
	jwksInitMaxWait = 60 * time.Second
	// jwksInitRetryDelay is the pause between initial-fetch attempts.
	jwksInitRetryDelay = 3 * time.Second
)

// JWKSVerifier verifies JWT tokens using JWKS from Keycloak.
type JWKSVerifier struct {
	jwks *keyfunc.JWKS
	// expectedIssuer is the realm URL that legitimate tokens must declare in
	// their "iss" claim, e.g. https://keycloak/realms/myrealm. Empty disables
	// the issuer check (used only by tests that cannot know the issuer).
	expectedIssuer string
	// expectedAudience is the client ID that legitimate tokens must declare in
	// their "aud" claim. Empty disables the audience check.
	expectedAudience string
}

// NewJWKSVerifier creates a new verifier that fetches JWKS keys from the given
// URL and validates the issuer and audience of every token.
//
// expectedIssuer is the realm URL (e.g. https://keycloak/realms/myrealm) that
// tokens must declare in "iss"; expectedAudience is the client ID that tokens
// must declare in "aud". Both are required in production so that a validly
// signed token minted for a different realm or client cannot authenticate to
// the gateway. An empty value disables the corresponding check; this is
// intended only for tests.
//
// Keys are cached and refreshed automatically.
func NewJWKSVerifier(jwksURL, expectedIssuer, expectedAudience string) (*JWKSVerifier, error) {
	opts := keyfunc.Options{
		// context.Background (not a short-lived cancelable context) so the
		// background key-refresh goroutine lives for the process lifetime;
		// it is stopped explicitly by Close -> EndBackground. Per-request
		// timeouts are governed by RefreshTimeout below.
		Ctx:               context.Background(),
		RefreshInterval:   5 * time.Minute,
		RefreshRateLimit:  1 * time.Minute,
		RefreshTimeout:    10 * time.Second,
		RefreshUnknownKID: true,
	}

	// Retry the initial fetch with a fixed backoff so a Keycloak that is not
	// yet listening (startup race) does not crash the gateway. We always make
	// at least one attempt; a zero/negative window degrades to single-shot,
	// which tests rely on for fast failure cases.
	deadline := time.Now().Add(jwksInitMaxWait)
	var jwks *keyfunc.JWKS
	var err error
	for attempt := 1; ; attempt++ {
		jwks, err = keyfunc.Get(jwksURL, opts)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("failed to create JWKS from %s after %d attempt(s): %w", jwksURL, attempt, err)
		}
		slog.Warn("JWKS init failed, retrying",
			"url", jwksURL,
			"attempt", attempt,
			"retry_in", jwksInitRetryDelay.String(),
			"error", err.Error(),
		)
		time.Sleep(jwksInitRetryDelay)
	}

	return &JWKSVerifier{
		jwks:             jwks,
		expectedIssuer:   expectedIssuer,
		expectedAudience: expectedAudience,
	}, nil
}

// VerifyToken verifies a JWT token string and returns the parsed claims.
// Returns error if the token is invalid, expired, has an unknown signing key,
// or fails the configured issuer/audience checks.
func (v *JWKSVerifier) VerifyToken(tokenString string) (jwt.MapClaims, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
		jwt.WithExpirationRequired(),
	}
	// Enforce issuer/audience binding so a validly signed token issued for a
	// different realm or client cannot be replayed against the gateway.
	if v.expectedIssuer != "" {
		opts = append(opts, jwt.WithIssuer(v.expectedIssuer))
	}
	if v.expectedAudience != "" {
		opts = append(opts, jwt.WithAudience(v.expectedAudience))
	}

	token, err := jwt.Parse(tokenString, v.jwks.Keyfunc, opts...)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// Close releases resources used by the JWKS verifier.
func (v *JWKSVerifier) Close() {
	v.jwks.EndBackground()
}

// TokenVerifier is an interface for verifying JWT tokens (useful for testing).
type TokenVerifier interface {
	VerifyToken(tokenString string) (jwt.MapClaims, error)
}
