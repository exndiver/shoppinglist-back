package service

import (
	"context"
	"errors"
	"strings"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/pgutil"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

type ListItemDetail struct {
	models.ListItem
	ProductName string `json:"product_name"`
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
		pc, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, it.ProductID)
		if err != nil {
			return nil, err
		}
		p, err := repository.GetProduct(ctx, s.Pool, ownerID, pc)
		if err != nil {
			return nil, err
		}
		outItems = append(outItems, ListItemDetail{
			ListItem:    it,
			ProductName: p.Name,
		})
	}

	return &ListDetail{List: *l, Items: outItems}, nil
}

func (s *Service) AddListItem(ctx context.Context, ownerID uuid.UUID, id, listID, productID uuid.UUID, offerID *uuid.UUID, quantity float64, priceSnapshot *float64, createdBy *string) (*ListItemDetail, error) {
	if quantity <= 0 {
		quantity = 1
	}

	if _, err := repository.GetShoppingList(ctx, s.Pool, ownerID, listID); err != nil {
		return nil, err
	}

	pc, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, productID)
	if err != nil {
		return nil, err
	}

	var resolvedOffer *uuid.UUID
	if offerID != nil {
		off, err := repository.GetOffer(ctx, s.Pool, ownerID, *offerID)
		if err != nil {
			return nil, err
		}
		op, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, off.ProductID)
		if err != nil {
			return nil, err
		}
		if op != pc {
			return nil, ErrBadRequest
		}
		resolvedOffer = offerID
	}

	it := models.ListItem{
		ID:            id,
		OwnerID:       ownerID,
		ListID:        listID,
		ProductID:     pc,
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

	got, err := repository.GetListItem(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	p, err := repository.GetProduct(ctx, s.Pool, ownerID, pc)
	if err != nil {
		return nil, err
	}
	return &ListItemDetail{ListItem: *got, ProductName: p.Name}, nil
}

func (s *Service) PatchListItem(ctx context.Context, ownerID, itemID uuid.UUID, patch repository.ListItemPatch) error {
	it, err := repository.GetListItem(ctx, s.Pool, ownerID, itemID)
	if err != nil {
		return err
	}

	pc, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, it.ProductID)
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
			op, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, off.ProductID)
			if err != nil {
				return err
			}
			if op != pc {
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
	pc, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, it.ProductID)
	if err != nil {
		return nil, err
	}
	p, err := repository.GetProduct(ctx, s.Pool, ownerID, pc)
	if err != nil {
		return nil, err
	}
	return &ListItemDetail{ListItem: *it, ProductName: p.Name}, nil
}
