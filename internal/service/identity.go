package service

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) UpsertProductIdentity(ctx context.Context, ownerID, productID uuid.UUID, externalID, source string) error {
	canon, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, productID)
	if err != nil {
		return err
	}
	return repository.UpsertProductIdentity(ctx, s.Pool, ownerID, canon, externalID, source)
}
