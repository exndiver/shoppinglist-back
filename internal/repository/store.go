package repository

import (
	"context"
	"time"

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
		FROM stores WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, id)
	s, err := scanStore(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return s, err
}

func InsertStoreReturning(ctx context.Context, db DBTX, s models.Store) (*models.Store, error) {
	var cb any
	if s.CreatedBy != nil {
		cb = *s.CreatedBy
	}
	row := db.QueryRow(ctx, `
		INSERT INTO stores (id, owner_id, name, normalized_name, merged_into, created_by)
		VALUES ($1, $2, $3, $4, NULL, $5)
		RETURNING id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
	`, s.ID, s.OwnerID, s.Name, s.NormalizedName, cb)
	out, err := scanStore(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return out, err
}

func UpdateStoreCanonicalReturning(ctx context.Context, db DBTX, ownerID, canonicalID uuid.UUID, name, normalized string, createdBy *string) (*models.Store, error) {
	var cb any
	if createdBy != nil {
		cb = *createdBy
	}
	row := db.QueryRow(ctx, `
		UPDATE stores
		SET name            = $3,
		    normalized_name = $4,
		    updated_at      = now(),
		    created_by      = COALESCE($5, created_by)
		WHERE owner_id = $1 AND id = $2 AND merged_into IS NULL AND `+sqlActive+`
		RETURNING id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
	`, ownerID, canonicalID, name, normalized, cb)
	out, err := scanStore(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return out, err
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
		  AND `+sqlActive+`
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

func ListStoresSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]models.Store, error) {
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM stores
		WHERE owner_id = $1 AND updated_at > $2 AND `+sqlActive+`
		ORDER BY updated_at ASC, id ASC
	`, ownerID, since)
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
