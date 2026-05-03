package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanProduct(row RowScanner) (*models.Product, error) {
	var p models.Product
	var merged pgtype.UUID
	var cb pgtype.Text
	err := row.Scan(
		&p.ID,
		&p.OwnerID,
		&p.Name,
		&p.NormalizedName,
		&merged,
		&cb,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if merged.Valid {
		u := uuid.UUID(merged.Bytes)
		p.MergedInto = &u
	}
	if cb.Valid {
		s := cb.String
		p.CreatedBy = &s
	}
	return &p, nil
}

func GetProduct(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.Product, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM products WHERE owner_id = $1 AND id = $2
	`, ownerID, id)
	p, err := scanProduct(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return p, err
}

func InsertProduct(ctx context.Context, db DBTX, p models.Product) error {
	var cb any
	if p.CreatedBy != nil {
		cb = *p.CreatedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO products (id, owner_id, name, normalized_name, merged_into, created_by)
		VALUES ($1, $2, $3, $4, NULL, $5)
	`, p.ID, p.OwnerID, p.Name, p.NormalizedName, cb)
	return err
}

func UpdateProductCanonical(ctx context.Context, db DBTX, ownerID, canonicalID uuid.UUID, name, normalized string, createdBy *string) error {
	var cb any
	if createdBy != nil {
		cb = *createdBy
	}
	tag, err := db.Exec(ctx, `
		UPDATE products
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

func MarkProductMerged(ctx context.Context, db DBTX, ownerID, sourceCanonicalID, targetCanonicalID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE products
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

func RepointListItemsProduct(ctx context.Context, db DBTX, ownerID, fromProductID, toProductID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE list_items SET product_id = $3, updated_at = now()
		WHERE owner_id = $1 AND product_id = $2
	`, ownerID, fromProductID, toProductID)
	return err
}

func SearchCanonicalProducts(ctx context.Context, db DBTX, ownerID uuid.UUID, normalizedContains string, limit int) ([]models.Product, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM products
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

	var out []models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func ListCanonicalProductsExclude(ctx context.Context, db DBTX, ownerID uuid.UUID, exclude uuid.UUID, limit int) ([]models.Product, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM products
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

	var out []models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}
