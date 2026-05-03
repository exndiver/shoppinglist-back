package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanStore(row RowScanner) (*models.Store, error) {
	var s models.Store
	var merged pgtype.UUID
	var cb pgtype.Text
	err := row.Scan(
		&s.ID,
		&s.OwnerID,
		&s.Name,
		&s.NormalizedName,
		&merged,
		&cb,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if merged.Valid {
		u := uuid.UUID(merged.Bytes)
		s.MergedInto = &u
	}
	if cb.Valid {
		str := cb.String
		s.CreatedBy = &str
	}
	return &s, nil
}

func GetStore(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.Store, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM stores WHERE owner_id = $1 AND id = $2
	`, ownerID, id)
	s, err := scanStore(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return s, err
}

func InsertStore(ctx context.Context, db DBTX, s models.Store) error {
	var cb any
	if s.CreatedBy != nil {
		cb = *s.CreatedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO stores (id, owner_id, name, normalized_name, merged_into, created_by)
		VALUES ($1, $2, $3, $4, NULL, $5)
	`, s.ID, s.OwnerID, s.Name, s.NormalizedName, cb)
	return err
}

func UpdateStoreCanonical(ctx context.Context, db DBTX, ownerID, canonicalID uuid.UUID, name, normalized string, createdBy *string) error {
	var cb any
	if createdBy != nil {
		cb = *createdBy
	}
	tag, err := db.Exec(ctx, `
		UPDATE stores
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

func SearchCanonicalStores(ctx context.Context, db DBTX, ownerID uuid.UUID, normalizedContains string, limit int) ([]models.Store, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM stores
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

	var out []models.Store
	for rows.Next() {
		s, err := scanStore(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}
