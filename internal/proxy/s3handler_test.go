package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTokenVerifier is a mock implementation of TokenVerifier for testing.
type MockTokenVerifier struct {
	claims jwt.MapClaims
	err    error
}

// VerifyToken implements TokenVerifier.VerifyToken.
func (m *MockTokenVerifier) VerifyToken(tokenString string) (jwt.MapClaims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

// fakeForwarder records whether Forward was invoked and writes a canned
// response, standing in for the real SigV4 pass-through.
type fakeForwarder struct {
	called      bool
	gotRequest  *http.Request
	status      int
	body        string
	respHeaders map[string]string
}

func (f *fakeForwarder) Forward(w http.ResponseWriter, r *http.Request) {
	f.called = true
	f.gotRequest = r
	for k, v := range f.respHeaders {
		w.Header().Set(k, v)
	}
	if f.status == 0 {
		f.status = http.StatusOK
	}
	w.WriteHeader(f.status)
	if f.body != "" {
		_, _ = io.WriteString(w, f.body)
	}
}

// fakeBackend is an in-memory S3Backend for testing handleListBuckets and the
// pass-through signing preview without a live endpoint.
type fakeBackend struct {
	buckets     []string
	listErr     error
	endpoint    string
	region      string
	creds       aws.Credentials
	credsErr    error
	listCalls   int
	nilBucketAt int // index in buckets to emit a nil *Name for (-1 = none)
}

func (b *fakeBackend) ListBuckets(ctx context.Context) (*s3.ListBucketsOutput, error) {
	b.listCalls++
	if b.listErr != nil {
		return nil, b.listErr
	}
	out := &s3.ListBucketsOutput{}
	for i, name := range b.buckets {
		if i == b.nilBucketAt {
			out.Buckets = append(out.Buckets, s3types.Bucket{Name: nil})
			continue
		}
		n := name
		out.Buckets = append(out.Buckets, s3types.Bucket{Name: &n})
	}
	return out, nil
}

func (b *fakeBackend) Endpoint() string { return b.endpoint }
func (b *fakeBackend) Region() string   { return b.region }
func (b *fakeBackend) Credentials(ctx context.Context) (aws.Credentials, error) {
	if b.credsErr != nil {
		return aws.Credentials{}, b.credsErr
	}
	return b.creds, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// createTestHandler builds a handler with a fake backend and forwarder so that
// the full gating + pass-through path can be exercised.
func createTestHandler(verifier *MockTokenVerifier) (*S3Handler, *fakeForwarder, *fakeBackend) {
	fwd := &fakeForwarder{status: http.StatusOK, body: "ok"}
	backend := &fakeBackend{nilBucketAt: -1}
	h := newS3HandlerWithDeps(verifier, backend, fwd, discardLogger())
	return h, fwd, backend
}

// Helper function to make a request with Bearer token.
func makeRequestWithToken(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// TestMissingAuthHeader tests that missing auth header returns 401.
func TestMissingAuthHeader(t *testing.T) {
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "testuser"}}
	handler, _, _ := createTestHandler(verifier)

	req := httptest.NewRequest("GET", "/bucket/object", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401")

	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "unauthorized", errResp["error"])
	assert.Equal(t, "missing or invalid authorization header", errResp["message"])
}

// TestInvalidAuthHeaderFormat tests that invalid auth header format returns 401.
func TestInvalidAuthHeaderFormat(t *testing.T) {
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "testuser"}}
	handler, _, _ := createTestHandler(verifier)

	req := httptest.NewRequest("GET", "/bucket/object", nil)
	req.Header.Set("Authorization", "Basic invalid")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401")
}

// TestInvalidToken tests that invalid token returns 401.
func TestInvalidToken(t *testing.T) {
	verifier := &MockTokenVerifier{err: jwt.ErrSignatureInvalid}
	handler, _, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/bucket/object", "invalid.token.here")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401")

	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "invalid_token", errResp["error"])
	assert.Equal(t, "token verification failed", errResp["message"])
}

// TestAdminOperationBlocked tests that admin operations are blocked before
// reaching the forwarder.
func TestAdminOperationBlocked(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"admin-rw"}},
	}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"PUT on bucket", "PUT", "/bucket"},
		{"DELETE on bucket", "DELETE", "/bucket"},
		{"PUT with policy query", "PUT", "/bucket?policy="},
		{"GET with acl query", "GET", "/bucket?acl="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, fwd, _ := createTestHandler(verifier)
			req := makeRequestWithToken(tt.method, tt.path, "valid-token")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 for admin operation")
			assert.False(t, fwd.called, "forwarder must not be called for admin ops")

			var errResp map[string]string
			require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
			assert.Equal(t, "operation_not_permitted", errResp["error"])
		})
	}
}

// TestAccessDeniedWrongBucket tests that access to wrong bucket is denied.
func TestAccessDeniedWrongBucket(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/other-bucket/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 for access denied")
	assert.False(t, fwd.called)

	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "forbidden", errResp["error"])
	assert.Equal(t, "insufficient permissions", errResp["message"])
}

