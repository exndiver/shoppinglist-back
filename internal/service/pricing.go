package service

import (
	"context"
	"errors"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/pgutil"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) AddPriceRecord(ctx context.Context, ownerID uuid.UUID, id, offerID uuid.UUID, price float64, packSize *float64, unit *string, recordedAt time.Time, recordedBy *string) (*models.PriceRecord, error) {
	// The offer may have been created by another participant of a shared list
	// (the client reuses whichever offer it holds for a good+store pair), so
	// resolve it owner-agnostically. The price record stays owned by the caller
	// — attribution is preserved, and readers surface the newest record.
	off, err := repository.GetOfferAny(ctx, s.Pool, offerID)
	if err != nil {
		return nil, err
	}

	pr := models.PriceRecord{
		ID:         id,
		OwnerID:    ownerID,
		OfferID:    off.ID,
		Price:      price,
		PackSize:   packSize,
		Unit:       unit,
		RecordedAt: recordedAt.UTC(),
		RecordedBy: recordedBy,
	}

	if existing, err := repository.GetPriceRecord(ctx, s.Pool, ownerID, id); err == nil {
		if existing.OfferID != off.ID || existing.OwnerID != ownerID {
			return nil, ErrConflict
		}
		if repository.PricesLikelyEqual(existing, &pr) {
			return existing, nil
		}
		return nil, ErrConflict
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	if err := repository.InsertPriceRecord(ctx, s.Pool, pr); err != nil {
		if pgutil.IsUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return repository.GetPriceRecord(ctx, s.Pool, ownerID, id)
}

func (s *Service) ListPriceHistory(ctx context.Context, ownerID, offerID uuid.UUID) ([]models.PriceRecord, error) {
	if _, err := repository.GetOffer(ctx, s.Pool, ownerID, offerID); err != nil {
		return nil, err
	}
	return repository.ListPricesForOffer(ctx, s.Pool, ownerID, offerID, 2000)
}

// LatestPrice returns the newest price row for an offer.
// If the offer exists but has no prices yet, (nil, false, nil).
func (s *Service) LatestPrice(ctx context.Context, ownerID, offerID uuid.UUID) (*models.PriceRecord, bool, error) {
	if _, err := repository.GetOffer(ctx, s.Pool, ownerID, offerID); err != nil {
		return nil, false, err
	}
	pr, err := repository.LatestPriceForOffer(ctx, s.Pool, ownerID, offerID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return pr, true, nil
}

func (s *Service) ListPriceRecordsSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]models.PriceRecord, error) {
	return repository.ListPriceRecordsSince(ctx, s.Pool, ownerID, since)
}
