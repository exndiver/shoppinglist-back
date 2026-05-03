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

type MergeBuckets struct {
	Exact    []models.Good
	Prefix   []models.Good
	Contains []models.Good
	Others   []models.Good
}

func (s *Service) UpsertGood(ctx context.Context, ownerID, id uuid.UUID, name string, createdBy *string) (*models.Good, error) {
	n := normalize.Name(name)
	if n == "" {
		return nil, ErrBadRequest
	}

	_, err := repository.GetGood(ctx, s.Pool, ownerID, id)
	if errors.Is(err, repository.ErrNotFound) {
		g := models.Good{
			ID:             id,
			OwnerID:        ownerID,
			Name:           strings.TrimSpace(name),
			NormalizedName: n,
			CreatedBy:      createdBy,
		}
		if err := repository.InsertGood(ctx, s.Pool, g); err != nil {
			if pgutil.IsUniqueViolation(err) {
				return nil, ErrConflict
			}
			return nil, err
		}
		return repository.GetGood(ctx, s.Pool, ownerID, id)
	}
	if err != nil {
		return nil, err
	}

	canon, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateGoodCanonical(ctx, s.Pool, ownerID, canon, strings.TrimSpace(name), n, createdBy); err != nil {
		return nil, err
	}
	return repository.GetGood(ctx, s.Pool, ownerID, canon)
}

func (s *Service) GetGoodCanonical(ctx context.Context, ownerID, id uuid.UUID) (*models.Good, error) {
	canon, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	return repository.GetGood(ctx, s.Pool, ownerID, canon)
}

func (s *Service) SearchGoods(ctx context.Context, ownerID uuid.UUID, q string) ([]models.Good, error) {
	nq := normalize.Name(q)
	return repository.SearchCanonicalGoods(ctx, s.Pool, ownerID, nq, 200)
}

func (s *Service) MergeCandidates(ctx context.Context, ownerID uuid.UUID, goodID uuid.UUID, q string) (*MergeBuckets, error) {
	excludeCanon, err := repository.ResolveGoodCanonical(ctx, s.Pool, ownerID, goodID)
	if err != nil {
		return nil, err
	}

	nq := normalize.Name(q)
	out := &MergeBuckets{
		Exact:    []models.Good{},
		Prefix:   []models.Good{},
		Contains: []models.Good{},
		Others:   []models.Good{},
	}

	add := func(dst *[]models.Good, g models.Good) {
		if g.ID == excludeCanon {
			return
		}
		for _, x := range *dst {
			if x.ID == g.ID {
				return
			}
		}
		*dst = append(*dst, g)
	}

	if nq == "" {
		rest, err := repository.ListCanonicalGoodsExclude(ctx, s.Pool, ownerID, excludeCanon, 50)
		if err != nil {
			return nil, err
		}
		out.Others = rest
		return out, nil
	}

	all, err := repository.SearchCanonicalGoods(ctx, s.Pool, ownerID, nq, 300)
	if err != nil {
		return nil, err
	}

	for _, g := range all {
		if g.ID == excludeCanon {
			continue
		}
		switch {
		case g.NormalizedName == nq:
			add(&out.Exact, g)
		case strings.HasPrefix(g.NormalizedName, nq) && g.NormalizedName != nq:
			add(&out.Prefix, g)
		case strings.Contains(g.NormalizedName, nq):
			add(&out.Contains, g)
		}
	}

	others, err := repository.ListCanonicalGoodsExclude(ctx, s.Pool, ownerID, excludeCanon, 80)
	if err != nil {
		return nil, err
	}
	for _, g := range others {
		if strings.Contains(g.NormalizedName, nq) {
			continue
		}
		add(&out.Others, g)
		if len(out.Others) >= 30 {
			break
		}
	}

	return out, nil
}
