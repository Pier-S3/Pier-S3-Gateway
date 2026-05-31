package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingSigner records the arguments it is called with and can be made to
// fail. It also injects an Authorization header so tests can assert signing
// occurred.
type recordingSigner struct {
	called      int
	lastPayload string
	lastService string
	lastRegion  string
	err         error
}

func (s *recordingSigner) SignHTTP(ctx context.Context, creds aws.Credentials, r *http.Request,
	payloadHash, service, region string, signingTime time.Time, optFns ...func(*v4.SignerOptions)) error {
	s.called++
	s.lastPayload = payloadHash
	s.lastService = service
	s.lastRegion = region
	if s.err != nil {
		return s.err
	}
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test")
	return nil
}

func newTestPassThrough(t *testing.T, backend S3Backend, signer s3Signer) *PassThrough {
	t.Helper()
	return &PassThrough{
		backend: backend,
		signer:  signer,
		logger:  discardLogger(),
		now:     func() time.Time { return time.Unix(0, 0).UTC() },
	}
}

// TestForwardStreamsResponse verifies the happy path: the request is signed,
// the user JWT is stripped, and the upstream body streams back.
func TestForwardStreamsResponse(t *testing.T) {
	var gotAuth string
	var gotMethod string
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "hello-from-upstream")
	}))
	defer upstream.Close()

	backend := &fakeBackend{
		endpoint: upstream.URL,
		region:   "us-east-1",
		creds:    aws.Credentials{AccessKeyID: "AK", SecretAccessKey: "SK"},
	}
	signer := &recordingSigner{}
	pt := newTestPassThrough(t, backend, signer)

	req := httptest.NewRequest("PUT", "/bucket/key", strings.NewReader("payload"))
	req.Header.Set("Authorization", "Bearer user-jwt")
	w := httptest.NewRecorder()

	pt.Forward(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello-from-upstream", w.Body.String())
	assert.Equal(t, "yes", w.Header().Get("X-Upstream"))
	assert.Equal(t, 1, signer.called, "request must be signed once")
	assert.Equal(t, unsignedPayload, signer.lastPayload)
	assert.Equal(t, "s3", signer.lastService)
	assert.Equal(t, "us-east-1", signer.lastRegion)
	assert.Equal(t, "PUT", gotMethod)
	assert.Equal(t, "payload", gotBody)
	assert.NotEqual(t, "Bearer user-jwt", gotAuth, "user JWT must not reach upstream")
	assert.Contains(t, gotAuth, "AWS4-HMAC-SHA256", "upstream must see the re-signed auth")
}

// TestForwardCredentialsError returns 502 when credentials cannot be retrieved.
func TestForwardCredentialsError(t *testing.T) {
	backend := &fakeBackend{
		endpoint: "http://127.0.0.1:1",
		region:   "us-east-1",
		credsErr: errors.New("no creds"),
	}
	pt := newTestPassThrough(t, backend, &recordingSigner{})

	req := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	pt.Forward(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "upstream_error")
}

// TestForwardInvalidEndpoint returns 502 for a misconfigured endpoint.
func TestForwardInvalidEndpoint(t *testing.T) {
	for _, ep := range []string{"", "://bad", "not-a-url"} {
		backend := &fakeBackend{endpoint: ep, region: "us-east-1"}
		pt := newTestPassThrough(t, backend, &recordingSigner{})

		req := httptest.NewRequest("GET", "/bucket/key", nil)
		w := httptest.NewRecorder()
		pt.Forward(w, req)

		assert.Equal(t, http.StatusBadGateway, w.Code, "endpoint %q should yield 502", ep)
	}
}

// TestForwardUpstreamUnreachable returns 502 via ErrorHandler when the backend
// cannot be reached.
func TestForwardUpstreamUnreachable(t *testing.T) {
	backend := &fakeBackend{
		// Reserved TEST-NET-1 address that will refuse/timeout quickly.
		endpoint: "http://127.0.0.1:1",
		region:   "us-east-1",
		creds:    aws.Credentials{AccessKeyID: "AK", SecretAccessKey: "SK"},
	}
	pt := newTestPassThrough(t, backend, &recordingSigner{})

	req := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	pt.Forward(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "upstream_error")
}

// TestSignedRequestPreview verifies rewrite + signing without a round-trip.
func TestSignedRequestPreview(t *testing.T) {
	backend := &fakeBackend{
		endpoint: "http://seaweed:8333",
		region:   "eu-west-1",
		creds:    aws.Credentials{AccessKeyID: "AK", SecretAccessKey: "SK"},
	}
	signer := &recordingSigner{}
	pt := newTestPassThrough(t, backend, signer)

	req := httptest.NewRequest("GET", "/bucket/key", nil)
	req.Header.Set("Authorization", "Bearer user-jwt")
	req.Header.Set("X-Amz-Date", "should-be-stripped")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-Keep-Me", "yes")

	out, err := pt.signedRequestPreview(req)
	require.NoError(t, err)

	assert.Equal(t, "seaweed:8333", out.URL.Host)
	assert.Equal(t, "http", out.URL.Scheme)
	assert.Equal(t, "eu-west-1", signer.lastRegion)
	// SigV4/hop-by-hop headers removed; non-conflicting header preserved.
	assert.Empty(t, out.Header.Get("X-Amz-Date"))
	assert.Empty(t, out.Header.Get("Connection"))
	assert.Equal(t, "yes", out.Header.Get("X-Keep-Me"))
	// Signer set its own Authorization.
	assert.Contains(t, out.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
}

// TestSignedRequestPreviewSignerError surfaces signer failures.
func TestSignedRequestPreviewSignerError(t *testing.T) {
	backend := &fakeBackend{
		endpoint: "http://seaweed:8333",
		region:   "us-east-1",
		creds:    aws.Credentials{AccessKeyID: "AK", SecretAccessKey: "SK"},
	}
	pt := newTestPassThrough(t, backend, &recordingSigner{err: errors.New("sign failed")})

	req := httptest.NewRequest("GET", "/bucket/key", nil)
	_, err := pt.signedRequestPreview(req)
	require.Error(t, err)
}

// TestSignedRequestPreviewBadEndpoint covers the endpoint validation branch.
func TestSignedRequestPreviewBadEndpoint(t *testing.T) {
	backend := &fakeBackend{endpoint: "", region: "us-east-1"}
	pt := newTestPassThrough(t, backend, &recordingSigner{})
	req := httptest.NewRequest("GET", "/b/k", nil)
	_, err := pt.signedRequestPreview(req)
	require.Error(t, err)
}

// TestStreamCopy exercises the streaming helper.
func TestStreamCopy(t *testing.T) {
	src := strings.NewReader("streamed-bytes")
	var dst bytes.Buffer
	n, err := StreamCopy(&dst, src)
	require.NoError(t, err)
	assert.Equal(t, int64(len("streamed-bytes")), n)
	assert.Equal(t, "streamed-bytes", dst.String())
}

// TestNewPassThrough wires a real signer and default clock.
func TestNewPassThrough(t *testing.T) {
	backend := &fakeBackend{endpoint: "http://x:8333", region: "us-east-1"}
	pt := NewPassThrough(backend, discardLogger())
	require.NotNil(t, pt)
	assert.NotNil(t, pt.signer)
	assert.NotNil(t, pt.now)
}

// TestErrString covers the nil and non-nil branches.
func TestErrString(t *testing.T) {
	assert.Equal(t, "endpoint missing scheme or host", errString(nil))
	assert.Equal(t, "boom", errString(errors.New("boom")))
}
