package proxy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteHeaders(t *testing.T) {
	original := http.Header{}
	original.Set("Authorization", "Bearer user-jwt")
	original.Set("X-Amz-Date", "20230101T000000Z")
	original.Set("X-Amz-Content-Sha256", "abc")
	original.Set("X-Amz-Security-Token", "tok")
	original.Set("Connection", "keep-alive")
	original.Set("Transfer-Encoding", "chunked")
	original.Set("Content-Type", "application/octet-stream")
	original.Set("X-Custom", "keep-me")

	cleaned := RewriteHeaders(original)

	// SigV4 / auth headers stripped.
	assert.Empty(t, cleaned.Get("Authorization"))
	assert.Empty(t, cleaned.Get("X-Amz-Date"))
	assert.Empty(t, cleaned.Get("X-Amz-Content-Sha256"))
	assert.Empty(t, cleaned.Get("X-Amz-Security-Token"))

	// Hop-by-hop stripped.
	assert.Empty(t, cleaned.Get("Connection"))
	assert.Empty(t, cleaned.Get("Transfer-Encoding"))

	// Forwardable headers preserved.
	assert.Equal(t, "application/octet-stream", cleaned.Get("Content-Type"))
	assert.Equal(t, "keep-me", cleaned.Get("X-Custom"))
}

func TestRewriteHeadersDoesNotMutateOriginal(t *testing.T) {
	original := http.Header{}
	original.Set("Authorization", "Bearer user-jwt")
	original.Add("X-Multi", "a")
	original.Add("X-Multi", "b")

	cleaned := RewriteHeaders(original)

	// Original is unchanged.
	assert.Equal(t, "Bearer user-jwt", original.Get("Authorization"))
	assert.Equal(t, []string{"a", "b"}, original["X-Multi"])

	// Mutating the cleaned copy does not affect the original.
	cleaned["X-Multi"][0] = "mutated"
	assert.Equal(t, "a", original["X-Multi"][0], "original backing array must not be shared")
}

func TestRewriteHeadersEmpty(t *testing.T) {
	cleaned := RewriteHeaders(http.Header{})
	assert.Empty(t, cleaned)
}
