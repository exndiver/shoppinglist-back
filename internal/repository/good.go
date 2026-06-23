package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanGood(row RowScanner) (*models.Good, error) {
	var g models.Good
	var catID pgtype.UUID
	var merged pgtype.UUID
	var cb pgtype.Text
	err := row.Scan(
		&g.ID,
		&g.OwnerID,
		&catID,
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
	if catID.Valid {
		u := uuid.UUID(catID.Bytes)
		g.CategoryID = &u
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
		SELECT id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, id)
	g, err := scanGood(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return g, err
}

// GetGoodAny fetches a good by id regardless of owner. Used to render the name
// of a collaborator's good (which lives in their scope) on a shared list item.
func GetGoodAny(ctx context.Context, db DBTX, id uuid.UUID) (*models.Good, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods WHERE id = $1 AND `+sqlActive+`
	`, id)
	g, err := scanGood(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return g, err
}

func InsertGoodReturning(ctx context.Context, db DBTX, g models.Good) (*models.Good, error) {
	var cb any
	if g.CreatedBy != nil {
		cb = *g.CreatedBy
	}
	row := db.QueryRow(ctx, `
		INSERT INTO goods (id, owner_id, category_id, name, normalized_name, merged_into, created_by)
		VALUES ($1, $2, $3, $4, $5, NULL, $6)
		RETURNING id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
	`, g.ID, g.OwnerID, g.CategoryID, g.Name, g.NormalizedName, cb)
	out, err := scanGood(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return out, err
}

func UpdateGoodCanonicalReturning(ctx context.Context, db DBTX, ownerID, canonicalID uuid.UUID, categoryID *uuid.UUID, name, normalized string, createdBy *string) (*models.Good, error) {
	var cb any
	if createdBy != nil {
		cb = *createdBy
	}
	row := db.QueryRow(ctx, `
		UPDATE goods
		SET category_id     = $3,
		    name             = $4,
		    normalized_name  = $5,
		    updated_at       = now(),
		    created_by       = COALESCE($6, created_by)
		WHERE owner_id = $1 AND id = $2 AND merged_into IS NULL AND `+sqlActive+`
		RETURNING id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
	`, ownerID, canonicalID, categoryID, name, normalized, cb)
	out, err := scanGood(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return out, err
}

func MarkGoodMerged(ctx context.Context, db DBTX, ownerID, sourceCanonicalID, targetCanonicalID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE goods
		SET merged_into = $3,
		    updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND merged_into IS NULL AND id <> $3 AND `+sqlActive+`
	`, ownerID, sourceCanonicalID, targetCanonicalID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = db.Exec(ctx, `
		UPDATE goods SET updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, targetCanonicalID)
	return err
}

// TransferCategoryOnGoodMerge copies category from source to target when target has none.
func TransferCategoryOnGoodMerge(ctx context.Context, db DBTX, ownerID, sourceCanonicalID, targetCanonicalID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE goods tgt
		SET category_id = src.category_id,
		    updated_at = now()
		FROM goods src
		WHERE src.owner_id = $1 AND src.id = $2
		  AND tgt.owner_id = $1 AND tgt.id = $3
		  AND tgt.merged_into IS NULL
		  AND src.deleted_at IS NULL
		  AND tgt.deleted_at IS NULL
		  AND src.category_id IS NOT NULL
		  AND tgt.category_id IS NULL
	`, ownerID, sourceCanonicalID, targetCanonicalID)
	return err
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
		SELECT id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods
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
		SELECT id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods
		WHERE owner_id = $1
		  AND merged_into IS NULL
		  AND `+sqlActive+`
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

func ListGoodsSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]models.Good, error) {
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, category_id, name, normalized_name, merged_into, created_by, created_at, updated_at
		FROM goods
		WHERE owner_id = $1 AND updated_at > $2 AND `+sqlActive+`
		ORDER BY updated_at ASC, id ASC
	`, ownerID, since)
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
