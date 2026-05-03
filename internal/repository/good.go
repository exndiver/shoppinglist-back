package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanGood(row RowScanner) (*models.Good, error) {
	var g models.Good
	var merged pgtype.UUID
	var cb pgtype.Text
	err := row.Scan(
		&g.ID,
		&g.OwnerID,
		&g.Name,
		&g.NormalizedName,
		&merged,
		&cb,
		&g.CreatedAt,
		&g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if merged.Valid {
		u := uuid.UUID(merged.Bytes)
		g.MergedInto = &u
	}
	if cb.Valid {
		s := cb.String
		g.CreatedBy = &s
	}
	return &g, nil
}

func GetGood(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.Good, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods WHERE owner_id = $1 AND id = $2
	`, ownerID, id)
	g, err := scanGood(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return g, err
}

func InsertGood(ctx context.Context, db DBTX, g models.Good) error {
	var cb any
	if g.CreatedBy != nil {
		cb = *g.CreatedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO goods (id, owner_id, name, normalized_name, merged_into, created_by)
		VALUES ($1, $2, $3, $4, NULL, $5)
	`, g.ID, g.OwnerID, g.Name, g.NormalizedName, cb)
	return err
}

func UpdateGoodCanonical(ctx context.Context, db DBTX, ownerID, canonicalID uuid.UUID, name, normalized string, createdBy *string) error {
	var cb any
	if createdBy != nil {
		cb = *createdBy
	}
	tag, err := db.Exec(ctx, `
		UPDATE goods
		SET name = $3,
		    normalized_name = $4,
		    updated_at = now(),
		    created_by = COALESCE($5, created_by)
		WHERE owner_id = $1 AND id = $2 AND merged_into IS NULL
	`, ownerID, canonicalID, name, normalized, cb)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func MarkGoodMerged(ctx context.Context, db DBTX, ownerID, sourceCanonicalID, targetCanonicalID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE goods
		SET merged_into = $3,
		    updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND merged_into IS NULL AND id <> $3
	`, ownerID, sourceCanonicalID, targetCanonicalID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func RepointListItemsGood(ctx context.Context, db DBTX, ownerID, fromGoodID, toGoodID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE list_items SET good_id = $3, updated_at = now()
		WHERE owner_id = $1 AND good_id = $2
	`, ownerID, fromGoodID, toGoodID)
	return err
}

func SearchCanonicalGoods(ctx context.Context, db DBTX, ownerID uuid.UUID, normalizedContains string, limit int) ([]models.Good, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods
		WHERE owner_id = $1
		  AND merged_into IS NULL
		  AND ($2 = '' OR normalized_name LIKE '%' || $2 || '%')
		ORDER BY normalized_name ASC
		LIMIT $3
	`, ownerID, normalizedContains, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Good
	for rows.Next() {
		g, err := scanGood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func ListCanonicalGoodsExclude(ctx context.Context, db DBTX, ownerID uuid.UUID, exclude uuid.UUID, limit int) ([]models.Good, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods
		WHERE owner_id = $1
		  AND merged_into IS NULL
		  AND id <> $2
		ORDER BY normalized_name ASC
		LIMIT $3
	`, ownerID, exclude, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Good
	for rows.Next() {
		g, err := scanGood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}
