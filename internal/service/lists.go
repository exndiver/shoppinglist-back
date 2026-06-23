package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/pgutil"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

type ListItemDetail struct {
	models.ListItem
	GoodName string `json:"good_name"`
}

type ListDetail struct {
	List  models.ShoppingList `json:"list"`
	Items []ListItemDetail    `json:"items"`
}

func (s *Service) UpsertShoppingList(ctx context.Context, ownerID, id uuid.UUID, name string, createdBy *string) (*models.ShoppingList, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return nil, ErrBadRequest
	}

	_, err := repository.GetShoppingList(ctx, s.Pool, ownerID, id)
	if errors.Is(err, repository.ErrNotFound) {
		l := models.ShoppingList{
			ID:        id,
			OwnerID:   ownerID,
			Name:      n,
			CreatedBy: createdBy,
		}
		if err := repository.InsertShoppingList(ctx, s.Pool, l); err != nil {
			if pgutil.IsUniqueViolation(err) {
				return nil, ErrConflict
			}
			return nil, err
		}
		return repository.GetShoppingList(ctx, s.Pool, ownerID, id)
	}
	if err != nil {
		return nil, err
	}

	if err := repository.UpdateShoppingList(ctx, s.Pool, ownerID, id, n, createdBy); err != nil {
		return nil, err
	}
	return repository.GetShoppingList(ctx, s.Pool, ownerID, id)
}

func (s *Service) ListShoppingLists(ctx context.Context, ownerID uuid.UUID) ([]models.ShoppingList, error) {
	return repository.ListShoppingLists(ctx, s.Pool, ownerID, 200)
}

func (s *Service) GetListDetail(ctx context.Context, ownerID, listID uuid.UUID) (*ListDetail, error) {
	l, err := repository.GetShoppingList(ctx, s.Pool, ownerID, listID)
	if err != nil {
		return nil, err
	}

	items, err := repository.ListItemsByList(ctx, s.Pool, ownerID, listID)
	if err != nil {
		return nil, err
	}

	outItems := make([]ListItemDetail, 0, len(items))
	for _, it := range items {
		gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, it.GoodID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return nil, err
		}
		g, err := repository.GetGood(ctx, s.Pool, ownerID, gc)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return nil, err
		}
		it.GoodID = gc
		outItems = append(outItems, ListItemDetail{
			ListItem: it,
			GoodName: g.Name,
		})
	}

	return &ListDetail{List: *l, Items: outItems}, nil
}

func (s *Service) AddListItem(ctx context.Context, ownerID uuid.UUID, id, listID, goodID uuid.UUID, offerID *uuid.UUID, quantity float64, priceSnapshot *float64, createdBy *string) (*ListItemDetail, error) {
	if quantity <= 0 {
		quantity = 1
	}

	// The author or any edit-access member may add. The item lives in the list
	// owner's data scope (preserving the sl.owner_id = li.owner_id invariant the
	// owner's own sync relies on); created_by attributes the member who added it.
	access, ok, err := repository.AccessForOwner(ctx, s.Pool, listID, ownerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if access != models.ShareAccessEdit {
		return nil, ErrBadRequest
	}
	listOwnerID, err := repository.GetListOwner(ctx, s.Pool, listID)
	if err != nil {
		return nil, err
	}

	// Goods/offers are resolved in the caller's catalog — a member adds their
	// own good, which the owner imports via the foreign-goods sync path.
	gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, goodID)
	if err != nil {
		return nil, err
	}

	var resolvedOffer *uuid.UUID
	if offerID != nil {
		off, err := repository.GetOffer(ctx, s.Pool, ownerID, *offerID)
		if err != nil {
			return nil, err
		}
		og, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, off.GoodID)
		if err != nil {
			return nil, err
		}
		if og != gc {
			return nil, ErrBadRequest
		}
		resolvedOffer = offerID
	}

	it := models.ListItem{
		ID:            id,
		OwnerID:       listOwnerID,
		ListID:        listID,
		GoodID:        gc,
		OfferID:       resolvedOffer,
		Quantity:      quantity,
		PriceSnapshot: priceSnapshot,
		IsPurchased:   false,
		CreatedBy:     createdBy,
	}

	if err := repository.InsertListItem(ctx, s.Pool, it); err != nil {
		if pgutil.IsUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}

	got, err := repository.GetListItem(ctx, s.Pool, listOwnerID, id)
	if err != nil {
		return nil, err
	}
	got.GoodID = gc
	g, err := repository.GetGood(ctx, s.Pool, ownerID, gc)
	if err != nil {
		return nil, err
	}
	return &ListItemDetail{ListItem: *got, GoodName: g.Name}, nil
}

func (s *Service) PatchListItem(ctx context.Context, ownerID, itemID uuid.UUID, patch repository.ListItemPatch) error {
	it, err := repository.GetListItem(ctx, s.Pool, ownerID, itemID)
	if err != nil {
		return err
	}

	gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, it.GoodID)
	if err != nil {
		return err
	}

	if patch.OfferIDPresent {
		if patch.OfferID == nil {
			// clearing offer allowed without validation
		} else {
			off, err := repository.GetOffer(ctx, s.Pool, ownerID, *patch.OfferID)
			if err != nil {
				return err
			}
			og, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, off.GoodID)
			if err != nil {
				return err
			}
			if og != gc {
				return ErrBadRequest
			}
		}
	}

	return repository.PatchListItem(ctx, s.Pool, ownerID, itemID, patch)
}

func (s *Service) GetListItemDetail(ctx context.Context, ownerID, itemID uuid.UUID) (*ListItemDetail, error) {
	it, err := repository.GetListItem(ctx, s.Pool, ownerID, itemID)
	if err != nil {
		return nil, err
	}
	gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, it.GoodID)
	if err != nil {
		return nil, err
	}
	it.GoodID = gc
	g, err := repository.GetGood(ctx, s.Pool, ownerID, gc)
	if err != nil {
		return nil, err
	}
	return &ListItemDetail{ListItem: *it, GoodName: g.Name}, nil
}

func (s *Service) ListListsSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]models.ShoppingList, error) {
	return repository.ListListsSince(ctx, s.Pool, ownerID, since)
}

func (s *Service) ListListItemsSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]models.ListItem, error) {
	items, err := repository.ListListItemsSince(ctx, s.Pool, ownerID, since)
	if err != nil {
		return nil, err
	}
	out := make([]models.ListItem, 0, len(items))
	for _, it := range items {
		gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, it.GoodID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return nil, err
		}
		it.GoodID = gc
		out = append(out, it)
	}
	return out, nil
}
