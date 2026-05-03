package service

import (
	"context"
	"errors"
	"strings"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/normalize"
	"github.com/exndiver/shopping-backend/internal/pgutil"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) UpsertStore(ctx context.Context, ownerID, id uuid.UUID, name string, createdBy *string) (*models.Store, error) {
	n := normalize.Name(name)
	if n == "" {
		return nil, ErrBadRequest
	}

	_, err := repository.GetStore(ctx, s.Pool, ownerID, id)
	if errors.Is(err, repository.ErrNotFound) {
		st := models.Store{
			ID:             id,
			OwnerID:        ownerID,
			Name:           strings.TrimSpace(name),
			NormalizedName: n,
			CreatedBy:      createdBy,
		}
		if err := repository.InsertStore(ctx, s.Pool, st); err != nil {
			if pgutil.IsUniqueViolation(err) {
				return nil, ErrConflict
			}
			return nil, err
		}
		return repository.GetStore(ctx, s.Pool, ownerID, id)
	}
	if err != nil {
		return nil, err
	}

	canon, err := repository.ResolveStoreCanonical(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateStoreCanonical(ctx, s.Pool, ownerID, canon, strings.TrimSpace(name), n, createdBy); err != nil {
		return nil, err
	}
	return repository.GetStore(ctx, s.Pool, ownerID, canon)
}

func (s *Service) SearchStores(ctx context.Context, ownerID uuid.UUID, q string) ([]models.Store, error) {
	nq := normalize.Name(q)
	return repository.SearchCanonicalStores(ctx, s.Pool, ownerID, nq, 200)
}
