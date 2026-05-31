package proxy

import (
	"net/http"
)

// hopByHopHeaders are headers that should not be forwarded to the upstream.
// Per RFC 7230 section 6.1 these are connection-specific and must be stripped
// when proxying.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// sigV4Headers are headers that, if forwarded from the client, would conflict
// with the signature we compute against the upstream. They must be removed
// before re-signing so the signer produces a canonical request that matches
// the headers actually sent.
var sigV4Headers = []string{
	"Authorization",
	"X-Amz-Date",
	"X-Amz-Content-Sha256",
	"X-Amz-Security-Token",
	"X-Amz-Credential",
	"X-Amz-Signature",
	"X-Amz-Signedheaders",
	"X-Amz-Algorithm",
}

// RewriteHeaders creates a clean copy of request headers suitable for forwarding to S3.
//
// It removes:
//   - The user's Authorization header (their Keycloak JWT) and any other SigV4
//     related headers, because the gateway re-signs the request with the
//     service-account credentials.
//   - Hop-by-hop headers that must not cross a proxy boundary.
//
// The returned header set is a fresh map; the input is never mutated.
func RewriteHeaders(original http.Header) http.Header {
	cleaned := make(http.Header, len(original))
	for key, values := range original {
		// Copy the value slice so callers cannot observe mutation of the
		// original header's backing arrays.
		cp := make([]string, len(values))
		copy(cp, values)
		cleaned[key] = cp
	}

	// Strip SigV4 / user-auth headers - the gateway adds its own signature.
	for _, h := range sigV4Headers {
		cleaned.Del(h)
	}

	// Strip hop-by-hop headers.
	for _, h := range hopByHopHeaders {
		cleaned.Del(h)
	}

	return cleaned
}
