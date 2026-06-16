package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GoodMergeWouldCycle reports whether merging fromID into toID would create a merge cycle.
// true, nil  — cycle detected (или fromID == toID), операцию надо отклонить.
// false, nil — безопасно.
// false, ErrNotFound — toID или кто-то из его предков не существует.
// true, ErrDepthExceeded — цепочка длиннее 64 (защита от зациклившихся данных).
func GoodMergeWouldCycle(ctx context.Context, db DBTX, ownerID, fromID, toID uuid.UUID) (bool, error) {
	if fromID == toID {
		return true, nil
	}
	// Идём по цепочке merged_into от toID; если встретим fromID — цикл.
	var hit, overflow bool
	err := db.QueryRow(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, merged_into, 0 AS depth
			FROM goods
			WHERE owner_id = $1 AND id = $3 AND `+sqlActive+`
			UNION ALL
			SELECT g.id, g.merged_into, c.depth + 1
			FROM goods g
			JOIN chain c ON g.id = c.merged_into
			WHERE g.owner_id = $1 AND g.`+sqlActiveFrag+` AND c.depth < 64
		)
		SELECT EXISTS(SELECT 1 FROM chain WHERE id = $2),
		       EXISTS(SELECT 1 FROM chain WHERE depth >= 64)
	`, ownerID, fromID, toID).Scan(&hit, &overflow)
	if err == pgx.ErrNoRows {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if overflow {
		return true, ErrDepthExceeded
	}
	return hit, nil
}
