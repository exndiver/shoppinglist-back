package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/exndiver/shopping-backend/internal/contextkey"
	"github.com/exndiver/shopping-backend/internal/httpx"
	"github.com/google/uuid"
)

const HeaderDeviceID = "X-Device-Id"

// IsPublicPath reports whether a path must be reachable without bearer auth.
func IsPublicPath(path string) bool {
	return path == "/health" || path == "/metrics"
}

// BearerOwner extracts Authorization: Bearer <owner_uuid> into context as owner id.
// Public paths (health, metrics) bypass authentication.
func BearerOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		raw := r.Header.Get("Authorization")
		if raw == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing Authorization header")
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(raw, prefix) {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid Authorization scheme")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(raw, prefix))
		ownerID, err := uuid.Parse(token)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid bearer token (expected UUID)")
			return
		}
		ctx := context.WithValue(r.Context(), contextkey.OwnerID, ownerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func OwnerFromContext(ctx context.Context) (uuid.UUID, bool) {
	v := ctx.Value(contextkey.OwnerID)
	id, ok := v.(uuid.UUID)
	return id, ok
}
