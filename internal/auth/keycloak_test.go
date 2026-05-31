package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKID = "test-key-1"

// generateRSAKeyPair generates an RSA public/private key pair for testing.
func generateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// encodeJWK builds a single RSA JWK with correct base64url-encoded big-endian
// modulus (n) and exponent (e), as required by RFC 7518.
func encodeJWK(publicKey *rsa.PublicKey, kid string) map[string]interface{} {
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())

	// Encode the exponent as a big-endian byte slice with no leading zeros.
	eBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(eBuf, uint64(publicKey.E))
	i := 0
	for i < len(eBuf)-1 && eBuf[i] == 0 {
		i++
	}
	e := base64.RawURLEncoding.EncodeToString(eBuf[i:])

	return map[string]interface{}{
		"kty": "RSA",
		"kid": kid,
		"n":   n,
		"e":   e,
		"use": "sig",
		"alg": "RS256",
	}
}

// createMockJWKSServer creates a mock JWKS endpoint that serves a public key as
// a valid JWK. It serves on every path so callers can point any JWKS URL at it.
func createMockJWKSServer(publicKey *rsa.PublicKey, kid string) *httptest.Server {
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{encodeJWK(publicKey, kid)},
	}
	body, _ := json.Marshal(jwks)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

// createSignedToken creates a JWT token signed with the private key and a kid header.
func createSignedToken(privateKey *rsa.PrivateKey, claims jwt.MapClaims, expiresAt time.Time) (string, error) {
	return createSignedTokenWithMethod(privateKey, claims, expiresAt, jwt.SigningMethodRS256, testKID)
}

func createSignedTokenWithMethod(privateKey *rsa.PrivateKey, claims jwt.MapClaims, expiresAt time.Time, method jwt.SigningMethod, kid string) (string, error) {
	merged := jwt.MapClaims{"exp": expiresAt.Unix()}
	for k, v := range claims {
		merged[k] = v
	}
	token := jwt.NewWithClaims(method, merged)
	if kid != "" {
		token.Header["kid"] = kid
	}
	return token.SignedString(privateKey)
}

// newTestVerifier stands up a JWKS server for publicKey and wires a real
// JWKSVerifier at it. The server and verifier are cleaned up via t.Cleanup.
func newTestVerifier(t *testing.T, publicKey *rsa.PublicKey, kid string) *JWKSVerifier {
	t.Helper()
	srv := createMockJWKSServer(publicKey, kid)
	t.Cleanup(srv.Close)

	// Issuer/audience checks disabled here so the bulk of the signature/
	// expiry tests need not embed iss/aud; dedicated tests below exercise the
	// issuer/audience enforcement explicitly.
	v, err := NewJWKSVerifier(srv.URL+"/realms/test/protocol/openid-connect/certs", "", "")
	require.NoError(t, err, "NewJWKSVerifier should succeed against mock JWKS server")
	t.Cleanup(v.Close)
	return v
}

// newTestVerifierWithClaims stands up a JWKS server and a verifier that
// enforces the given expected issuer and audience.
func newTestVerifierWithClaims(t *testing.T, publicKey *rsa.PublicKey, kid, issuer, audience string) *JWKSVerifier {
	t.Helper()
	srv := createMockJWKSServer(publicKey, kid)
	t.Cleanup(srv.Close)

	v, err := NewJWKSVerifier(srv.URL+"/realms/test/protocol/openid-connect/certs", issuer, audience)
	require.NoError(t, err, "NewJWKSVerifier should succeed against mock JWKS server")
	t.Cleanup(v.Close)
	return v
}

// TestNewJWKSVerifierError verifies that pointing the verifier at an
// unreachable/invalid JWKS URL surfaces an error rather than a nil verifier.
func TestNewJWKSVerifierError(t *testing.T) {
	// Disable the startup retry window so the unreachable case fails fast
	// (single attempt) instead of retrying for the production default.
	orig := jwksInitMaxWait
	jwksInitMaxWait = 0
	defer func() { jwksInitMaxWait = orig }()

	v, err := NewJWKSVerifier("http://127.0.0.1:0/does-not-exist", "", "")
	assert.Error(t, err, "unreachable JWKS URL should error")
	assert.Nil(t, v, "verifier should be nil on error")
}

