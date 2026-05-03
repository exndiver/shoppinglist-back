package service

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
)

func (s *Service) UpsertGoodIdentity(ctx context.Context, ownerID, goodID uuid.UUID, externalID, source string) error {
	canon, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, goodID)
	if err != nil {
		return err
	}
	return repository.UpsertGoodIdentity(ctx, s.Pool, ownerID, canon, externalID, source)
}
