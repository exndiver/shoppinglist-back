package db

import (
	"context"
	"fmt"
	"time"

	"github.com/exndiver/shopping-backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	connCfg, err := pgxpool.ParseConfig(cfg.PostgresURL())
	if err != nil {
		return nil, fmt.Errorf("parse postgres url: %w", err)
	}

	// Консервативные дефолты для старта; дальше настроим под нагрузку.
	connCfg.MaxConns = 10
	connCfg.MinConns = 0
	connCfg.MaxConnLifetime = 30 * time.Minute
	connCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, connCfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
