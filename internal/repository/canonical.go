package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func ResolveGoodCanonical(ctx context.Context, db DBTX, ownerID, goodID uuid.UUID) (uuid.UUID, error) {
	cur := goodID
	for depth := 0; depth < 64; depth++ {
		var merged pgtype.UUID
		err := db.QueryRow(ctx, `
			SELECT merged_into FROM goods WHERE owner_id = $1 AND id = $2
		`, ownerID, cur).Scan(&merged)
		if err == pgx.ErrNoRows {
			return uuid.Nil, ErrNotFound
		}
		if err != nil {
			return uuid.Nil, err
		}
		if !merged.Valid {
			return cur, nil
		}
		cur = uuid.UUID(merged.Bytes)
	}
	return uuid.Nil, ErrDepthExceeded
}

func ResolveStoreCanonical(ctx context.Context, db DBTX, ownerID, storeID uuid.UUID) (uuid.UUID, error) {
	cur := storeID
	for depth := 0; depth < 64; depth++ {
		var merged pgtype.UUID
		err := db.QueryRow(ctx, `
			SELECT merged_into FROM stores WHERE owner_id = $1 AND id = $2
		`, ownerID, cur).Scan(&merged)
		if err == pgx.ErrNoRows {
			return uuid.Nil, ErrNotFound
		}
		if err != nil {
			return uuid.Nil, err
		}
		if !merged.Valid {
			return cur, nil
		}
		cur = uuid.UUID(merged.Bytes)
	}
	return uuid.Nil, ErrDepthExceeded
}
