package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanOffer(row RowScanner) (*models.Offer, error) {
	var o models.Offer
	var cb pgtype.Text
	err := row.Scan(
		&o.ID,
		&o.OwnerID,
		&o.GoodID,
		&o.StoreID,
		&cb,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if cb.Valid {
		s := cb.String
		o.CreatedBy = &s
	}
	return &o, nil
}

func InsertOffer(ctx context.Context, db DBTX, o models.Offer) error {
	var cb any
	if o.CreatedBy != nil {
		cb = *o.CreatedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO offers (id, owner_id, good_id, store_id, created_by)
		VALUES ($1, $2, $3, $4, $5)
	`, o.ID, o.OwnerID, o.GoodID, o.StoreID, cb)
	return err
}

func GetOffer(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.Offer, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, good_id, store_id, created_by, created_at, updated_at
		FROM offers WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, id)
	o, err := scanOffer(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return o, err
}

func FindOfferByTriple(ctx context.Context, db DBTX, ownerID, goodID, storeID uuid.UUID) (*models.Offer, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, good_id, store_id, created_by, created_at, updated_at
		FROM offers WHERE owner_id = $1 AND good_id = $2 AND store_id = $3 AND `+sqlActive+`
	`, ownerID, goodID, storeID)
	o, err := scanOffer(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return o, err
}

func TouchOfferUpdatedAt(ctx context.Context, db DBTX, ownerID, offerID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE offers SET updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, offerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func UpdateOfferGoodID(ctx context.Context, db DBTX, ownerID, offerID, newGoodID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE offers SET good_id = $3, updated_at = now()
		WHERE owner_id = $1 AND id = $2 AND `+sqlActive+`
	`, ownerID, offerID, newGoodID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func DeleteOffer(ctx context.Context, db DBTX, ownerID, offerID uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM offers WHERE owner_id = $1 AND id = $2`, ownerID, offerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func RepointPriceRecords(ctx context.Context, db DBTX, fromOfferID, toOfferID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE price_records SET offer_id = $2 WHERE offer_id = $1
	`, fromOfferID, toOfferID)
	return err
}

func ListOffersForGood(ctx context.Context, db DBTX, ownerID, goodID uuid.UUID) ([]models.Offer, error) {
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, good_id, store_id, created_by, created_at, updated_at
		FROM offers
		WHERE owner_id = $1 AND good_id = $2 AND `+sqlActive+`
	`, ownerID, goodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Offer
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}



func ListOffersSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]models.Offer, error) {
	var rows pgx.Rows
	var err error
	if since.IsZero() {
		rows, err = db.Query(ctx, `
			SELECT id, owner_id, good_id, store_id, created_by, created_at, updated_at
			FROM offers
			WHERE owner_id = $1 AND `+sqlActive+`
			ORDER BY updated_at ASC, id ASC
		`, ownerID)
	} else {
		rows, err = db.Query(ctx, `
			SELECT id, owner_id, good_id, store_id, created_by, created_at, updated_at
			FROM offers
			WHERE owner_id = $1 AND updated_at > $2 AND `+sqlActive+`
			ORDER BY updated_at ASC, id ASC
		`, ownerID, since)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Offer
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}
