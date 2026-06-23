package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/exndiver/shopping-backend/internal/config"
	"github.com/exndiver/shopping-backend/internal/handlers"
	"github.com/exndiver/shopping-backend/internal/metrics"
	"github.com/exndiver/shopping-backend/internal/middleware"
	"github.com/exndiver/shopping-backend/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BuildVersion is a marker to confirm which code a deploy is actually running.
// Bump it whenever you need to verify a deploy landed; can be overridden at
// build time with -ldflags "-X .../internal/app.BuildVersion=<sha>".
var BuildVersion = "svc-goodfilter-fix"

type Server struct {
	cfg  config.Config
	db   *pgxpool.Pool
	http *http.Server
}

func NewServer(cfg config.Config, pool *pgxpool.Pool) *Server {
	mux := http.NewServeMux()

	health := handlers.NewHealthHandler(pool)
	mux.HandleFunc("GET /health", health.Get)

	// Build marker so a deploy can be verified to actually be running this code.
	// Bump on each change that needs confirming live; overridable via -ldflags.
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(BuildVersion + "\n"))
	})

	if cfg.MetricsEnabled {
		mux.Handle(cfg.MetricsPath, metrics.Handler())
	}

	svc := service.New(pool)
	api := handlers.NewAPI(svc)
	mux.Handle("/", api)

	// Middleware order (наружу внутрь):
	//   RequestID → Recover → Metrics → RateLimit → BearerOwner → MaxBody → AccessLog → mux
	// Owner кладётся в context до AccessLog, чтобы enduser.id попадал в лог.
	slow := cfg.LogSlowRequest
	var h http.Handler = mux
	h = middleware.AccessLog(slow, h)
	h = middleware.MaxBody(cfg.HTTPMaxBodyBytes, h)
	h = middleware.BearerOwner(h)
	if cfg.RateLimit > 0 {
		h = middleware.RateLimit(cfg.RateLimit, cfg.RateLimitBurst, h)
	}
	if cfg.MetricsEnabled {
		h = metrics.Middleware(h)
	}
	h = middleware.Recover(h)
	h = middleware.RequestID(h)

	s := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
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
