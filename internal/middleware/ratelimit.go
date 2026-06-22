package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/exndiver/shopping-backend/internal/httpx"
)

// RateLimit applies a global in-memory token-bucket limiter (approx. rps).
// rps <= 0 disables limiting. burst <= 0 falls back to rps.
// Public paths (/health, /metrics) bypass the limiter.
//
// Implementation note: a fixed refill of `burst` tokens every `1/rps` window
// via a mutex. Good enough for a single-instance pet project; for multi-instance
// deployments use a shared store (Redis) or an external limiter.
func RateLimit(rps float64, burst int, next http.Handler) http.Handler {
	if rps <= 0 {
		return next
	}
	if burst <= 0 {
		burst = int(rps)
		if burst < 1 {
			burst = 1
		}
	}
	interval := time.Duration(float64(time.Second) / rps)
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	rl := &tokenBucket{
		tokens:   burst,
		max:      burst,
		interval: interval,
		last:     time.Now(),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.take() {
			httpx.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type tokenBucket struct {
	mu       sync.Mutex
	tokens   int
	max      int
	interval time.Duration
	last     time.Time
}

func (b *tokenBucket) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	refill := int(now.Sub(b.last) / b.interval)
	if refill > 0 {
		b.tokens += refill
		if b.tokens > b.max {
			b.tokens = b.max
		}
		b.last = b.last.Add(time.Duration(refill) * b.interval)
		if b.last.After(now) {
			b.last = now
		}
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}
