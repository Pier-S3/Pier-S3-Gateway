package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRateLimiterAllowBurstThenRefill verifies that a key may burst up to the
// configured capacity, is then throttled, and recovers after enough time
// elapses for the bucket to refill.
func TestRateLimiterAllowBurstThenRefill(t *testing.T) {
	// 60 req/min = 1 token/sec, burst of 3.
	rl := newRateLimiter(60, 3)
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	// Burst: first 3 succeed, 4th is throttled.
	assert.True(t, rl.allow("ip-a"))
	assert.True(t, rl.allow("ip-a"))
	assert.True(t, rl.allow("ip-a"))
	assert.False(t, rl.allow("ip-a"), "4th request should exceed burst")

	// After 1 second one token refills.
	now = now.Add(time.Second)
	assert.True(t, rl.allow("ip-a"), "one token should have refilled")
	assert.False(t, rl.allow("ip-a"), "only one token refilled")
}

// TestRateLimiterPerKeyIsolation verifies that distinct keys have independent
// buckets, so one noisy client cannot starve another.
func TestRateLimiterPerKeyIsolation(t *testing.T) {
	rl := newRateLimiter(60, 1)
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	assert.True(t, rl.allow("ip-a"))
	assert.False(t, rl.allow("ip-a"))
	// Different key still has a full bucket.
	assert.True(t, rl.allow("ip-b"))
}

// TestRateLimitMiddleware429 verifies the middleware returns 429 once the
// limiter is exhausted for a client IP.
func TestRateLimitMiddleware429(t *testing.T) {
	rl := newRateLimiter(60, 1)
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	called := 0
	h := rateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	do := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.7:5555"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	assert.Equal(t, http.StatusOK, do().Code)
	w := do()
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "second request should be throttled")
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
	assert.Equal(t, 1, called, "handler should run only for the allowed request")
}

// TestClientIPForwardedFor verifies the client IP is taken from the RIGHT-MOST
// X-Forwarded-For hop (the value appended by the trusted ingress, which a remote
// client cannot forge), else from RemoteAddr. A spoofed left-most entry must be
// ignored so it cannot be used to rotate the rate-limit key.
func TestClientIPForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	// "198.51.100.5" is attacker-supplied; "10.0.0.1" was appended by the proxy.
	req.Header.Set("X-Forwarded-For", "198.51.100.5, 10.0.0.1")
	assert.Equal(t, "10.0.0.1", clientIP(req))

	// A single-entry header (proxy replaced rather than appended) is used as-is.
	reqSingle := httptest.NewRequest("GET", "/", nil)
	reqSingle.RemoteAddr = "10.0.0.9:1234"
	reqSingle.Header.Set("X-Forwarded-For", "203.0.113.42")
	assert.Equal(t, "203.0.113.42", clientIP(reqSingle))

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.2:4321"
	assert.Equal(t, "10.0.0.2", clientIP(req2))
}
