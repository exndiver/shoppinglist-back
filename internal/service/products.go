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
	Exact    []models.Product
	Prefix   []models.Product
	Contains []models.Product
	Others   []models.Product
}

func (s *Service) UpsertProduct(ctx context.Context, ownerID, id uuid.UUID, name string, createdBy *string) (*models.Product, error) {
	n := normalize.Name(name)
	if n == "" {
		return nil, ErrBadRequest
	}

	_, err := repository.GetProduct(ctx, s.Pool, ownerID, id)
	if errors.Is(err, repository.ErrNotFound) {
		p := models.Product{
			ID:             id,
			OwnerID:        ownerID,
			Name:           strings.TrimSpace(name),
			NormalizedName: n,
			CreatedBy:      createdBy,
		}
		if err := repository.InsertProduct(ctx, s.Pool, p); err != nil {
			if pgutil.IsUniqueViolation(err) {
				return nil, ErrConflict
			}
			return nil, err
		}
		return repository.GetProduct(ctx, s.Pool, ownerID, id)
	}
	if err != nil {
		return nil, err
	}

	canon, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateProductCanonical(ctx, s.Pool, ownerID, canon, strings.TrimSpace(name), n, createdBy); err != nil {
		return nil, err
	}
	return repository.GetProduct(ctx, s.Pool, ownerID, canon)
}

func (s *Service) GetProductCanonical(ctx context.Context, ownerID, id uuid.UUID) (*models.Product, error) {
	canon, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, id)
	if err != nil {
		return nil, err
	}
	return repository.GetProduct(ctx, s.Pool, ownerID, canon)
}

func (s *Service) SearchProducts(ctx context.Context, ownerID uuid.UUID, q string) ([]models.Product, error) {
	nq := normalize.Name(q)
	return repository.SearchCanonicalProducts(ctx, s.Pool, ownerID, nq, 200)
}

func (s *Service) MergeCandidates(ctx context.Context, ownerID uuid.UUID, productID uuid.UUID, q string) (*MergeBuckets, error) {
	excludeCanon, err := repository.ResolveProductCanonical(ctx, s.Pool, ownerID, productID)
	if err != nil {
		return nil, err
	}

	nq := normalize.Name(q)
	out := &MergeBuckets{
		Exact:    []models.Product{},
		Prefix:   []models.Product{},
		Contains: []models.Product{},
		Others:   []models.Product{},
	}

	add := func(dst *[]models.Product, p models.Product) {
		if p.ID == excludeCanon {
			return
		}
		for _, x := range *dst {
			if x.ID == p.ID {
				return
			}
		}
		*dst = append(*dst, p)
	}

	if nq == "" {
		rest, err := repository.ListCanonicalProductsExclude(ctx, s.Pool, ownerID, excludeCanon, 50)
		if err != nil {
			return nil, err
		}
		out.Others = rest
		return out, nil
	}

	all, err := repository.SearchCanonicalProducts(ctx, s.Pool, ownerID, nq, 300)
	if err != nil {
		return nil, err
	}

	for _, p := range all {
		if p.ID == excludeCanon {
			continue
		}
		switch {
		case p.NormalizedName == nq:
			add(&out.Exact, p)
		case strings.HasPrefix(p.NormalizedName, nq) && p.NormalizedName != nq:
			add(&out.Prefix, p)
		case strings.Contains(p.NormalizedName, nq):
			add(&out.Contains, p)
		}
	}

	others, err := repository.ListCanonicalProductsExclude(ctx, s.Pool, ownerID, excludeCanon, 80)
	if err != nil {
		return nil, err
	}
	for _, p := range others {
		if strings.Contains(p.NormalizedName, nq) {
			continue
		}
		add(&out.Others, p)
		if len(out.Others) >= 30 {
			break
		}
	}

	return out, nil
}
