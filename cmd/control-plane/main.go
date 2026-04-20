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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/gen/go/onboarding/v1/onboardingv1connect"
	"github.com/bryanbaek/mission/gen/go/query/v1/queryv1connect"
	"github.com/bryanbaek/mission/gen/go/semantic/v1/semanticv1connect"
	"github.com/bryanbaek/mission/gen/go/tenant/v1/tenantv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/config"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/db"
	llmgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	anthropicgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm/anthropic"
	openaigateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm/openai"
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
	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelInfo},
		),
	)
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
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
	schemaRepo := repository.NewTenantSchemaRepository(pool)
	semanticLayerRepo := repository.NewTenantSemanticLayerRepository(pool)
	onboardingRepo := repository.NewOnboardingRepository(pool)
	inviteRepo := repository.NewInviteRepository(pool)

	// Controllers
	tenantCtrl := controller.NewTenantController(tenantRepo, tokenRepo)
	agentSessions := controller.NewAgentSessionManager(
		controller.AgentSessionManagerConfig{
			StaleAfter:  25 * time.Second,
			PingTimeout: 5 * time.Second,
		},
	)
	schemaCtrl := controller.NewSchemaController(
		schemaRepo,
		agentSessions,
		controller.SchemaControllerConfig{},
	)
	llmProviders := make([]llmgateway.Provider, 0, 2)
	if cfg.AnthropicAPIKey != "" {
		llmProviders = append(
			llmProviders,
			anthropicgateway.New(anthropicgateway.Config{
				APIKey: cfg.AnthropicAPIKey,
			}),
		)
	}
	if cfg.OpenAIAPIKey != "" {
		llmProviders = append(
			llmProviders,
			openaigateway.New(openaigateway.Config{
				APIKey: cfg.OpenAIAPIKey,
			}),
		)
	}
	llmRouter := llmgateway.NewRouter(cfg.DefaultLLMProvider, llmProviders...)
	semanticLayerCtrl := controller.NewSemanticLayerController(
		tenantCtrl,
		schemaRepo,
		semanticLayerRepo,
		llmRouter,
		controller.SemanticLayerControllerConfig{
			Model: cfg.SemanticLayerModel,
		},
	)
	queryCtrl := controller.NewQueryController(
		tenantCtrl,
		schemaRepo,
		semanticLayerRepo,
		agentSessions,
		llmRouter,
		controller.QueryControllerConfig{
			Model: cfg.QueryModel,
		},
	)
	onboardingCtrl := controller.NewOnboardingController(
		onboardingRepo,
		inviteRepo,
		tenantCtrl,
		agentSessions,
		schemaCtrl,
		semanticLayerRepo,
		controller.OnboardingControllerConfig{
			EdgeAgentImage: cfg.EdgeAgentImage,
		},
	)

	// Auth
	var verifier auth.Verifier
	if cfg.ClerkSecretKey != "" {
		verifier = auth.NewClerkVerifier(cfg.ClerkSecretKey)
	} else {
		slog.Warn(
			"CLERK_SECRET_KEY not set — auth disabled (dev mode only)",
		)
		verifier = &auth.FakeVerifier{Tokens: map[string]auth.User{
			"dev-token": {ID: "dev_user_001"},
		}}
	}

	// Handlers
	healthHandler := handler.NewHealthHandler(pool)
	tenantHandler := handler.NewTenantHandler(tenantCtrl)
	agentHandler := handler.NewAgentHandler(agentSessions)
	debugAgentHandler := handler.NewAgentDebugHandler(agentSessions)
	queryDebugHandler := handler.NewQueryDebugHandler(tenantCtrl, agentSessions)
	schemaDebugHandler := handler.NewSchemaDebugHandler(tenantCtrl, schemaCtrl)
	semanticLayerHandler := handler.NewSemanticLayerHandler(semanticLayerCtrl)
	queryHandler := handler.NewQueryHandler(queryCtrl)
	onboardingHandler := handler.NewOnboardingHandler(onboardingCtrl)
	tenantPath, tenantSvc := tenantv1connect.NewTenantServiceHandler(tenantHandler)
	semanticPath, semanticSvc := semanticv1connect.NewSemanticLayerServiceHandler(
		semanticLayerHandler,
	)
	queryPath, querySvc := queryv1connect.NewQueryServiceHandler(queryHandler)
	onboardingPath, onboardingSvc := onboardingv1connect.NewOnboardingServiceHandler(
		onboardingHandler,
	)
	agentPath, agentSvc := agentv1connect.NewAgentServiceHandler(agentHandler)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Public
	r.With(middleware.Timeout(30*time.Second)).
		Get("/healthz", healthHandler.Healthz)

	// Authenticated (Connect-RPC)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Use(auth.RequireAuth(verifier))
		r.Mount(tenantPath, tenantSvc)
		r.Mount(semanticPath, semanticSvc)
	})

	// Query service gets a longer timeout because the pipeline may make two
	// LLM calls (generation + summarization) plus an edge-agent round trip.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(90 * time.Second))
		r.Use(auth.RequireAuth(verifier))
		r.Mount(queryPath, querySvc)
	})

	// Onboarding can block on edge-agent connectivity and schema capture.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(90 * time.Second))
		r.Use(auth.RequireAuth(verifier))
		r.Mount(onboardingPath, onboardingSvc)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(70 * time.Second))
		r.Use(auth.RequireAuth(verifier))
		r.Get("/api/debug/tenants/{tenantID}/query", queryDebugHandler.GetStatus)
		r.Post("/api/debug/tenants/{tenantID}/query", queryDebugHandler.ExecuteQuery)
		r.Post(
			"/api/debug/tenants/{tenantID}/schema/introspect",
			schemaDebugHandler.Introspect,
		)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAgentToken(tokenRepo))
		r.Mount(agentPath, agentSvc)
	})

	if cfg.Env != "production" {
		r.Get("/api/debug/agents", debugAgentHandler.ListSessions)
		r.Post("/api/debug/agents/{sessionID}/ping", debugAgentHandler.PingSession)
	}

	// h2c lets the agent tunnel use HTTP/2 over cleartext TCP (no TLS required
	// locally). Connect server-streaming requires HTTP/2; without this the
	// server falls back to HTTP/1.1 and the stream closes immediately.
	// ReadTimeout is disabled because long-lived streams would be cut at 30s.
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           h2c.NewHandler(r, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("control-plane listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
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

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("control-plane stopped")
	return nil
}
