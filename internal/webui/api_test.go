package webui

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTokenVerifier is a mock implementation of TokenVerifier for testing.
type MockTokenVerifier struct {
	claims     jwt.MapClaims
	err        error
	verifyFunc func(tokenString string) (jwt.MapClaims, error)
}

// VerifyToken implements TokenVerifier.VerifyToken.
func (m *MockTokenVerifier) VerifyToken(tokenString string) (jwt.MapClaims, error) {
	if m.verifyFunc != nil {
		return m.verifyFunc(tokenString)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

// TestGetListBucketsWithoutToken tests GET /api/v1/buckets without authentication.
func TestGetListBucketsWithoutToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		err: jwt.ErrSignatureInvalid,
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleListBuckets))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401 without token")

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized", errResp["error"])
	assert.Equal(t, "missing authentication", errResp["message"])
}

// TestGetListBucketsWithInvalidToken tests GET /api/v1/buckets with invalid token.
func TestGetListBucketsWithInvalidToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		err: jwt.ErrSignatureInvalid,
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleListBuckets))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401 with invalid token")

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_token", errResp["error"])
	assert.Equal(t, "token verification failed", errResp["message"])
}

// TestGetListBucketsWithValidToken tests GET /api/v1/buckets with valid token.
func TestGetListBucketsWithValidToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-ro", "dev-rw"},
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleListBuckets))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "should return 200 with valid token")

	var buckets []BucketResponse
	err := json.NewDecoder(w.Body).Decode(&buckets)
	require.NoError(t, err)

	// Verify bucket responses are correct
	bucketMap := make(map[string]BucketResponse)
	for _, b := range buckets {
		bucketMap[b.Name] = b
	}

	assert.Len(t, bucketMap, 2, "should have 2 buckets")
	assert.True(t, bucketMap["reports"].Permissions.Read)
	assert.False(t, bucketMap["reports"].Permissions.Write)
	assert.True(t, bucketMap["dev"].Permissions.Read)
	assert.True(t, bucketMap["dev"].Permissions.Write)
}

// TestGetAuthMeWithValidToken tests GET /api/v1/auth/me with valid token.
func TestGetAuthMeWithValidToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":                "user-123",
			"preferred_username": "testuser",
			"groups":             []interface{}{"reports-ro", "dev-rw"},
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(HandleAuthMe))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "should return 200")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "testuser", resp["username"], "should return username")
	groups, ok := resp["groups"].([]interface{})
	require.True(t, ok, "groups should be array")
	assert.Len(t, groups, 2, "should have 2 groups")
	assert.Equal(t, "reports-ro", groups[0])
	assert.Equal(t, "dev-rw", groups[1])
}

// TestGetAuthMeWithoutToken tests GET /api/v1/auth/me without token.
func TestGetAuthMeWithoutToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		err: jwt.ErrSignatureInvalid,
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(HandleAuthMe))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401")
}

// TestGetAuthMeWithSubOnly tests GET /api/v1/auth/me with only sub claim.
func TestGetAuthMeWithSubOnly(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub": "user-456",
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(HandleAuthMe))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "user-456", resp["username"], "should use sub when preferred_username is missing")
}

// TestPutObjectToReadOnlyBucket tests PUT to read-only bucket returns 403.
func TestPutObjectToReadOnlyBucket(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("PUT", "/api/v1/buckets/reports/objects/test.txt", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-ro"}, // read-only
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandlePutObject))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 for write on read-only")

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "write_not_allowed", errResp["error"])
	assert.Equal(t, "no write access to bucket", errResp["message"])
}

// TestPutObjectToReadWriteBucket tests PUT to read-write bucket is allowed.
func TestPutObjectToReadWriteBucket(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("PUT", "/api/v1/buckets/reports/objects/test.txt", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-rw"}, // read-write
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandlePutObject))
	handler.ServeHTTP(w, req)

	// Note: Will return 502 because mock S3Client is nil, but not 403
	assert.NotEqual(t, http.StatusForbidden, w.Code, "should not deny write on read-write bucket")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code, "should not deny authentication")
}

// TestDeleteObjectWithoutPermission tests DELETE without permission returns 403.
func TestDeleteObjectWithoutPermission(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("DELETE", "/api/v1/buckets/reports/objects/test.txt", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-ro"}, // read-only
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleDeleteObject))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 for delete on read-only")

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "write_not_allowed", errResp["error"])
}

