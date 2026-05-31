package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pier-s3/internal/auth"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend is an in-memory s3Backend implementation for handler tests.
type mockBackend struct {
	listBucketsFn func(ctx context.Context) (*s3.ListBucketsOutput, error)
	listFn        func(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error)
	getFn         func(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error)
	headFn        func(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error)
	putFn         func(ctx context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error)
	deleteFn      func(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error)

	// captured arguments for assertions
	lastBucket, lastKey, lastPrefix, lastToken string
	lastMaxKeys                                int32
	lastPutBody                                string
}

func (m *mockBackend) ListBuckets(ctx context.Context) (*s3.ListBucketsOutput, error) {
	if m.listBucketsFn == nil {
		return &s3.ListBucketsOutput{}, nil
	}
	return m.listBucketsFn(ctx)
}

func (m *mockBackend) ListObjects(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
	m.lastBucket, m.lastPrefix, m.lastToken, m.lastMaxKeys = bucket, prefix, token, maxKeys
	return m.listFn(ctx, bucket, prefix, delimiter, token, maxKeys)
}

func (m *mockBackend) GetObject(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
	m.lastBucket, m.lastKey = bucket, key
	return m.getFn(ctx, bucket, key)
}

func (m *mockBackend) HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	m.lastBucket, m.lastKey = bucket, key
	return m.headFn(ctx, bucket, key)
}

func (m *mockBackend) PutObject(ctx context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	m.lastBucket = aws.ToString(in.Bucket)
	m.lastKey = aws.ToString(in.Key)
	if in.Body != nil {
		b, _ := io.ReadAll(in.Body)
		m.lastPutBody = string(b)
	}
	return m.putFn(ctx, in)
}

