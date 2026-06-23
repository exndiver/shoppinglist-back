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

func (s *Service) PatchListItem(ctx context.Context, callerID, itemID uuid.UUID, patch repository.ListItemPatch) error {
	// Resolve the item by id, then authorize the caller against its list so an
	// edit-access member (not just the owner) may toggle/quantity-edit. The item
	// lives in the list owner's scope, so the write is scoped to it.OwnerID.
	it, err := repository.GetListItemByID(ctx, s.Pool, itemID)
	if err != nil {
		return err
	}
	access, ok, err := repository.AccessForOwner(ctx, s.Pool, it.ListID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if access != models.ShareAccessEdit {
		return ErrBadRequest
	}

	// Offer pinning is validated within the item's owner scope. Clearing the
	// offer needs no validation.
	if patch.OfferIDPresent && patch.OfferID != nil {
		gc, err := repository.ResolveGoodCanonical(ctx, s.Pool, it.OwnerID, it.GoodID)
		if err != nil {
			return err
		}
		off, err := repository.GetOffer(ctx, s.Pool, it.OwnerID, *patch.OfferID)
		if err != nil {
			return err
		}
		og, err := repository.ResolveGoodCanonical(ctx, s.Pool, it.OwnerID, off.GoodID)
		if err != nil {
			return err
		}
		if og != gc {
			return ErrBadRequest
		}
	}

	return repository.PatchListItem(ctx, s.Pool, it.OwnerID, itemID, patch)
}

// DeleteListItem tombstones an item if the caller is the list author or an
// edit-access member, so the deletion propagates to every participant.
func (s *Service) DeleteListItem(ctx context.Context, callerID, itemID uuid.UUID) error {
	it, err := repository.GetListItemByID(ctx, s.Pool, itemID)
	if err != nil {
		return err
	}
	access, ok, err := repository.AccessForOwner(ctx, s.Pool, it.ListID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if access != models.ShareAccessEdit {
		return ErrBadRequest
	}
	return repository.SoftDeleteListItem(ctx, s.Pool, it.OwnerID, itemID)
}

// DeletedListItemIDsSince returns the ids of items tombstoned since `since` that
// the caller should drop locally — both on their own lists and on lists shared
// to them — deduplicated.
func (s *Service) DeletedListItemIDsSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	own, err := repository.ListDeletedListItemIDsForOwnerSince(ctx, s.Pool, ownerID, since)
	if err != nil {
		return nil, err
	}
	shared, err := repository.ListDeletedSharedListItemIDsForMemberSince(ctx, s.Pool, ownerID, since)
	if err != nil {
		return nil, err
	}
	seen := make(map[uuid.UUID]struct{}, len(own)+len(shared))
	out := make([]uuid.UUID, 0, len(own)+len(shared))
	for _, id := range own {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	for _, id := range shared {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *Service) GetListItemDetail(ctx context.Context, callerID, itemID uuid.UUID) (*ListItemDetail, error) {
	it, err := repository.GetListItemByID(ctx, s.Pool, itemID)
	if err != nil {
		return nil, err
	}
	if _, ok, err := repository.AccessForOwner(ctx, s.Pool, it.ListID, callerID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	// The good may belong to a collaborator (different scope), so look it up by
	// id and degrade gracefully if it can't be found — the name is cosmetic.
	goodName := ""
	if g, err := repository.GetGoodAny(ctx, s.Pool, it.GoodID); err == nil {
		it.GoodID = g.ID
		goodName = g.Name
	}
	return &ListItemDetail{ListItem: *it, GoodName: goodName}, nil
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
