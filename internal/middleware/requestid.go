package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const HeaderRequestID = "X-Request-ID"

// RequestID ensures X-Request-ID on the response and a logBag on the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(HeaderRequestID))
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set(HeaderRequestID, id)

		bag := &logBag{requestID: id}
		ctx := context.WithValue(r.Context(), logBagKey, bag)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
