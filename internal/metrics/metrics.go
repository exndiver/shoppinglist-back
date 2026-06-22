// Package metrics provides a minimal Prometheus-text-exposition collector and
// HTTP middleware for RED metrics (requests, errors, duration) without pulling
// an external client library.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	// Buckets in seconds, aligned with common Prometheus defaults for HTTP latency.
	bucketLe0 = 0.005
	bucketLe1 = 0.01
	bucketLe2 = 0.025
	bucketLe3 = 0.05
	bucketLe4 = 0.1
	bucketLe5 = 0.25
	bucketLe6 = 0.5
	bucketLe7 = 1
	bucketLe8 = 2.5
	bucketLe9 = 5
	bucketLeA = 10
	infBucket = "+Inf"
)

var buckets = []float64{
	bucketLe0, bucketLe1, bucketLe2, bucketLe3, bucketLe4,
	bucketLe5, bucketLe6, bucketLe7, bucketLe8, bucketLe9, bucketLeA,
}

type Collector struct {
	mu            sync.Mutex
	requestsTotal map[string]int64 // key: "<status_class>" e.g. "2xx","4xx","5xx"
	errorsTotal   int64
	durationCount int64
	durationSum   float64
	durationBkt   [11]int64 // aligned with buckets
}

var defaultCollector = &Collector{
	requestsTotal: make(map[string]int64),
}

// Middleware records RED metrics for every request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		dur := time.Since(start).Seconds()
		defaultCollector.observe(sw.status, dur)
	})
}

// Handler exposes the metrics in Prometheus text exposition format.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = io.WriteString(w, defaultCollector.render())
	})
}

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

func (c *Collector) observe(status int, durSec float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cls := statusClass(status)
	c.requestsTotal[cls]++
	if status >= 500 {
		c.errorsTotal++
	}
	c.durationCount++
	c.durationSum += durSec
	for i, le := range buckets {
		if durSec <= le {
			c.durationBkt[i]++
		}
	}
}

func (c *Collector) render() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var b []byte
	b = append(b, []byte("# HELP http_requests_total Total HTTP requests by status class.\n")...)
	b = append(b, []byte("# TYPE http_requests_total counter\n")...)
	for _, cls := range []string{"2xx", "3xx", "4xx", "5xx"} {
		b = append(b, []byte(fmt.Sprintf("http_requests_total{status=%q} %d\n", cls, c.requestsTotal[cls]))...)
	}

	b = append(b, []byte("# HELP http_request_errors_total Total HTTP server errors (5xx).\n")...)
	b = append(b, []byte("# TYPE http_request_errors_total counter\n")...)
	b = append(b, []byte(fmt.Sprintf("http_request_errors_total %d\n", c.errorsTotal))...)

	b = append(b, []byte("# HELP http_request_duration_seconds HTTP request latency.\n")...)
	b = append(b, []byte("# TYPE http_request_duration_seconds histogram\n")...)
	for i, le := range buckets {
		b = append(b, []byte(fmt.Sprintf("http_request_duration_seconds_bucket{le=%q} %d\n",
			strconv.FormatFloat(le, 'g', -1, 64), c.durationBkt[i]))...)
	}
	b = append(b, []byte(fmt.Sprintf("http_request_duration_seconds_bucket{le=%q} %d\n", infBucket, c.durationCount))...)
	b = append(b, []byte(fmt.Sprintf("http_request_duration_seconds_sum %s\n", strconv.FormatFloat(c.durationSum, 'g', -1, 64)))...)
	b = append(b, []byte(fmt.Sprintf("http_request_duration_seconds_count %d\n", c.durationCount))...)

	return string(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}
