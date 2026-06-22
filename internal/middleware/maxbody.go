package middleware

import (
	"net/http"
)

// MaxBody caps the request body size to defend against oversized payloads.
// max <= 0 disables the limit. When the limit is exceeded, downstream
// json.Decode returns an error (http.MaxBytesError) and the handler emits 400.
func MaxBody(max int64, next http.Handler) http.Handler {
	if max <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, max)
		}
		next.ServeHTTP(w, r)
	})
}