// TestNewJWKSVerifierRetriesThenSucceeds verifies that a JWKS endpoint which is
// briefly unavailable (e.g. Keycloak still starting) is tolerated: the verifier
// retries and succeeds once the endpoint comes up, rather than failing at boot.
func TestNewJWKSVerifierRetriesThenSucceeds(t *testing.T) {
	_, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	jwksBody, _ := json.Marshal(map[string]interface{}{
		"keys": []map[string]interface{}{encodeJWK(publicKey, testKID)},
	})

	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Simulate Keycloak not ready yet.
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	// Shrink retry timing so the test is fast but still exercises >1 attempt.
	origWait, origDelay := jwksInitMaxWait, jwksInitRetryDelay
	jwksInitMaxWait = 5 * time.Second
	jwksInitRetryDelay = 20 * time.Millisecond
	defer func() { jwksInitMaxWait, jwksInitRetryDelay = origWait, origDelay }()

	v, err := NewJWKSVerifier(srv.URL+"/realms/test/protocol/openid-connect/certs", "", "")
	require.NoError(t, err, "verifier should succeed once JWKS endpoint becomes available")
	require.NotNil(t, v)
	defer v.Close()
	assert.GreaterOrEqual(t, attempts, 3, "should have retried until the endpoint served keys")
}

// TestVerifyTokenValid exercises the real NewJWKSVerifier + VerifyToken path.
func TestVerifyTokenValid(t *testing.T) {
	privateKey, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	v := newTestVerifier(t, publicKey, testKID)

	claims := jwt.MapClaims{
		"sub":                "user-123",
		"preferred_username": "testuser",
		"email":              "test@example.com",
		"groups":             []interface{}{"reports-ro", "dev-rw"},
	}
	tokenString, err := createSignedToken(privateKey, claims, time.Now().Add(time.Hour))
	require.NoError(t, err)

	got, err := v.VerifyToken(tokenString)
	require.NoError(t, err, "valid token should verify")
	assert.Equal(t, "user-123", got["sub"])
	assert.Equal(t, "testuser", got["preferred_username"])
	assert.Equal(t, "test@example.com", got["email"])
}

// TestVerifyTokenExpired ensures expired tokens are rejected by VerifyToken.
func TestVerifyTokenExpired(t *testing.T) {
	privateKey, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	v := newTestVerifier(t, publicKey, testKID)

	tokenString, err := createSignedToken(privateKey, jwt.MapClaims{"sub": "user-456"}, time.Now().Add(-time.Hour))
	require.NoError(t, err)

	claims, err := v.VerifyToken(tokenString)
	assert.Error(t, err, "expired token should be rejected")
	assert.Nil(t, claims)
}

// TestVerifyTokenMissingExp ensures tokens without exp are rejected
// (WithExpirationRequired) by VerifyToken.
func TestVerifyTokenMissingExp(t *testing.T) {
	privateKey, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	v := newTestVerifier(t, publicKey, testKID)

	// Build a token with NO exp claim.
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "no-exp"})
	token.Header["kid"] = testKID
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	claims, err := v.VerifyToken(tokenString)
	assert.Error(t, err, "token without exp should be rejected")
	assert.Nil(t, claims)
}

// TestVerifyTokenWrongKey ensures a token signed by a key not in the JWKS is rejected.
func TestVerifyTokenWrongKey(t *testing.T) {
	privateKey1, _, err := generateRSAKeyPair()
	require.NoError(t, err)
	_, publicKey2, err := generateRSAKeyPair()
	require.NoError(t, err)

	// JWKS only knows publicKey2; token is signed by privateKey1 but advertises
	// a kid that is not present, forcing keyfunc to reject it.
	v := newTestVerifier(t, publicKey2, testKID)

	tokenString, err := createSignedTokenWithMethod(privateKey1, jwt.MapClaims{"sub": "u"}, time.Now().Add(time.Hour), jwt.SigningMethodRS256, "unknown-kid")
	require.NoError(t, err)

	claims, err := v.VerifyToken(tokenString)
	assert.Error(t, err, "token with unknown signing key should be rejected")
	assert.Nil(t, claims)
}

