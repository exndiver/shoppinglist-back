package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanPriceRecord(row RowScanner) (*models.PriceRecord, error) {
	var pr models.PriceRecord
	var pack pgtype.Float8
	var unit pgtype.Text
	var rb pgtype.Text
	err := row.Scan(
		&pr.ID,
		&pr.OwnerID,
		&pr.OfferID,
		&pr.Price,
		&pack,
		&unit,
		&pr.RecordedAt,
		&rb,
		&pr.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if pack.Valid {
		v := pack.Float64
		pr.PackSize = &v
	}
	if unit.Valid {
		u := unit.String
		pr.Unit = &u
	}
	if rb.Valid {
		s := rb.String
		pr.RecordedBy = &s
	}
	return &pr, nil
}

func InsertPriceRecord(ctx context.Context, db DBTX, pr models.PriceRecord) error {
	var pack any
	var unit any
	var rb any
	if pr.PackSize != nil {
		pack = *pr.PackSize
	}
	if pr.Unit != nil {
		unit = *pr.Unit
	}
	if pr.RecordedBy != nil {
		rb = *pr.RecordedBy
	}
	_, err := db.Exec(ctx, `
		INSERT INTO price_records (id, owner_id, offer_id, price, pack_size, unit, recorded_at, recorded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, pr.ID, pr.OwnerID, pr.OfferID, pr.Price, pack, unit, pr.RecordedAt, rb)
	return err
}

func GetPriceRecord(ctx context.Context, db DBTX, ownerID, id uuid.UUID) (*models.PriceRecord, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, offer_id, price, pack_size, unit, recorded_at, recorded_by, created_at
		FROM price_records WHERE owner_id = $1 AND id = $2
	`, ownerID, id)
	pr, err := scanPriceRecord(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return pr, err
}

func ListPricesForOffer(ctx context.Context, db DBTX, ownerID, offerID uuid.UUID, limit int) ([]models.PriceRecord, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	rows, err := db.Query(ctx, `
		SELECT id, owner_id, offer_id, price, pack_size, unit, recorded_at, recorded_by, created_at
		FROM price_records
		WHERE owner_id = $1 AND offer_id = $2
		ORDER BY recorded_at ASC, id ASC
		LIMIT $3
	`, ownerID, offerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.PriceRecord
	for rows.Next() {
		pr, err := scanPriceRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *pr)
	}
	return out, rows.Err()
}

func LatestPriceForOffer(ctx context.Context, db DBTX, ownerID, offerID uuid.UUID) (*models.PriceRecord, error) {
	row := db.QueryRow(ctx, `
		SELECT id, owner_id, offer_id, price, pack_size, unit, recorded_at, recorded_by, created_at
		FROM price_records
		WHERE owner_id = $1 AND offer_id = $2
		ORDER BY recorded_at DESC, id DESC
		LIMIT 1
	`, ownerID, offerID)
	pr, err := scanPriceRecord(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return pr, err
}

// PricesLikelyEqual compares fields relevant for idempotent replay (MVP).
func PricesLikelyEqual(a, b *models.PriceRecord) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != b.ID {
		return false
	}
	if a.OwnerID != b.OwnerID || a.OfferID != b.OfferID {
		return false
	}
	if a.Price != b.Price {
		return false
	}
	ptrEq := func(x, y *float64) bool {
		switch {
		case x == nil && y == nil:
			return true
		case x != nil && y != nil && *x == *y:
			return true
		default:
			return false
		}
	}
	if !ptrEq(a.PackSize, b.PackSize) {
		return false
	}
	ptrEqStr := func(x, y *string) bool {
		switch {
		case x == nil && y == nil:
			return true
		case x != nil && y != nil && *x == *y:
			return true
		default:
			return false
		}
	}
	if !ptrEqStr(a.Unit, b.Unit) {
		return false
	}
	return a.RecordedAt.UTC().Truncate(time.Millisecond).Equal(b.RecordedAt.UTC().Truncate(time.Millisecond))
}
