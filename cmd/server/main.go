package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exndiver/shopping-backend/internal/app"
	"github.com/exndiver/shopping-backend/internal/config"
	"github.com/exndiver/shopping-backend/internal/db"
	"github.com/exndiver/shopping-backend/internal/migrations"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := migrations.Up(cfg); err != nil {
			log.Fatal(err)
		}
	}

	srv := app.NewServer(cfg, pool)

	go func() {
		if err := srv.Start(); err != nil {
			log.Println(err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}
