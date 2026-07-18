package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/exndiver/shopping-backend/internal/httpx"
)

// RateLimit applies an in-memory token-bucket limiter (approx. rps) PER CLIENT,
// so one abusive client cannot starve everyone else. rps <= 0 disables
// limiting. burst <= 0 falls back to rps. Public paths (/health, /metrics,
// /version) bypass the limiter.
//
// The client key is the bearer owner id when present (cheap header parse — this
// middleware runs before BearerOwner), else the originating IP
// (CF-Connecting-IP / first X-Forwarded-For hop / RemoteAddr).
//
// Buckets idle for a while are evicted lazily on access, keeping memory
// bounded. Single-instance only; multi-instance deployments need a shared
// store (Redis) or an external limiter.
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

	rl := &keyedLimiter{
		buckets:  make(map[string]*tokenBucket),
		max:      burst,
		interval: interval,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.take(clientKey(r)) {
			httpx.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientKey identifies the caller for limiting: bearer token if present
// (invalid tokens still bucket together per raw value, which is fine — they
// get 401 right after), else the originating IP.
func clientKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) {
			if tok := strings.TrimSpace(auth[len(prefix):]); tok != "" {
				return "o:" + tok
			}
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return "ip:" + ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return "ip:" + strings.TrimSpace(first)
		}
		return "ip:" + strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "ip:" + r.RemoteAddr
	}
	return "ip:" + host
}

// evictAfter is how long a bucket may sit unused before lazy eviction.
const evictAfter = 10 * time.Minute

// evictEvery limits how often a full eviction sweep runs.
const evictEvery = time.Minute

type keyedLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	max       int
	interval  time.Duration
	lastSweep time.Time
}

func (l *keyedLimiter) take(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.Sub(l.lastSweep) > evictEvery {
		for k, b := range l.buckets {
			if now.Sub(b.last) > evictAfter {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	b := l.buckets[key]
	if b == nil {
		b = &tokenBucket{tokens: l.max, max: l.max, interval: l.interval, last: now}
		l.buckets[key] = b
	}
	return b.take()
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
