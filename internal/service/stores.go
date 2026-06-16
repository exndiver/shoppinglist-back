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

func (s *Service) UpsertStore(ctx context.Context, ownerID, id uuid.UUID, name string, createdBy *string) (*models.Store, error) {
	n := normalize.Name(name)
	if n == "" {
		return nil, ErrBadRequest
	}

	_, err := repository.GetStore(ctx, s.Pool, ownerID, id)
	if isNotFound(err) {
		st := models.Store{
			ID:             id,
			OwnerID:        ownerID,
			Name:           strings.TrimSpace(name),
			NormalizedName: n,
			CreatedBy:      createdBy,
		}
		return repository.InsertStoreReturning(ctx, s.Pool, st)
	}
	if err != nil {
		return nil, err
	}

	canon, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	return repository.UpdateStoreCanonicalReturning(ctx, s.Pool, ownerID, canon, strings.TrimSpace(name), n, createdBy)
}

func (s *Service) SearchStores(ctx context.Context, ownerID uuid.UUID, q string) ([]models.Store, error) {
	nq := normalize.Name(q)
	return repository.SearchCanonicalStores(ctx, s.Pool, ownerID, nq, 200)
}

func (s *Service) ListStoresSince(ctx context.Context, ownerID uuid.UUID, since time.Time) ([]models.Store, error) {
	return repository.ListStoresSince(ctx, s.Pool, ownerID, since)
}
