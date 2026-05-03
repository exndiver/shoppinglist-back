package service

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) MergeGoods(ctx context.Context, ownerID, sourceGoodID, targetGoodID uuid.UUID) error {
	ca, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, sourceGoodID)
	if err != nil {
		return err
	}
	cb, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, targetGoodID)
	if err != nil {
		return err
	}
	if ca == cb {
		return nil
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := repository.MarkGoodMerged(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repointOffersAfterGoodMerge(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repository.RepointListItemsGood(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repository.DeleteIdentityConflictsForMerge(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repository.RepointGoodIdentities(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func repointOffersAfterGoodMerge(ctx context.Context, tx repository.DBTX, ownerID, fromGood, toGood uuid.UUID) error {
	offers, err := repository.ListOffersForGood(ctx, tx, ownerID, fromGood)
	if err != nil {
		return err
	}

	for _, o := range offers {
		existing, err := repository.FindOfferByTriple(ctx, tx, ownerID, toGood, o.StoreID)
		if err != nil && err != repository.ErrNotFound {
			return err
		}
		if err == repository.ErrNotFound {
			if err := repository.UpdateOfferGoodID(ctx, tx, ownerID, o.ID, toGood); err != nil {
				return err
			}
			continue
		}

		if existing.ID == o.ID {
			continue
		}

		if err := repository.RepointPriceRecords(ctx, tx, o.ID, existing.ID); err != nil {
			return err
		}
		if err := repository.DeleteOffer(ctx, tx, ownerID, o.ID); err != nil {
			return err
		}
	}

	return nil
}
