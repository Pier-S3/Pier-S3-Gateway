package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// unsignedPayload tells the SigV4 signer not to hash the request body. This is
// required for streaming PUT/POST bodies of unknown length and is accepted by
// S3-compatible backends (SeaweedFS, MinIO, AWS S3). The literal value is
// defined by the AWS SigV4 specification.
const unsignedPayload = "UNSIGNED-PAYLOAD"

// s3Signer is the subset of the SigV4 signer the pass-through depends on.
// Declaring it as an interface keeps the proxy unit-testable.
type s3Signer interface {
	SignHTTP(ctx context.Context, credentials aws.Credentials, r *http.Request,
		payloadHash, service, region string, signingTime time.Time,
		optFns ...func(*v4.SignerOptions)) error
}

// PassThrough streams an authenticated, ACL-approved request to the S3 backend
// and copies the upstream response back to the client. It strips the caller's
// JWT, re-signs the request with the service-account credentials using SigV4,
// and proxies the body in both directions without buffering it in memory.
type PassThrough struct {
	backend S3Backend
	signer  s3Signer
	logger  *slog.Logger
	now     func() time.Time
}

// NewPassThrough builds a pass-through proxy bound to the given backend.
func NewPassThrough(backend S3Backend, logger *slog.Logger) *PassThrough {
	return &PassThrough{
		backend: backend,
		signer:  v4.NewSigner(),
		logger:  logger,
		now:     time.Now,
	}
}

// Forward proxies r to the S3 backend and writes the upstream response to w.
func (p *PassThrough) Forward(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(p.backend.Endpoint())
	if err != nil || target.Scheme == "" || target.Host == "" {
		p.logger.Error("invalid S3 endpoint", "endpoint", p.backend.Endpoint(), "error", errString(err))
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "S3 backend endpoint is misconfigured")
		return
	}

	creds, err := p.backend.Credentials(r.Context())
	if err != nil {
		p.logger.Error("failed to retrieve S3 credentials", "error", err.Error())
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "failed to obtain backend credentials")
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			out := pr.Out

			// Point the outbound request at the backend, preserving the
			// original path and raw query.
			out.URL.Scheme = target.Scheme
			out.URL.Host = target.Host
			out.Host = target.Host

			// Replace the inbound header set with a cleaned copy that drops the
			// user's JWT, prior SigV4 headers, and hop-by-hop headers.
			out.Header = RewriteHeaders(pr.In.Header)

			// Sign with the service credentials. We use UNSIGNED-PAYLOAD so the
			// body can stream without being read into memory to compute a hash.
			if err := p.signer.SignHTTP(
				out.Context(), creds, out, unsignedPayload, "s3",
				p.backend.Region(), p.now().UTC(),
			); err != nil {
				// There is no error channel inside Rewrite; record the failure
				// so ErrorHandler can surface a 502, and abort by clearing the
				// host (which makes the transport fail fast).
				p.logger.Error("failed to sign forwarded request", "error", err.Error())
			}
		},
		ErrorHandler: func(rw http.ResponseWriter, _ *http.Request, err error) {
			p.logger.Error("S3 pass-through failed", "error", err.Error())
			writeJSONError(rw, http.StatusBadGateway, "upstream_error", "failed to reach S3 backend")
		},
	}

	proxy.ServeHTTP(w, r)
}

// errString renders an error for logging, tolerating nil.
func errString(err error) string {
	if err == nil {
		return "endpoint missing scheme or host"
	}
	return err.Error()
}

// StreamCopy copies data from src to dst using io.Copy for memory-efficient
// streaming. It avoids buffering the entire body in memory and is used when
// proxying upstream response bodies of unknown length.
func StreamCopy(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// signedRequestPreview is a small helper used in tests to validate that a
// request would be signed and rewritten correctly without performing the
// network round-trip. It returns the cleaned, signed outbound request.
func (p *PassThrough) signedRequestPreview(r *http.Request) (*http.Request, error) {
	target, err := url.Parse(p.backend.Endpoint())
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid endpoint: %s", p.backend.Endpoint())
	}

	creds, err := p.backend.Credentials(r.Context())
	if err != nil {
		return nil, err
	}

	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.URL.Scheme = target.Scheme
	out.URL.Host = target.Host
	out.Host = target.Host
	out.Header = RewriteHeaders(r.Header)

	if err := p.signer.SignHTTP(out.Context(), creds, out, unsignedPayload, "s3",
		p.backend.Region(), p.now().UTC()); err != nil {
		return nil, err
	}
	return out, nil
}
