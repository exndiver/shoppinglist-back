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
	// Shared-list participants are equals: the good and the store may belong to
	// any participant (a member pricing a good the owner added, at a store
	// somebody else created), so resolve them owner-agnostically — same rule
	// AddListItem already uses. The offer row itself is still owned by the
	// caller, so each participant keeps their own price history and the app
	// surfaces the newest.
	gc, err := repository.ResolveGoodCanonicalAny(ctx, s.Pool, goodID)
	if err != nil {
		return nil, err
	}
	sc, err := repository.ResolveStoreCanonicalAny(ctx, s.Pool, storeID)
	if err != nil {
		return nil, err
	}

	if existing, err := repository.GetOffer(ctx, s.Pool, ownerID, id); err == nil {
		eg, err := repository.ResolveGoodCanonicalAny(ctx, s.Pool, existing.GoodID)
		if err != nil {
			return nil, err
		}
		es, err := repository.ResolveStoreCanonicalAny(ctx, s.Pool, existing.StoreID)
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
		if isUniqueViolation(err) {
			// Two different constraints can land here. First: the id already
			// exists under ANOTHER participant — shared-list members reuse
			// whichever offer they hold for a good+store pair, so they push back
			// the owner's offer id. offers.id is a global primary key while the
			// probe above is owner-scoped, so it never saw that row. If it
			// already describes the same pair the caller's intent is satisfied;
			// return it so both participants share one offer and simply attach
			// their own price records to it.
			if shared, aerr := repository.GetOfferAny(ctx, s.Pool, id); aerr == nil {
				ag, gerr := repository.ResolveGoodCanonicalAny(ctx, s.Pool, shared.GoodID)
				as, serr := repository.ResolveStoreCanonicalAny(ctx, s.Pool, shared.StoreID)
				if gerr == nil && serr == nil && ag == gc && as == sc {
					return shared, nil
				}
				return nil, ErrConflict
			}
			// Second: unique violation on (owner_id, good_id, store_id) — one of
			// our own offers already covers the same pair under a different id.
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
		// Participants of a shared list are equals, so an offer may legitimately
		// point at a good or store owned by somebody else. Resolve
		// owner-agnostically to match the write path (UpsertOffer) — resolving
		// within the caller made every such offer unreadable.
		//
		// A row that still cannot be resolved keeps its raw id instead of
		// failing the call: this feeds /sync/batch, and one dangling reference
		// must never take down a client's entire sync.
		gc, err := repository.ResolveGoodCanonicalAny(ctx, s.Pool, items[i].GoodID)
		if err == nil {
			items[i].GoodID = gc
		} else if !isNotFound(err) {
			return nil, err
		}
		sc, err := repository.ResolveStoreCanonicalAny(ctx, s.Pool, items[i].StoreID)
		if err == nil {
			items[i].StoreID = sc
		} else if !isNotFound(err) {
			return nil, err
		}
	}
	return items, nil
}
