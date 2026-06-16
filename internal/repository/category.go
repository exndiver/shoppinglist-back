package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func scanCategory(row RowScanner) (*models.Category, error) {
	var c models.Category
	err := row.Scan(
		&c.ID,
		&c.OwnerID,
		&c.Name,
		&c.NormalizedName,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func GetCategory(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.Category, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, name, normalized_name, created_at, updated_at
		FROM categories WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, id)
	c, err := scanCategory(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return c, err
}

// UpsertCategoryReturning inserts or updates a category in one round-trip.
// Returns ErrNotFound when a soft-deleted row blocks the update.
func UpsertCategoryReturning(ctx context.Context, db DBTX, c models.Category) (*models.Category, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO categories (id, owner_id, name, normalized_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		  SET name            = EXCLUDED.name,
		      normalized_name = EXCLUDED.normalized_name,
		      updated_at      = now()
		WHERE categories.owner_id = $2
		  AND categories.deleted_at IS NULL
		RETURNING id, owner_id, name, normalized_name, created_at, updated_at
	`, c.ID, c.OwnerID, c.Name, c.NormalizedName)
	out, err := scanCategory(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return out, err
}

func SearchCategories(ctx context.Context, db DBTX, ownerID uuid.UUID, normalizedContains string, limit int) ([]models.Category, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, created_at, updated_at
		FROM categories
		WHERE owner_id = $1
		  AND `+sqlActive+`
		  AND ($2 = '' OR normalized_name ILIKE '%' || $2 || '%')
		ORDER BY normalized_name ASC
		LIMIT $3
	`, ownerID, normalizedContains, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func ListCategoriesSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]models.Category, error) {
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, created_at, updated_at
		FROM categories
		WHERE owner_id = $1 AND updated_at > $2 AND `+sqlActive+`
		ORDER BY updated_at ASC, id ASC
	`, ownerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
