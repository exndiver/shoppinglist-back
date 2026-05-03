package service

import (
	"errors"

	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = repository.ErrNotFound
	ErrConflict      = repository.ErrConflict
	ErrBadRequest    = errors.New("bad request")
	ErrDepthExceeded = repository.ErrDepthExceeded
)

type Service struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool}
}
