package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exndiver/shopping-backend/internal/app"
	"github.com/exndiver/shopping-backend/internal/config"
	"github.com/exndiver/shopping-backend/internal/db"
	"github.com/exndiver/shopping-backend/internal/logging"
	"github.com/exndiver/shopping-backend/internal/migrations"
)

func main() {
	defer func() { _ = logging.Close() }()

	started := time.Now().UTC()
	cfg := config.Load()
	logging.Init(cfg)
	slog.LogAttrs(context.Background(), slog.LevelInfo, "process.start", logging.Startup(started)...)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		slog.Error("db.connect", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := migrations.Up(cfg); err != nil {
			slog.Error("migrations.up", slog.Any("error", err))
			os.Exit(1)
		}
	}

	srv := app.NewServer(cfg, pool)
	app.StartTombstonePruner(ctx, pool)

	go func() {
		if err := srv.Start(); err != nil {
			slog.Error("http.server", slog.Any("error", err))
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("process.shutdown_signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http.server.shutdown", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("process.exit_ok")
}