// TestAccessDeniedWriteOnReadOnly tests that write ops are denied on a ro bucket.
func TestAccessDeniedWriteOnReadOnly(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("PUT", "/reports/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 for write on read-only")
	assert.False(t, fwd.called)

	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "forbidden", errResp["error"])
}

// TestValidReadRequest tests that a valid read request reaches the forwarder
// and the upstream response is streamed back.
func TestValidReadRequest(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler, fwd, _ := createTestHandler(verifier)
	fwd.status = http.StatusOK
	fwd.body = "object-bytes"
	fwd.respHeaders = map[string]string{"Content-Type": "application/octet-stream"}

	req := makeRequestWithToken("GET", "/reports/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, fwd.called, "valid read should be forwarded")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "object-bytes", w.Body.String())
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
}

// TestValidWriteRequest tests that a valid write request reaches the forwarder.
func TestValidWriteRequest(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-rw"}},
	}
	handler, fwd, _ := createTestHandler(verifier)
	fwd.status = http.StatusOK

	req := makeRequestWithToken("PUT", "/reports/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, fwd.called, "valid write should be forwarded")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestValidDeleteRequest tests that a valid delete request reaches the forwarder.
func TestValidDeleteRequest(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"data-rw"}},
	}
	handler, fwd, _ := createTestHandler(verifier)
	fwd.status = http.StatusNoContent

	req := makeRequestWithToken("DELETE", "/data/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, fwd.called, "valid delete should be forwarded")
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestUpstreamErrorMapped verifies that an upstream error status produced by
// the forwarder is passed through unchanged to the client.
func TestUpstreamErrorMapped(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler, fwd, _ := createTestHandler(verifier)
	fwd.status = http.StatusNotFound
	fwd.body = "<Error><Code>NoSuchKey</Code></Error>"

	req := makeRequestWithToken("GET", "/reports/missing", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, fwd.called)
	assert.Equal(t, http.StatusNotFound, w.Code, "upstream status should pass through")
	assert.Contains(t, w.Body.String(), "NoSuchKey")
}

// TestForwarderNotConfigured verifies the handler returns 502 (not panic) when
// no backend/forwarder is wired but auth/ACL pass.
func TestForwarderNotConfigured(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler := NewS3Handler(verifier, nil, discardLogger())

	req := makeRequestWithToken("GET", "/reports/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)

	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "upstream_error", errResp["error"])
}

// TestMultipleGroupsAccess tests that a user with multiple groups gets the
// expected gating per bucket/method.
func TestMultipleGroupsAccess(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro", "dev-rw"}},
	}

	tests := []struct {
		name          string
		method        string
		path          string
		expectAllowed bool
	}{
		{"GET reports allowed", "GET", "/reports/file", true},
		{"PUT reports denied", "PUT", "/reports/file", false},
		{"GET dev allowed", "GET", "/dev/file", true},
		{"PUT dev allowed", "PUT", "/dev/file", true},
		{"other bucket denied", "GET", "/other/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, fwd, _ := createTestHandler(verifier)
			req := makeRequestWithToken(tt.method, tt.path, "valid-token")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectAllowed {
				assert.True(t, fwd.called, "should pass gating and be forwarded")
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.False(t, fwd.called, "should be denied before forwarding")
				assert.Equal(t, http.StatusForbidden, w.Code)
			}
		})
	}
}

// TestNoGroups tests that a user without groups has no access.
func TestNoGroups(t *testing.T) {
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "testuser"}}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/any-bucket/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 with no groups")
	assert.False(t, fwd.called)
}

// TestEmptyGroups tests that a user with an empty groups array has no access.
func TestEmptyGroups(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{}},
	}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/any-bucket/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should return 403 with empty groups")
	assert.False(t, fwd.called)
}

// TestRootPathListBuckets verifies GET / lists buckets filtered by ACL.
func TestRootPathListBuckets(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro", "dev-rw"}},
	}
	handler, _, backend := createTestHandler(verifier)
	backend.buckets = []string{"reports", "dev", "secret"}

	req := makeRequestWithToken("GET", "/", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, backend.listCalls, "ListBuckets should be called once")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got []struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))

	names := make(map[string]bool)
	for _, b := range got {
		names[b.Name] = true
	}
	assert.True(t, names["reports"], "reports should be visible")
	assert.True(t, names["dev"], "dev should be visible")
	assert.False(t, names["secret"], "secret must be filtered out by ACL")
}

