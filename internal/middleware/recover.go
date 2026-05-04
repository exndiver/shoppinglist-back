package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/exndiver/shopping-backend/internal/httpx"
)

type headerSentWriter struct {
	http.ResponseWriter
	sent bool
}

func (w *headerSentWriter) WriteHeader(code int) {
	if w.sent {
		return
	}
	w.sent = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *headerSentWriter) Write(b []byte) (int, error) {
	if !w.sent {
		w.sent = true
	}
	return w.ResponseWriter.Write(b)
}

// Recover logs panics with stack trace and returns 500 JSON if headers were not sent.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hw := &headerSentWriter{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				attrs := []slog.Attr{
					slog.Any("error", rec),
					slog.String("stack", string(stack)),
					slog.String("http.method", r.Method),
					slog.String("http.path", r.URL.Path),
				}
				if bag := logBagFrom(r.Context()); bag != nil {
					attrs = append(attrs, slog.String("request.id", bag.requestID))
				}
				slog.LogAttrs(r.Context(), slog.LevelError, "panic_recovered", attrs...)

				if !hw.sent {
					httpx.WriteError(hw, http.StatusInternalServerError, "INTERNAL", "internal server error")
				}
			}
		}()
		next.ServeHTTP(hw, r)
	})
}