// TestVerifyTokenWrongSignatureSameKID ensures a token with a matching kid but
// signed by a different private key fails the signature check.
func TestVerifyTokenWrongSignatureSameKID(t *testing.T) {
	privateKey1, _, err := generateRSAKeyPair()
	require.NoError(t, err)
	_, publicKey2, err := generateRSAKeyPair()
	require.NoError(t, err)

	v := newTestVerifier(t, publicKey2, testKID)

	// Same kid as the JWKS, but signed with the wrong key.
	tokenString, err := createSignedToken(privateKey1, jwt.MapClaims{"sub": "u"}, time.Now().Add(time.Hour))
	require.NoError(t, err)

	claims, err := v.VerifyToken(tokenString)
	assert.Error(t, err, "wrong signature should be rejected")
	assert.Nil(t, claims)
}

// TestVerifyTokenMalformed covers empty and malformed token strings.
func TestVerifyTokenMalformed(t *testing.T) {
	_, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	v := newTestVerifier(t, publicKey, testKID)

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt"},
		{"too-many-segments", "not.a.valid.jwt.token.format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := v.VerifyToken(tt.token)
			assert.Error(t, err, "malformed token should be rejected")
			assert.Nil(t, claims)
		})
	}
}

// TestVerifyTokenRejectsHS256 ensures the method allow-list inside VerifyToken
// rejects non-RSA signing methods (alg confusion protection).
func TestVerifyTokenRejectsHS256(t *testing.T) {
	_, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	v := newTestVerifier(t, publicKey, testKID)

	// Forge an HS256 token (the secret is irrelevant; it must be rejected before
	// signature verification because the method is not allowed).
	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "attacker",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	hsToken.Header["kid"] = testKID
	tokenString, err := hsToken.SignedString([]byte("secret"))
	require.NoError(t, err)

	claims, err := v.VerifyToken(tokenString)
	assert.Error(t, err, "HS256 token must be rejected by method allow-list")
	assert.Nil(t, claims)
}

// TestVerifyTokenIssuerAudience exercises issuer/audience enforcement: a token
// is accepted only when both iss and aud match the verifier's configuration,
// and rejected if either is wrong or missing.
func TestVerifyTokenIssuerAudience(t *testing.T) {
	privateKey, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	const wantIssuer = "https://keycloak.example.com/realms/test"
	const wantAudience = "s3-proxy"

	v := newTestVerifierWithClaims(t, publicKey, testKID, wantIssuer, wantAudience)

	tests := []struct {
		name    string
		iss     interface{}
		aud     interface{}
		wantErr bool
	}{
		{name: "matching iss and aud", iss: wantIssuer, aud: wantAudience, wantErr: false},
		{name: "wrong issuer (other realm)", iss: "https://keycloak.example.com/realms/other", aud: wantAudience, wantErr: true},
		{name: "wrong audience (other client)", iss: wantIssuer, aud: "other-client", wantErr: true},
		{name: "missing issuer", iss: nil, aud: wantAudience, wantErr: true},
		{name: "missing audience", iss: wantIssuer, aud: nil, wantErr: true},
		{name: "matching aud among multiple", iss: wantIssuer, aud: []string{"other", wantAudience}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := jwt.MapClaims{"sub": "user-1"}
			if tt.iss != nil {
				claims["iss"] = tt.iss
			}
			if tt.aud != nil {
				claims["aud"] = tt.aud
			}
			tokenString, err := createSignedToken(privateKey, claims, time.Now().Add(time.Hour))
			require.NoError(t, err)

			got, err := v.VerifyToken(tokenString)
			if tt.wantErr {
				assert.Error(t, err, "token should be rejected")
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err, "token should be accepted")
				assert.NotNil(t, got)
			}
		})
	}
}

// TestTokenVerifierInterface verifies that JWKSVerifier implements TokenVerifier.
func TestTokenVerifierInterface(t *testing.T) {
	var _ TokenVerifier = (*JWKSVerifier)(nil)
}

// TestEncodeJWKRoundTrip sanity-checks that the JWK we serve in tests decodes
// back to the original public key (guards the test helper itself).
func TestEncodeJWKRoundTrip(t *testing.T) {
	_, publicKey, err := generateRSAKeyPair()
	require.NoError(t, err)

	jwk := encodeJWK(publicKey, testKID)

	nBytes, err := base64.RawURLEncoding.DecodeString(jwk["n"].(string))
	require.NoError(t, err)
	assert.Equal(t, 0, new(big.Int).SetBytes(nBytes).Cmp(publicKey.N), "modulus should round-trip")

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk["e"].(string))
	require.NoError(t, err)
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	assert.Equal(t, publicKey.E, e, "exponent should round-trip")
}
