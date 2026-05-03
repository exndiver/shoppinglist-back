package service

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) MergeProducts(ctx context.Context, ownerID, sourceProductID, targetProductID uuid.UUID) error {
	ca, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, sourceProductID)
	if err != nil {
		return err
	}
	cb, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, targetProductID)
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

	if err := repository.MarkProductMerged(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repointOffersAfterProductMerge(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repository.RepointListItemsProduct(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repository.DeleteIdentityConflictsForMerge(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}
	if err := repository.RepointProductIdentities(ctx, tx, ownerID, ca, cb); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func repointOffersAfterProductMerge(ctx context.Context, tx repository.DBTX, ownerID, fromProd, toProd uuid.UUID) error {
	offers, err := repository.ListOffersForProduct(ctx, tx, ownerID, fromProd)
	if err != nil {
		return err
	}

	for _, o := range offers {
		existing, err := repository.FindOfferByTriple(ctx, tx, ownerID, toProd, o.StoreID)
		if err != nil && err != repository.ErrNotFound {
			return err
		}
		if err == repository.ErrNotFound {
			if err := repository.UpdateOfferProductID(ctx, tx, ownerID, o.ID, toProd); err != nil {
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