// TestRootPathListBucketsNilName ensures a nil bucket name does not panic and
// is skipped.
func TestRootPathListBucketsNilName(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler, _, backend := createTestHandler(verifier)
	backend.buckets = []string{"reports", "ignored"}
	backend.nilBucketAt = 1 // emit nil *Name at index 1

	req := makeRequestWithToken("GET", "/", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Len(t, got, 1)
	assert.Equal(t, "reports", got[0].Name)
}

// TestRootPathListBucketsUpstreamError maps a backend error to 502.
func TestRootPathListBucketsUpstreamError(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler, _, backend := createTestHandler(verifier)
	backend.listErr = errors.New("connection refused")

	req := makeRequestWithToken("GET", "/", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "upstream_error", errResp["error"])
}

// TestRootPathListBucketsNoBackend returns 502 when backend is nil.
func TestRootPathListBucketsNoBackend(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-ro"}},
	}
	handler := NewS3Handler(verifier, nil, discardLogger())

	req := makeRequestWithToken("GET", "/", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestRootPathNonGet returns 400 for non-GET requests at root.
func TestRootPathNonGet(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-rw"}},
	}
	handler, _, _ := createTestHandler(verifier)

	req := makeRequestWithToken("DELETE", "/", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
	assert.Equal(t, "bad_request", errResp["error"])
}

// TestInvalidGroups tests that invalid group names are ignored but a valid one
// still grants access.
func TestInvalidGroups(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub":    "testuser",
			"groups": []interface{}{"invalid-group", "no-policy", "reports-ro"},
		},
	}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/reports/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, fwd.called, "should forward with a valid group in the list")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBucketPathExtraction tests that bucket path extraction gates correctly.
func TestBucketPathExtraction(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"my-bucket-rw"}},
	}

	tests := []struct {
		name          string
		path          string
		expectForward bool
	}{
		// GET on a bucket-only path is a list-objects request; it is not an
		// admin op and the rw group grants read, so it is forwarded.
		{"bucket only", "/my-bucket", true},
		{"bucket and object", "/my-bucket/object", true},
		{"bucket and nested path", "/my-bucket/folder/subfolder/object", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, fwd, _ := createTestHandler(verifier)
			req := makeRequestWithToken("GET", tt.path, "valid-token")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectForward, fwd.called)
		})
	}
}

// TestNilVerifierClaims tests handling of nil claims from the verifier.
func TestNilVerifierClaims(t *testing.T) {
	verifier := &MockTokenVerifier{claims: nil}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/bucket/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "should deny access with nil claims")
	assert.False(t, fwd.called)
}

// TestResponseFormat tests that error responses are proper JSON.
func TestResponseFormat(t *testing.T) {
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "testuser"}}
	handler, _, _ := createTestHandler(verifier)

	// PUT on root => 400 bad_request (no bucket, non-GET)
	req := makeRequestWithToken("PUT", "/", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp), "response should be valid JSON")
	assert.NotEmpty(t, errResp["error"])
	assert.NotEmpty(t, errResp["message"])
}

// TestCaseInsensitiveBearer tests that the Bearer scheme is case-insensitive.
func TestCaseInsensitiveBearer(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{"sub": "testuser", "groups": []interface{}{"reports-rw"}},
	}

	for _, bearer := range []string{"Bearer", "bearer", "BEARER", "BeArEr"} {
		t.Run("Bearer case: "+bearer, func(t *testing.T) {
			handler, _, _ := createTestHandler(verifier)
			req := httptest.NewRequest("GET", "/reports/object", nil)
			req.Header.Set("Authorization", bearer+" valid-token")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusUnauthorized, w.Code, "Bearer should be case-insensitive")
		})
	}
}

// TestRealmAccessRoles tests groups parsed from the realm_access.roles claim.
func TestRealmAccessRoles(t *testing.T) {
	verifier := &MockTokenVerifier{
		claims: jwt.MapClaims{
			"sub": "testuser",
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"/reports-ro"},
			},
		},
	}
	handler, fwd, _ := createTestHandler(verifier)

	req := makeRequestWithToken("GET", "/reports/object", "valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, fwd.called, "should forward via realm_access.roles group")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestNewS3HandlerWiring verifies the real-client branch wires a backend and
// forwarder (no nil-interface trap), while a nil client leaves them unset.
func TestNewS3HandlerWiring(t *testing.T) {
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "u"}}

	client, err := NewS3Client("http://x:8333", "ak", "sk", "us-east-1")
	require.NoError(t, err)

	withClient := NewS3Handler(verifier, client, discardLogger())
	assert.NotNil(t, withClient.backend, "backend should be wired from real client")
	assert.NotNil(t, withClient.forwarder, "forwarder should be wired from real client")

	withoutClient := NewS3Handler(verifier, nil, discardLogger())
	assert.Nil(t, withoutClient.backend, "nil client must leave backend nil (no typed-nil trap)")
	assert.Nil(t, withoutClient.forwarder, "nil client must leave forwarder nil")
}

// TestPathBucket covers path parsing directly.
func TestPathBucket(t *testing.T) {
	tests := []struct {
		path       string
		wantBucket string
	}{
		{"/", ""},
		{"", ""},
		{"/bucket", "bucket"},
		{"/bucket/key", "bucket"},
		{"/bucket/a/b/c", "bucket"},
		{"bucket/key", "bucket"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.wantBucket, pathBucket(tt.path))
		})
	}
}

// TestExtractBearerToken covers token extraction edge cases.
func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", ""},
		{"bearer with token", "Bearer abc", "abc"},
		{"lowercase bearer", "bearer abc", "abc"},
		{"basic scheme", "Basic abc", ""},
		{"no scheme", "abc", ""},
		{"trailing spaces", "Bearer   abc  ", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			assert.Equal(t, tt.want, extractBearerToken(req))
		})
	}
}
