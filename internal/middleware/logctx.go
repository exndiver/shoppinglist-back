package middleware

import (
	"context"

	"github.com/google/uuid"
)

type logCtxKey int

const logBagKey logCtxKey = iota

// logBag is attached in RequestID and updated in BearerOwner for access logs.
type logBag struct {
	requestID string
	ownerID   *uuid.UUID
}

func logBagFrom(ctx context.Context) *logBag {
	v := ctx.Value(logBagKey)
	if v == nil {
		return nil
	}
	b, _ := v.(*logBag)
	return b
}

// attachOwnerID records the authenticated owner for access logging (same request tree as RequestID).
func attachOwnerID(ctx context.Context, id uuid.UUID) {
	if b := logBagFrom(ctx); b != nil {
		b.ownerID = &id
	}
}
