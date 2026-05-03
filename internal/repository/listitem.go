package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanListItem(row RowScanner) (*models.ListItem, error) {
	var it models.ListItem
	var offer pgtype.UUID
	var snap pgtype.Float8
	var cb pgtype.Text
	err := row.Scan(
		&it.ID,
		&it.OwnerID,
		&it.ListID,
		&it.GoodID,
		&offer,
		&it.Quantity,
		&snap,
		&it.IsPurchased,
		&cb,
		&it.CreatedAt,
		&it.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if offer.Valid {
		u := uuid.UUID(offer.Bytes)
		it.OfferID = &u
	}
	if snap.Valid {
		v := snap.Float64
		it.PriceSnapshot = &v
	}
	if cb.Valid {
		s := cb.String
		it.CreatedBy = &s
	}
	return &it, nil
}

func InsertListItem(ctx context.Context, db DBTX, it models.ListItem) error {
	var offer any
	var snap any
	var cb any
	if it.OfferID != nil {
		offer = *it.OfferID
	}
	if it.PriceSnapshot != nil {
		snap = *it.PriceSnapshot
	}
	if it.CreatedBy != nil {
		cb = *it.CreatedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO list_items (
		  id, owner_id, list_id, good_id, offer_id, quantity, price_snapshot, is_purchased, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, it.ID, it.OwnerID, it.ListID, it.GoodID, offer, it.Quantity, snap, it.IsPurchased, cb)
	return err
}

func GetListItem(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.ListItem, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, list_id, good_id, offer_id, quantity, price_snapshot, is_purchased, created_by, created_at, updated_at
		FROM list_items WHERE owner_id = $1 AND id = $2
	`, ownerID, id)
	it, err := scanListItem(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return it, err
}

func ListItemsByList(ctx context.Context, db DBTX, ownerID, listID uuid.UUID) ([]models.ListItem, error) {
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, list_id, good_id, offer_id, quantity, price_snapshot, is_purchased, created_by, created_at, updated_at
		FROM list_items
		WHERE owner_id = $1 AND list_id = $2
		ORDER BY created_at ASC, id ASC
	`, ownerID, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ListItem
	for rows.Next() {
		it, err := scanListItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// ListItemPatch updates only fields explicitly provided by the caller.
type ListItemPatch struct {
	Quantity       *float64
	IsPurchased    *bool
	OfferID        *uuid.UUID
	OfferIDPresent bool // true if JSON contained offer_id key (including explicit null → clear)
}

func PatchListItem(ctx context.Context, db DBTX, ownerID, id uuid.UUID, patch ListItemPatch) error {
	it, err := GetListItem(ctx, db, ownerID, id)
	if err != nil {
		return err
	}
	q := it.Quantity
	if patch.Quantity != nil {
		q = *patch.Quantity
	}
	ip := it.IsPurchased
	if patch.IsPurchased != nil {
		ip = *patch.IsPurchased
	}

	var offerArg any
	switch {
	case patch.OfferIDPresent:
		if patch.OfferID != nil {
			offerArg = *patch.OfferID
		} else {
			offerArg = nil
		}
	default:
		if it.OfferID != nil {
			offerArg = *it.OfferID
		} else {
			offerArg = nil
		}
	}

	tag, err := db.Exec(ctx, `
		UPDATE list_items
		SET quantity = $3,
		    is_purchased = $4,
		    offer_id = $5,
		    updated_at = now()
		WHERE owner_id = $1 AND id = $2
	`, ownerID, id, q, ip, offerArg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
