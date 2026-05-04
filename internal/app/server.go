package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/exndiver/shopping-backend/internal/config"
	"github.com/exndiver/shopping-backend/internal/handlers"
	"github.com/exndiver/shopping-backend/internal/middleware"
	"github.com/exndiver/shopping-backend/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg  config.Config
	db   *pgxpool.Pool
	http *http.Server
}

func NewServer(cfg config.Config, pool *pgxpool.Pool) *Server {
	mux := http.NewServeMux()

	health := handlers.NewHealthHandler(pool)
	mux.HandleFunc("GET /health", health.Get)

	svc := service.New(pool)
	api := handlers.NewAPI(svc)
	mux.Handle("/", middleware.BearerOwner(api))

	slow := cfg.LogSlowRequest
	var h http.Handler = mux
	h = middleware.AccessLog(slow, h)
	h = middleware.Recover(h)
	h = middleware.RequestID(h)

	s := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{
		cfg:  cfg,
		db:   pool,
		http: s,
	}
}

func (s *Server) Start() error {
	slog.Info("http.server.listen", slog.String("addr", s.http.Addr))

	err := s.http.ListenAndServe()
	if err == nil || err == http.ErrServerClosed {
		return nil
	}
	return fmt.Errorf("http server: %w", err)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
