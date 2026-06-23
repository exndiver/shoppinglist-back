package repository

import (
	"context"
	"time"

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
		SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity, li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
		FROM list_items li
		INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.owner_id = li.owner_id AND sl.deleted_at IS NULL
		WHERE li.owner_id = $1 AND li.id = $2 AND li.deleted_at IS NULL
	`, ownerID, id)
	it, err := scanListItem(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return it, err
}

// GetListItemByID fetches an item by id regardless of owner (the item lives in
// the list owner's scope). Used for collaborative edits where the caller is an
// edit-access member, not the owner; the caller's access is checked separately.
func GetListItemByID(ctx context.Context, db DBTX, id uuid.UUID) (*models.ListItem, error) {
	row := db.QueryRow(ctx, `
		SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity, li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
		FROM list_items li
		INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.deleted_at IS NULL
		WHERE li.id = $1 AND li.deleted_at IS NULL
	`, id)
	it, err := scanListItem(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return it, err
}

func ListItemsByList(ctx context.Context, db DBTX, ownerID, listID uuid.UUID) ([]models.ListItem, error) {
	rows, err := db.Query(ctx, `
		SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity, li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
		FROM list_items li
		INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.owner_id = li.owner_id AND sl.deleted_at IS NULL
		WHERE li.owner_id = $1 AND li.list_id = $2
		ORDER BY li.created_at ASC, li.id ASC
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
		  AND EXISTS (
		    SELECT 1 FROM shopping_lists sl
		    WHERE sl.id = list_items.list_id AND sl.owner_id = list_items.owner_id AND sl.deleted_at IS NULL
		  )
	`, ownerID, id, q, ip, offerArg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ListListItemsSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]models.ListItem, error) {
	var rows pgx.Rows
	var err error
	// Scope by LIST ownership (sl.owner_id = $1), not item ownership, so the
	// owner of a shared list receives items a collaborator added regardless of
	// which owner_id those item rows carry. Items on the owner's own lists all
	// match too. Items on lists shared *to* this caller are delivered separately
	// via the shared-sync path.
	if since.IsZero() {
		rows, err = db.Query(ctx, `
			SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity, li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
			FROM list_items li
			INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.owner_id = $1 AND sl.deleted_at IS NULL
			WHERE li.deleted_at IS NULL
			ORDER BY li.updated_at ASC, li.id ASC
		`, ownerID)
	} else {
		rows, err = db.Query(ctx, `
			SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity, li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
			FROM list_items li
			INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.owner_id = $1 AND sl.deleted_at IS NULL
			WHERE li.updated_at > $2 AND li.deleted_at IS NULL
			ORDER BY li.updated_at ASC, li.id ASC
		`, ownerID, since)
	}
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

// SoftDeleteListItem tombstones an item (so the deletion can sync to other
// devices) scoped to the owner whose data the item lives in. Returns ErrNotFound
// if the item is missing or already deleted.
func SoftDeleteListItem(ctx context.Context, db DBTX, ownerID, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE list_items SET deleted_at = now(), updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL
	`, ownerID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDeletedListItemIDsForOwnerSince returns ids of items tombstoned on the
// owner's own lists since the given time, so the owner's other devices drop them.
func ListDeletedListItemIDsForOwnerSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `
		SELECT li.id
		FROM list_items li
		INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.owner_id = $1
		WHERE li.deleted_at IS NOT NULL AND li.updated_at > $2
		ORDER BY li.updated_at ASC, li.id ASC
	`, ownerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUUIDs(rows)
}

func scanUUIDs(rows pgx.Rows) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