func (m *mockBackend) DeleteObject(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error) {
	m.lastBucket, m.lastKey = bucket, key
	return m.deleteFn(ctx, bucket, key)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// withGroups injects groups into a request context, simulating AuthMiddleware,
// so handlers can be tested without a verifier.
func withGroups(r *http.Request, groups []string) *http.Request {
	ctx := context.WithValue(r.Context(), groupsKey, groups)
	return r.WithContext(ctx)
}

func errReader() io.ReadCloser { return io.NopCloser(&failingReader{}) }

type failingReader struct{}

func (f *failingReader) Read(p []byte) (int, error) { return 0, errors.New("boom") }

// --- HandleListObjects success / pagination ---

func TestHandleListObjectsSuccess(t *testing.T) {
	backend := &mockBackend{
		listFn: func(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []types.Object{
					{Key: aws.String("a.txt"), Size: aws.Int64(10), ETag: aws.String("\"abc\""), LastModified: aws.Time(time.Unix(1000, 0))},
					{Key: aws.String("b.txt"), Size: aws.Int64(20), ETag: aws.String("def")},
				},
				CommonPrefixes:        []types.CommonPrefix{{Prefix: aws.String("sub/")}},
				IsTruncated:           aws.Bool(true),
				NextContinuationToken: aws.String("next-token"),
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects?prefix=p/&max_keys=50&page_token=tok", nil)
	req.SetPathValue("bucket", "reports")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()

	api.HandleListObjects(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp ListObjectsResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Len(t, resp.Objects, 2)
	assert.Equal(t, "a.txt", resp.Objects[0].Key)
	assert.Equal(t, int64(10), resp.Objects[0].SizeBytes)
	assert.Equal(t, "abc", resp.Objects[0].ETag, "ETag quotes should be trimmed")
	assert.Equal(t, []string{"sub/"}, resp.Prefixes)
	assert.True(t, resp.Truncated)
	assert.Equal(t, "next-token", resp.NextPageToken)

	// query params propagated to backend
	assert.Equal(t, "p/", backend.lastPrefix)
	assert.Equal(t, "tok", backend.lastToken)
	assert.Equal(t, int32(50), backend.lastMaxKeys)
}

func TestHandleListObjectsEmpty(t *testing.T) {
	backend := &mockBackend{
		listFn: func(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects", nil)
	req.SetPathValue("bucket", "reports")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()

	api.HandleListObjects(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	// Empty slices must serialize as [] not null.
	body := w.Body.String()
	assert.Contains(t, body, `"objects":[]`)
	assert.Contains(t, body, `"prefixes":[]`)
}

func TestHandleListObjectsInvalidMaxKeysDefaults(t *testing.T) {
	backend := &mockBackend{
		listFn: func(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	for _, mk := range []string{"0", "-5", "9999", "abc"} {
		req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects?max_keys="+mk, nil)
		req.SetPathValue("bucket", "reports")
		req = withGroups(req, []string{"reports-ro"})
		w := httptest.NewRecorder()
		api.HandleListObjects(w, req)
		require.Equal(t, http.StatusOK, w.Code, "max_keys=%s", mk)
		assert.Equal(t, int32(1000), backend.lastMaxKeys, "invalid max_keys=%s should default to 1000", mk)
	}
}

func TestHandleListObjectsUpstreamError(t *testing.T) {
	backend := &mockBackend{
		listFn: func(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("upstream down")
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects", nil)
	req.SetPathValue("bucket", "reports")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()

	api.HandleListObjects(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandleListObjectsForbidden(t *testing.T) {
	api := newAPIWithBackend(&mockBackend{}, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/secret/objects", nil)
	req.SetPathValue("bucket", "secret")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleListObjects(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleListObjectsMissingBucket(t *testing.T) {
	api := newAPIWithBackend(&mockBackend{}, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets//objects", nil)
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleListObjects(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleListObjectsBackendUnavailable(t *testing.T) {
	api := NewAPI(nil, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects", nil)
	req.SetPathValue("bucket", "reports")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleListObjects(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- HandleGetObject ---

func TestHandleGetObjectSuccess(t *testing.T) {
	backend := &mockBackend{
		getFn: func(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{
				Body:          io.NopCloser(strings.NewReader("hello world")),
				ContentType:   aws.String("text/plain"),
				ContentLength: aws.Int64(11),
				ETag:          aws.String("\"etag123\""),
				LastModified:  aws.Time(time.Unix(1700000000, 0)),
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects/dir/file.txt", nil)
	req.SetPathValue("bucket", "reports")
	req.SetPathValue("key", "dir/file.txt")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()

	api.HandleGetObject(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello world", w.Body.String())
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Equal(t, "11", w.Header().Get("Content-Length"))
	assert.Equal(t, "\"etag123\"", w.Header().Get("ETag"))
	assert.NotEmpty(t, w.Header().Get("Last-Modified"))
	assert.Equal(t, "dir/file.txt", backend.lastKey)
}

func TestHandleGetObjectUpstreamError(t *testing.T) {
	backend := &mockBackend{
		getFn: func(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
			return nil, errors.New("no such key")
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects/x", nil)
	req.SetPathValue("bucket", "reports")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleGetObject(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandleGetObjectBodyCopyError(t *testing.T) {
	backend := &mockBackend{
		getFn: func(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: errReader()}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects/x", nil)
	req.SetPathValue("bucket", "reports")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	// Should not panic even though the body read fails after headers committed.
	api.HandleGetObject(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleGetObjectMissingKey(t *testing.T) {
	api := newAPIWithBackend(&mockBackend{}, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects/", nil)
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleGetObject(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- HandleGetObjectMeta ---

func TestHandleGetObjectMetaSuccess(t *testing.T) {
	backend := &mockBackend{
		headFn: func(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
			return &s3.HeadObjectOutput{
				ContentType:   aws.String("application/json"),
				ContentLength: aws.Int64(42),
				ETag:          aws.String("\"meta-etag\""),
				LastModified:  aws.Time(time.Unix(1700000000, 0)),
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/meta/dir/file.json", nil)
	req.SetPathValue("bucket", "reports")
	req.SetPathValue("key", "dir/file.json")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()

	api.HandleGetObjectMeta(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var meta map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&meta))
	assert.Equal(t, "dir/file.json", meta["key"])
	assert.Equal(t, "application/json", meta["content_type"])
	assert.Equal(t, float64(42), meta["content_length"])
	assert.Equal(t, "meta-etag", meta["etag"], "ETag quotes trimmed")
}

func TestHandleGetObjectMetaUpstreamError(t *testing.T) {
	backend := &mockBackend{
		headFn: func(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
			return nil, errors.New("404")
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/meta/x", nil)
	req.SetPathValue("bucket", "reports")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleGetObjectMeta(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandleGetObjectMetaForbidden(t *testing.T) {
	api := newAPIWithBackend(&mockBackend{}, testLogger())
	req := httptest.NewRequest("GET", "/api/v1/buckets/secret/meta/x", nil)
	req.SetPathValue("bucket", "secret")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()
	api.HandleGetObjectMeta(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- HandlePutObject ---

func TestHandlePutObjectSuccess(t *testing.T) {
	backend := &mockBackend{
		putFn: func(ctx context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
			return &s3.PutObjectOutput{}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("PUT", "/api/v1/buckets/dev/objects/up/file.bin", strings.NewReader("payload-bytes"))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.SetPathValue("bucket", "dev")
	req.SetPathValue("key", "up/file.bin")
	req = withGroups(req, []string{"dev-rw"})
	w := httptest.NewRecorder()

	api.HandlePutObject(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "uploaded", resp["status"])
	assert.Equal(t, "up/file.bin", resp["key"])
	assert.Equal(t, "payload-bytes", backend.lastPutBody, "body should be streamed to backend")
	assert.Equal(t, "dev", backend.lastBucket)
}

func TestHandlePutObjectUpstreamError(t *testing.T) {
	backend := &mockBackend{
		putFn: func(ctx context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
			return nil, errors.New("write failed")
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("PUT", "/api/v1/buckets/dev/objects/x", strings.NewReader("d"))
	req.SetPathValue("bucket", "dev")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"dev-rw"})
	w := httptest.NewRecorder()
	api.HandlePutObject(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestHandlePutObjectDefaultContentType(t *testing.T) {
	var gotCT string
	backend := &mockBackend{
		putFn: func(ctx context.Context, in *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
			gotCT = aws.ToString(in.ContentType)
			return &s3.PutObjectOutput{}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("PUT", "/api/v1/buckets/dev/objects/x", strings.NewReader("d"))
	req.SetPathValue("bucket", "dev")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"dev-rw"})
	w := httptest.NewRecorder()
	api.HandlePutObject(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", gotCT)
}

// --- HandleDeleteObject ---

func TestHandleDeleteObjectSuccess(t *testing.T) {
	backend := &mockBackend{
		deleteFn: func(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error) {
			return &s3.DeleteObjectOutput{}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("DELETE", "/api/v1/buckets/dev/objects/gone.txt", nil)
	req.SetPathValue("bucket", "dev")
	req.SetPathValue("key", "gone.txt")
	req = withGroups(req, []string{"dev-rw"})
	w := httptest.NewRecorder()
	api.HandleDeleteObject(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "deleted", resp["status"])
	assert.Equal(t, "gone.txt", resp["key"])
}

func TestHandleDeleteObjectUpstreamError(t *testing.T) {
	backend := &mockBackend{
		deleteFn: func(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error) {
			return nil, errors.New("delete failed")
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	req := httptest.NewRequest("DELETE", "/api/v1/buckets/dev/objects/x", nil)
	req.SetPathValue("bucket", "dev")
	req.SetPathValue("key", "x")
	req = withGroups(req, []string{"dev-rw"})
	w := httptest.NewRecorder()
	api.HandleDeleteObject(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// --- RegisterRoutes integration: must not panic and must route correctly ---

func TestRegisterRoutesRoutingAndAuth(t *testing.T) {
	backend := &mockBackend{
		listFn: func(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		getFn: func(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("body=" + key))}, nil
		},
		headFn: func(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
			return &s3.HeadObjectOutput{ContentType: aws.String("text/plain")}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	verifier := &MockTokenVerifier{claims: jwt.MapClaims{"sub": "u", "groups": []interface{}{"reports-rw"}}}
	oidcCfg := auth.NewOIDCConfig("https://kc.example", "click", "cli", "", "https://app/cb")

	mux := http.NewServeMux()
	// Must not panic - the old {key...}/meta pattern was invalid.
	require.NotPanics(t, func() { api.RegisterRoutes(mux, verifier, oidcCfg, auth.ClaimMapper{}) })

	// A key literally containing "meta" routes to GetObject, not meta handler.
	req := httptest.NewRequest("GET", "/api/v1/buckets/reports/objects/some/meta", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "some/meta", backend.lastKey, "key ending in meta must reach GetObject intact")
	assert.Equal(t, "body=some/meta", w.Body.String())

	// The /meta/ prefix reaches the metadata handler.
	req = httptest.NewRequest("GET", "/api/v1/buckets/reports/meta/dir/obj", nil)
	req.Header.Set("Authorization", "Bearer t")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "dir/obj", backend.lastKey)

	// Unauthenticated request to a protected route is rejected.
	req = httptest.NewRequest("GET", "/api/v1/buckets", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Path-parsing helpers ---

func TestExtractPathParam(t *testing.T) {
	tests := []struct {
		path, segment, want string
	}{
		{"/api/v1/buckets/my-bucket/objects", "buckets", "my-bucket"},
		{"/api/v1/buckets/my-bucket", "buckets", "my-bucket"},
		{"/api/v1/buckets/", "buckets", ""},
		{"/api/v1/other", "buckets", ""},
		{"", "buckets", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, extractPathParam(tt.path, tt.segment), "path=%q", tt.path)
	}
}

func TestExtractBucketAndKey(t *testing.T) {
	tests := []struct {
		path, wantB, wantK string
	}{
		{"/api/v1/buckets/b/objects/k", "b", "k"},
		{"/api/v1/buckets/b/objects/a/b/c.txt", "b", "a/b/c.txt"},
		{"/api/v1/buckets/b/objects/", "b", ""},
		{"/api/v1/buckets/b/nope/k", "", ""},
		{"/wrong/prefix/objects/k", "", ""},
	}
	for _, tt := range tests {
		b, k := extractBucketAndKey(tt.path)
		assert.Equal(t, tt.wantB, b, "path=%q bucket", tt.path)
		assert.Equal(t, tt.wantK, k, "path=%q key", tt.path)
	}
}

func TestExtractBucketAndKeyMeta(t *testing.T) {
	tests := []struct {
		path, wantB, wantK string
	}{
		{"/api/v1/buckets/b/meta/k", "b", "k"},
		{"/api/v1/buckets/b/meta/a/b/c.txt", "b", "a/b/c.txt"},
		{"/api/v1/buckets/b/meta/", "b", ""},
		{"/api/v1/buckets/b/objects/k", "", ""},
		{"/wrong/b/meta/k", "", ""},
	}
	for _, tt := range tests {
		b, k := extractBucketAndKeyMeta(tt.path)
		assert.Equal(t, tt.wantB, b, "path=%q bucket", tt.path)
		assert.Equal(t, tt.wantK, k, "path=%q key", tt.path)
	}
}

func TestBucketFromRequestFallback(t *testing.T) {
	// No PathValue set -> falls back to URL parsing.
	req := httptest.NewRequest("GET", "/api/v1/buckets/fallback/objects", nil)
	assert.Equal(t, "fallback", bucketFromRequest(req))

	// PathValue takes precedence.
	req2 := httptest.NewRequest("GET", "/whatever", nil)
	req2.SetPathValue("bucket", "from-pathvalue")
	assert.Equal(t, "from-pathvalue", bucketFromRequest(req2))
}

func TestBucketAndKeyMetaFromRequestFallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/buckets/b/meta/k/k2", nil)
	b, k := bucketAndKeyMetaFromRequest(req)
	assert.Equal(t, "b", b)
	assert.Equal(t, "k/k2", k)
}

// --- deref helpers ---

func TestDerefHelpers(t *testing.T) {
	assert.Equal(t, "", deref(nil))
	s := "x"
	assert.Equal(t, "x", deref(&s))

	assert.Equal(t, int64(0), derefInt64(nil))
	n := int64(7)
	assert.Equal(t, int64(7), derefInt64(&n))

	assert.False(t, derefBool(nil))
	b := true
	assert.True(t, derefBool(&b))

	assert.True(t, derefTime(nil).IsZero())
	now := time.Now()
	assert.Equal(t, now, derefTime(&now))
}

// --- GetClaims ---

func TestGetClaims(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	assert.Nil(t, GetClaims(req), "no claims in context returns nil")

	claims := jwt.MapClaims{"sub": "u"}
	ctx := context.WithValue(req.Context(), claimsKey, claims)
	req = req.WithContext(ctx)
	assert.Equal(t, claims, GetClaims(req))
}

func TestGetUsernameGetGroupsEmpty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	assert.Equal(t, "", GetUsername(req))
	assert.Nil(t, GetGroups(req))
}

// TestHandleListBucketsWildcard verifies that a wildcard ("*-ro") grant expands
// to every bucket the backend reports, unioned with named-group permissions.
func TestHandleListBucketsWildcard(t *testing.T) {
	backend := &mockBackend{
		listBucketsFn: func(ctx context.Context) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []types.Bucket{
					{Name: aws.String("alpha")},
					{Name: aws.String("reports")},
					{Name: aws.String("zeta")},
				},
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	// Wildcard read everywhere + full read/write/delete on "reports".
	req = withGroups(req, []string{"*-ro", "reports-rw"})
	w := httptest.NewRecorder()

	api.HandleListBuckets(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []BucketResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	// All three backend buckets are present, sorted by name.
	require.Len(t, resp, 3)
	assert.Equal(t, []string{"alpha", "reports", "zeta"},
		[]string{resp[0].Name, resp[1].Name, resp[2].Name})

	byName := map[string]PermissionFlags{}
	for _, b := range resp {
		byName[b.Name] = b.Permissions
	}
	// Wildcard-only buckets: read, no write/delete.
	assert.Equal(t, PermissionFlags{Read: true}, byName["alpha"])
	assert.Equal(t, PermissionFlags{Read: true}, byName["zeta"])
	// Named rw unions over the wildcard read.
	assert.Equal(t, PermissionFlags{Read: true, Write: true, Delete: true}, byName["reports"])
}

// TestHandleListBucketsNoWildcardSkipsBackend verifies that without a wildcard
// grant the handler does NOT call the backend (named groups only).
func TestHandleListBucketsNoWildcardSkipsBackend(t *testing.T) {
	called := false
	backend := &mockBackend{
		listBucketsFn: func(ctx context.Context) (*s3.ListBucketsOutput, error) {
			called = true
			return &s3.ListBucketsOutput{Buckets: []types.Bucket{{Name: aws.String("secret")}}}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	req = withGroups(req, []string{"reports-ro"})
	w := httptest.NewRecorder()

	api.HandleListBuckets(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.False(t, called, "backend ListBuckets must not be called without a wildcard grant")
	var resp []BucketResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "reports", resp[0].Name)
}
