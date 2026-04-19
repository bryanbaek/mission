package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanbaek/mission/gen/go/tenant/v1/tenantv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/config"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/db"
	"github.com/bryanbaek/mission/internal/controlplane/handler"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

func main() {
	if err := run(); err != nil {
		slog.Error("control-plane exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	// Repositories
	tenantRepo := repository.NewTenantRepository(pool)
	tokenRepo := repository.NewTenantTokenRepository(pool)

	// Controllers
	tenantCtrl := controller.NewTenantController(tenantRepo, tokenRepo)

	// Auth
	var verifier auth.Verifier
	if cfg.ClerkSecretKey != "" {
		verifier = auth.NewClerkVerifier(cfg.ClerkSecretKey)
	} else {
		slog.Warn("CLERK_SECRET_KEY not set — auth disabled (dev mode only)")
		verifier = &auth.FakeVerifier{Tokens: map[string]auth.User{
			"dev-token": {ID: "dev_user_001"},
		}}
	}

	// Handlers
	healthHandler := handler.NewHealthHandler(pool)
	tenantHandler := handler.NewTenantHandler(tenantCtrl)
	tenantPath, tenantSvc := tenantv1connect.NewTenantServiceHandler(tenantHandler)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Public
	r.Get("/healthz", healthHandler.Healthz)

	// Authenticated (Connect-RPC)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(verifier))
		r.Mount(tenantPath, tenantSvc)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("control-plane listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("control-plane stopped")
	return nil
}