// TestGetObjectAllowed tests GET is allowed on read-only bucket.
func TestGetObjectAllowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects/test.txt", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-ro"},
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleGetObject))
	handler.ServeHTTP(w, req)

	// Will return 502 because mock S3Client is nil, but not 403
	assert.NotEqual(t, http.StatusForbidden, w.Code, "GET should be allowed on read-only")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

// TestAuthMiddlewareContextStorage tests that auth middleware stores values in context.
func TestAuthMiddlewareContextStorage(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":                "user-123",
			"preferred_username": "testuser",
			"groups":             []interface{}{"reports-ro"},
		},
	}

	authMw := AuthMiddleware(verifier)

	contextCheckHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUsername(r)
		groups := GetGroups(r)

		assert.Equal(t, "testuser", username, "username should be in context")
		assert.Equal(t, []string{"reports-ro"}, groups, "groups should be in context")

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler := authMw(contextCheckHandler)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestMultipleGroupsAccess tests user with multiple groups accessing different buckets.
func TestMultipleGroupsAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-ro", "dev-rw"},
		},
	}

	tests := []struct {
		name          string
		path          string
		method        string
		expectAllowed bool
	}{
		{
			name:          "GET reports allowed",
			path:          "/api/v1/buckets/reports/objects/file",
			method:        "GET",
			expectAllowed: true,
		},
		{
			name:          "PUT reports denied",
			path:          "/api/v1/buckets/reports/objects/file",
			method:        "PUT",
			expectAllowed: false,
		},
		{
			name:          "PUT dev allowed",
			path:          "/api/v1/buckets/dev/objects/file",
			method:        "PUT",
			expectAllowed: true,
		},
		{
			name:          "GET dev allowed",
			path:          "/api/v1/buckets/dev/objects/file",
			method:        "GET",
			expectAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			w := httptest.NewRecorder()

			var handler http.Handler
			switch tt.method {
			case "GET":
				authMw := AuthMiddleware(verifier)
				handler = authMw(http.HandlerFunc(api.HandleGetObject))
			case "PUT":
				authMw := AuthMiddleware(verifier)
				handler = authMw(http.HandlerFunc(api.HandlePutObject))
			}

			handler.ServeHTTP(w, req)

			if tt.expectAllowed {
				assert.NotEqual(t, http.StatusForbidden, w.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, w.Code)
			}
		})
	}
}

// TestGetListBucketsEmptyGroups tests listing buckets with no groups.
func TestGetListBucketsEmptyGroups(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub": "testuser",
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleListBuckets))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var buckets []BucketResponse
	err := json.NewDecoder(w.Body).Decode(&buckets)
	require.NoError(t, err)

	// Should return empty list
	assert.Len(t, buckets, 0, "should return empty bucket list for user with no groups")
}

// TestAuthMeResponseFormat tests that auth/me returns correct format.
func TestAuthMeResponseFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":                "user-123",
			"preferred_username": "testuser",
			"groups":             []interface{}{"group1", "group2"},
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(HandleAuthMe))
	handler.ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["username"])
	assert.NotNil(t, resp["groups"])
}

// TestBucketListResponseFormat tests that bucket list response has correct format.
func TestBucketListResponseFormat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := NewAPI(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"reports-ro", "dev-rw"},
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(api.HandleListBuckets))
	handler.ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var buckets []BucketResponse
	err := json.NewDecoder(w.Body).Decode(&buckets)
	require.NoError(t, err)

	// Check structure
	for _, b := range buckets {
		assert.NotEmpty(t, b.Name)
		assert.IsType(t, PermissionFlags{}, b.Permissions)
	}
}

// TestTokenFromCookie tests auth middleware can extract token from cookie.
func TestTokenFromCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: "token-from-cookie",
	})
	w := httptest.NewRecorder()

	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub": "user-from-cookie",
		},
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(HandleAuthMe))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "should accept token from cookie")
}

// TestAuthHeaderTakesPrecedenceOverCookie tests that Authorization header takes precedence.
func TestAuthHeaderTakesPrecedenceOverCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: "cookie-token",
	})
	w := httptest.NewRecorder()

	// Only return success for header-token
	verifier := &MockTokenVerifier{}
	callCount := 0
	verifier.verifyFunc = func(tokenString string) (jwt.MapClaims, error) {
		callCount++
		if tokenString == "header-token" {
			return jwt.MapClaims{"sub": "user-from-header"}, nil
		}
		return nil, jwt.ErrSignatureInvalid
	}

	authMw := AuthMiddleware(verifier)
	handler := authMw(http.HandlerFunc(HandleAuthMe))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "should use header token over cookie")
}
