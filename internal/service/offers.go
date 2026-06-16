package service

import (
	"context"
	"errors"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) UpsertOffer(ctx context.Context, ownerID uuid.UUID, id, goodID, storeID uuid.UUID, createdBy *string) (*models.Offer, error) {
	gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, goodID)
	if err != nil {
		return nil, err
	}
	sc, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, storeID)
	if err != nil {
		return nil, err
	}

	if existing, err := repository.GetOffer(ctx, s.Pool, ownerID, id); err == nil {
		eg, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, existing.GoodID)
		if err != nil {
			return nil, err
		}
		es, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, existing.StoreID)
		if err != nil {
			return nil, err
		}
		if eg == gc && es == sc {
			if err := repository.TouchOfferUpdatedAt(ctx, s.Pool, ownerID, id); err != nil {
				return nil, err
			}
			return repository.GetOffer(ctx, s.Pool, ownerID, id)
		}
		return nil, ErrConflict
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	o := models.Offer{
		ID:        id,
		OwnerID:   ownerID,
		GoodID:    gc,
		StoreID:   sc,
		CreatedBy: createdBy,
	}
	out, err := repository.InsertOfferReturning(ctx, s.Pool, o)
	if err != nil {
		// unique violation on (owner_id, good_id, store_id) — another offer for same triple
		if isUniqueViolation(err) {
			triple, terr := repository.FindOfferByTriple(ctx, s.Pool, ownerID, gc, sc)
			if terr != nil {
				return nil, terr
			}
			if triple.ID == id {
				if err := repository.TouchOfferUpdatedAt(ctx, s.Pool, ownerID, id); err != nil {
					return nil, err
				}
				return repository.GetOffer(ctx, s.Pool, ownerID, id)
			}
			return nil, ErrConflict
		}
		return nil, err
	}
	return out, nil
}

func (s *Service) ListOffersWithLatestPrices(ctx context.Context, ownerID, goodID uuid.UUID) ([]models.OfferWithLatestPrice, error) {
	gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, goodID)
	if err != nil {
		return nil, err
	}

	offers, err := repository.ListOffersForGood(ctx, s.Pool, ownerID, gc)
	if err != nil {
		return nil, err
	}

	out := make([]models.OfferWithLatestPrice, 0, len(offers))
	for _, o := range offers {
		storeCanon, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, o.StoreID)
		if err != nil {
			return nil, err
		}
		st, err := repository.GetStore(ctx, s.Pool, ownerID, storeCanon)
		if err != nil {
			return nil, err
		}

		item := models.OfferWithLatestPrice{
			OfferID: o.ID,
			Store:   *st,
		}

		if lp, err := repository.LatestPriceForOffer(ctx, s.Pool, ownerID, o.ID); err == nil {
			item.Latest = &models.PriceSnapshot{
				Price:    lp.Price,
				PackSize: lp.PackSize,
				Unit:     lp.Unit,
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}

		out = append(out, item)
	}

	return out, nil
}

func (s *Service) ListOffersSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]models.Offer, error) {
	items, err := repository.ListOffersSince(ctx, s.Pool, ownerID, since)
	if err != nil {
		return nil, err
	}
	for i := range items {
		// Skip canonical resolution for soft-deleted offers — the good/store may no longer be active
		if items[i].DeletedAt != nil {
			continue
		}
		gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, items[i].GoodID)
		if err != nil {
			return nil, err
		}
		items[i].GoodID = gc
		sc, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, items[i].StoreID)
		if err != nil {
			return nil, err
		}
		items[i].StoreID = sc
	}
	return items, nil
}
