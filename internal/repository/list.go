package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanShoppingList(row RowScanner) (*models.ShoppingList, error) {
	var l models.ShoppingList
	var cb pgtype.Text
	err := row.Scan(&l.ID, &l.OwnerID, &l.Name, &cb, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if cb.Valid {
		s := cb.String
		l.CreatedBy = &s
	}
	return &l, nil
}

func InsertShoppingList(ctx context.Context, db DBTX, l models.ShoppingList) error {
	var cb any
	if l.CreatedBy != nil {
		cb = *l.CreatedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO shopping_lists (id, owner_id, name, created_by)
		VALUES ($1, $2, $3, $4)
	`, l.ID, l.OwnerID, l.Name, cb)
	return err
}

func UpdateShoppingList(ctx context.Context, db DBTX, ownerID, id uuid.UUID, name string, createdBy *string) error {
	var cb any
	if createdBy != nil {
		cb = *createdBy
	}
	tag, err := db.Exec(ctx, `
		UPDATE shopping_lists
		SET name = $3,
		    updated_at = now(),
		    created_by = COALESCE($4, created_by)
		WHERE owner_id = $1 AND id = $2
	`, ownerID, id, name, cb)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func GetShoppingList(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.ShoppingList, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, name, created_by, created_at, updated_at
		FROM shopping_lists WHERE owner_id = $1 AND id = $2
	`, ownerID, id)
	l, err := scanShoppingList(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return l, err
}

func ListShoppingLists(ctx context.Context, db DBTX, ownerID uuid.UUID, limit int) ([]models.ShoppingList, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, name, created_by, created_at, updated_at
		FROM shopping_lists
		WHERE owner_id = $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ShoppingList
	for rows.Next() {
		l, err := scanShoppingList(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}
