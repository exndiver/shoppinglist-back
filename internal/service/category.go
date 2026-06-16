package service

import (
	"context"
	"strings"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/normalize"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) UpsertCategory(ctx context.Context, ownerID, id uuid.UUID, name string) (*models.Category, error) {
	n := normalize.Name(name)
	if n == "" {
		return nil, ErrBadRequest
	}
	c := models.Category{
		ID:             id,
		OwnerID:        ownerID,
		Name:           strings.TrimSpace(name),
		NormalizedName: n,
	}
	out, err := repository.UpsertCategoryReturning(ctx, s.Pool, c)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) GetCategory(ctx context.Context, ownerID, id uuid.UUID) (*models.Category, error) {
	return repository.GetCategory(ctx, s.Pool, ownerID, id)
}

func (s *Service) SearchCategories(ctx context.Context, ownerID uuid.UUID, q string) ([]models.Category, error) {
	nq := normalize.Name(q)
	return repository.SearchCategories(ctx, s.Pool, ownerID, nq, 200)
}

func (s *Service) ListCategoriesSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]models.Category, error) {
	return repository.ListCategoriesSince(ctx, s.Pool, ownerID, since)
}
