package webui

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a simple per-key token-bucket limiter implemented with the
// standard library only (no external dependencies). It is safe for concurrent
// use. Each key (e.g. a client IP) gets an independent bucket that refills at
// a fixed rate up to a burst capacity.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64       // tokens added per second
	burst   float64       // maximum tokens in a bucket
	idleFor time.Duration // evict buckets untouched for this long
	now     func() time.Time
	lastGC  time.Time
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter builds a limiter allowing `burst` requests instantly and then
// `ratePerMinute` sustained requests per minute per key.
func newRateLimiter(ratePerMinute, burst int) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    float64(ratePerMinute) / 60.0,
		burst:   float64(burst),
		idleFor: 10 * time.Minute,
		now:     time.Now,
	}
}

// allow reports whether a request for key may proceed, consuming one token.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.gcLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	// Refill based on elapsed time, capped at burst.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// gcLocked drops buckets that have not been touched recently to bound memory.
// Caller must hold l.mu.
func (l *rateLimiter) gcLocked(now time.Time) {
	if now.Sub(l.lastGC) < l.idleFor {
		return
	}
	l.lastGC = now
	for k, b := range l.buckets {
		if now.Sub(b.last) > l.idleFor {
			delete(l.buckets, k)
		}
	}
}

// rateLimitMiddleware returns middleware that throttles requests per client IP
// using the given limiter. Over-limit requests get 429 Too Many Requests.
func rateLimitMiddleware(l *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(clientIP(r)) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the best-effort client IP for rate-limiting keys.
//
// It uses the RIGHT-MOST X-Forwarded-For entry, not the left-most. The left-most
// entry is fully attacker-controlled: a client can send any
// "X-Forwarded-For: 1.2.3.4" it likes, and the trusted ingress only *appends*
// the real peer, so the right-most value is the one our own proxy added and
// cannot be forged from outside. Trusting the left-most entry (the previous
// behaviour) let any caller rotate the value per request and bypass the per-IP
// limit entirely. Falls back to the transport remote address when no header is
// present (e.g. direct intra-cluster requests).
//
// This assumes a single trusted proxy hop in front of the gateway (the
// Kubernetes ingress). Behind N chained trusted proxies, the limiter should be
// extended to skip N right-most hops.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.LastIndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[i+1:])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
