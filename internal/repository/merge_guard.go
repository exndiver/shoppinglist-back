package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GoodMergeWouldCycle reports whether merging fromID into toID would create a merge cycle.
func GoodMergeWouldCycle(ctx context.Context, db DBTX, ownerID, fromID, toID uuid.UUID) (bool, error) {
	if fromID == toID {
		return true, nil
	}
	cur := toID
	for depth := 0; depth < 64; depth++ {
		if cur == fromID {
			return true, nil
		}
		var merged pgtype.UUID
		err := db.QueryRow(ctx, `
			SELECT merged_into FROM goods
			WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
		`, ownerID, cur).Scan(&merged)
		if err == pgx.ErrNoRows {
			return false, ErrNotFound
		}
		if err != nil {
			return false, err
		}
		if !merged.Valid {
			return false, nil
		}
		cur = uuid.UUID(merged.Bytes)
	}
	return true, ErrDepthExceeded
}
