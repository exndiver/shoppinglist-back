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

func InsertCategory(ctx context.Context, db DBTX, c models.Category) error {
	_, err := db.Exec(ctx, `
		INSERT INTO categories (id, owner_id, name, normalized_name)
		VALUES ($1, $2, $3, $4)
	`, c.ID, c.OwnerID, c.Name, c.NormalizedName)
	return err
}

func UpdateCategory(ctx context.Context, db DBTX, ownerID, id uuid.UUID, name, normalized string) error {
	tag, err := db.Exec(ctx, `
		UPDATE categories
		SET name = $3,
		    normalized_name = $4,
		    updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, id, name, normalized)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
