package middleware

import (
	"context"

	"github.com/google/uuid"
)

type logCtxKey int

const logBagKey logCtxKey = iota

// logBag holds per-request attributes attached in RequestID.
type logBag struct {
	requestID string
}

func logBagFrom(ctx context.Context) *logBag {
	v := ctx.Value(logBagKey)
	if v == nil {
		return nil
	}
	b, _ := v.(*logBag)
	return b
}

// RequestIDOf returns the request id stored on the context, if any.
func RequestIDOf(ctx context.Context) string {
	if b := logBagFrom(ctx); b != nil {
		return b.requestID
	}
	return ""
}

// OwnerIDOf is a thin alias over OwnerFromContext for logging helpers.
func OwnerIDOf(ctx context.Context) (uuid.UUID, bool) {
	return OwnerFromContext(ctx)
}
