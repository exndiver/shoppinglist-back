package service

import (
	"context"
	"errors"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/pgutil"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) CreateOffer(ctx context.Context, ownerID uuid.UUID, id, productID, storeID uuid.UUID, createdBy *string) (*models.Offer, error) {
	pc, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, productID)
	if err != nil {
		return nil, err
	}
	sc, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, storeID)
	if err != nil {
		return nil, err
	}

	if existing, err := repository.GetOffer(ctx, s.Pool, ownerID, id); err == nil {
		ep, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, existing.ProductID)
		if err != nil {
			return nil, err
		}
		es, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, existing.StoreID)
		if err != nil {
			return nil, err
		}
		if ep == pc && es == sc {
			return existing, nil
		}
		return nil, ErrConflict
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	o := models.Offer{
		ID:        id,
		OwnerID:   ownerID,
		ProductID: pc,
		StoreID:   sc,
		CreatedBy: createdBy,
	}
	if err := repository.InsertOffer(ctx, s.Pool, o); err != nil {
		if pgutil.IsUniqueViolation(err) {
			triple, terr := repository.FindOfferByTriple(ctx, s.Pool, ownerID, pc, sc)
			if terr != nil {
				return nil, terr
			}
			if triple.ID == id {
				return triple, nil
			}
			return nil, ErrConflict
		}
		return nil, err
	}
	return repository.GetOffer(ctx, s.Pool, ownerID, id)
}

func (s *Service) ListOffersWithLatestPrices(ctx context.Context, ownerID, productID uuid.UUID) ([]models.OfferWithLatestPrice, error) {
	pc, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, productID)
	if err != nil {
		return nil, err
	}

	offers, err := repository.ListOffersForProduct(ctx, s.Pool, ownerID, pc)
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
		} else if errors.Is(err, repository.ErrNotFound) {
			item.Latest = nil
		} else {
			return nil, err
		}

		out = append(out, item)
	}

	return out, nil
}
