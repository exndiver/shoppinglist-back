package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog emits one structured line per request (latency, status, route, optional owner).
func AccessLog(slowThreshold time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusCapture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		if sw.status == 0 {
			sw.status = http.StatusOK
		}

		attrs := []slog.Attr{
			slog.String("http.method", r.Method),
			slog.String("http.path", r.URL.Path),
			slog.Int("http.status_code", sw.status),
			slog.Float64("http.duration_ms", float64(dur.Microseconds())/1000),
		}
		if pat := r.Pattern; pat != "" {
			attrs = append(attrs, slog.String("http.route", pat))
		}
		if ra := r.RemoteAddr; ra != "" {
			attrs = append(attrs, slog.String("client.address", ra))
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, slog.String("user_agent.original", ua))
		}
		if sw.bytes > 0 {
			attrs = append(attrs, slog.Int64("http.response.size_bytes", sw.bytes))
		}

		if bag := logBagFrom(r.Context()); bag != nil {
			attrs = append(attrs, slog.String("request.id", bag.requestID))
		}
		if ownerID, ok := OwnerIDOf(r.Context()); ok {
			attrs = append(attrs, slog.String("enduser.id", ownerID.String()))
		}

		// Уровень по объёму шума: 4xx остаётся info (фильтруйте по http.status_code в SIEM).
		level := slog.LevelInfo
		if sw.status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		if slowThreshold > 0 && dur > slowThreshold && level < slog.LevelWarn {
			level = slog.LevelWarn
		}

		slog.LogAttrs(r.Context(), level, "http.server.request", attrs...)
	})
}

type statusCapture struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusCapture) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusCapture) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}
