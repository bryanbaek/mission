package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/sync/errgroup"

	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/gen/go/onboarding/v1/onboardingv1connect"
	"github.com/bryanbaek/mission/gen/go/query/v1/queryv1connect"
	"github.com/bryanbaek/mission/gen/go/semantic/v1/semanticv1connect"
	"github.com/bryanbaek/mission/gen/go/starter/v1/starterv1connect"
	"github.com/bryanbaek/mission/gen/go/tenant/v1/tenantv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/config"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/db"
	llmgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/handler"
	"github.com/bryanbaek/mission/internal/controlplane/llmprovider"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
)

const frontendDistDir = "web/dist"

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

	llmProviders, semanticProviderModels, queryProviderModels, err := buildLLMRuntime(cfg)
	if err != nil {
		return fmt.Errorf("build llm runtime: %w", err)
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}
	if err := initSentry(cfg); err != nil {
		return fmt.Errorf("init sentry: %w", err)
	}
	defer sentry.Flush(2 * time.Second)

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, db.PoolConfig{
		MaxConns:          cfg.DBMaxConns,
		MinConns:          cfg.DBMinConns,
		HealthCheckPeriod: cfg.DBHealthCheckPeriod,
	})
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	// Repositories
	tenantRepo := repository.NewTenantRepository(pool)
	tokenRepo := repository.NewTenantTokenRepository(pool)
	schemaRepo := repository.NewTenantSchemaRepository(pool)
	semanticLayerRepo := repository.NewTenantSemanticLayerRepository(pool)
	queryRunRepo := repository.NewTenantQueryRunRepository(pool)
	queryFeedbackRepo := repository.NewTenantQueryFeedbackRepository(pool)
	canonicalQueryExampleRepo := repository.NewTenantCanonicalQueryExampleRepository(pool)
	starterQuestionsRepo := repository.NewStarterQuestionsRepository(pool)
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
	llmRouter := llmgateway.NewRouter(cfg.DefaultLLMProvider, llmProviders...)
	semanticLayerCtrl := controller.NewSemanticLayerController(
		tenantCtrl,
		schemaRepo,
		semanticLayerRepo,
		llmRouter,
		controller.SemanticLayerControllerConfig{
			Model:          cfg.SemanticLayerModel,
			ProviderModels: semanticProviderModels,
		},
	)
	queryCtrl := controller.NewQueryController(
		tenantCtrl,
		schemaRepo,
		semanticLayerRepo,
		queryRunRepo,
		queryFeedbackRepo,
		canonicalQueryExampleRepo,
		agentSessions,
		llmRouter,
		controller.QueryControllerConfig{
			Model:                 cfg.QueryModel,
			ProviderModels:        queryProviderModels,
			SummaryProviderModels: queryProviderModels,
		},
	)
	starterQuestionsCtrl := controller.NewStarterQuestionsController(
		tenantCtrl,
		semanticLayerRepo,
		starterQuestionsRepo,
		llmRouter,
		controller.StarterQuestionsControllerConfig{
			Model:          cfg.QueryModel,
			ProviderModels: queryProviderModels,
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
			EdgeAgentImage:        cfg.EdgeAgentImage,
			EdgeAgentVersion:      cfg.EdgeAgentVersion,
			PublicControlPlaneURL: cfg.PublicControlPlaneURL,
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
	starterQuestionsHandler := handler.NewStarterQuestionsHandler(starterQuestionsCtrl)
	onboardingHandler := handler.NewOnboardingHandler(onboardingCtrl)
	tenantPath, tenantSvc := tenantv1connect.NewTenantServiceHandler(tenantHandler)
	semanticPath, semanticSvc := semanticv1connect.NewSemanticLayerServiceHandler(
		semanticLayerHandler,
	)
	queryPath, querySvc := queryv1connect.NewQueryServiceHandler(queryHandler)
	starterQuestionsPath, starterQuestionsSvc := starterv1connect.NewStarterQuestionsServiceHandler(
		starterQuestionsHandler,
	)
	onboardingPath, onboardingSvc := onboardingv1connect.NewOnboardingServiceHandler(
		onboardingHandler,
	)
	agentPath, agentSvc := agentv1connect.NewAgentServiceHandler(agentHandler)
	frontendRuntimeConfigHandler := handler.NewFrontendRuntimeConfigHandler(
		handler.FrontendRuntimeConfig{
			ClerkPublishableKey: cfg.ClerkPublishableKey,
			SentryDSN:           cfg.FrontendSentryDSN,
			SentryEnvironment:   cfg.FrontendSentryEnvironment,
			SentryRelease:       cfg.FrontendSentryRelease,
		},
	)
	frontendHandler, err := loadFrontendHandler(cfg)
	if err != nil {
		return err
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l := slog.Default().With("request_id", middleware.GetReqID(r.Context()))
			next.ServeHTTP(w, r.WithContext(reqlog.WithLogger(r.Context(), l)))
		})
	})
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Connect-Protocol-Version"},
		ExposedHeaders:   []string{"Grpc-Status", "Grpc-Message"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(httprate.LimitByIP(cfg.RateLimitRPM, 1*time.Minute))
	r.Use(maxBodySize(cfg.MaxRequestBodyBytes))
	r.Use(middleware.Recoverer)
	r.Use(sentryMiddleware())

	// Public
	r.Get("/app-config.json", frontendRuntimeConfigHandler.JSON)
	r.Head("/app-config.json", frontendRuntimeConfigHandler.JSON)
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
	// A stricter per-IP rate limit protects upstream LLM spend.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(90 * time.Second))
		r.Use(auth.RequireAuth(verifier))
		r.Use(httprate.LimitByIP(cfg.RateLimitLLMRPM, 1*time.Minute))
		r.Mount(queryPath, querySvc)
		r.Mount(starterQuestionsPath, starterQuestionsSvc)
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
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(verifier))
			r.Get("/api/debug/agents", debugAgentHandler.ListSessions)
			r.Post("/api/debug/agents/{sessionID}/ping", debugAgentHandler.PingSession)
		})
	}
	registerFrontendRoutes(r, frontendHandler)

	// h2c lets the agent tunnel use HTTP/2 over cleartext TCP (no TLS required
	// locally). Connect server-streaming requires HTTP/2; without this the
	// server falls back to HTTP/1.1 and the stream closes immediately.
	//
	// ReadTimeout and WriteTimeout are intentionally 0: the agent command
	// stream is a long-lived HTTP/2 server-stream that can stay open for
	// hours. Setting a global write timeout would kill those connections.
	// Per-route timeouts are enforced via chi middleware.Timeout instead.
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           h2c.NewHandler(r, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		slog.Info("control-plane listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-groupCtx.Done()
		if ctx.Err() != nil {
			slog.Info("shutdown signal received")
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(),
			cfg.ShutdownTimeout,
		)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}
	slog.Info("control-plane stopped")
	return nil
}

func buildLLMRuntime(
	cfg config.Config,
) ([]llmgateway.Provider, map[string]string, map[string]string, error) {
	defaultProvider := strings.TrimSpace(cfg.DefaultLLMProvider)
	if _, ok := llmprovider.ByName(defaultProvider); !ok {
		return nil, nil, nil, fmt.Errorf("unsupported DEFAULT_LLM_PROVIDER %q", defaultProvider)
	}

	providers := make([]llmgateway.Provider, 0, len(llmprovider.Specs()))
	configured := make(map[string]struct{}, len(llmprovider.Specs()))
	for _, spec := range llmprovider.Specs() {
		apiKey := strings.TrimSpace(cfg.ProviderAPIKeys[spec.Name])
		if apiKey == "" {
			continue
		}

		provider, err := llmprovider.Build(spec.Name, apiKey, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		providers = append(providers, provider)
		configured[spec.Name] = struct{}{}
	}

	if len(providers) == 0 {
		return nil, nil, nil, fmt.Errorf(
			"no llm providers are configured; set at least one of %s",
			strings.Join(apiKeyEnvNames(), ", "),
		)
	}
	if _, ok := configured[defaultProvider]; !ok {
		return nil, nil, nil, fmt.Errorf(
			"DEFAULT_LLM_PROVIDER %q is not configured",
			defaultProvider,
		)
	}

	return providers,
		llmgateway.CloneProviderModels(cfg.SemanticLayerProviderModels),
		llmgateway.CloneProviderModels(cfg.QueryProviderModels),
		nil
}

func apiKeyEnvNames() []string {
	names := make([]string, 0, len(llmprovider.Specs()))
	for _, spec := range llmprovider.Specs() {
		names = append(names, spec.APIKeyEnv)
	}
	return names
}

func loadFrontendHandler(cfg config.Config) (http.Handler, error) {
	frontendHandler, err := handler.NewFrontendHandler(frontendDistDir)
	switch {
	case err == nil:
		return frontendHandler, nil
	case errors.Is(err, os.ErrNotExist):
		if cfg.Env == "production" {
			return nil, fmt.Errorf(
				"frontend assets missing at %s: %w",
				filepath.Join(frontendDistDir, "index.html"),
				err,
			)
		}
		slog.Warn(
			"frontend assets missing; SPA routes disabled",
			"path",
			filepath.Join(frontendDistDir, "index.html"),
		)
		return nil, nil
	default:
		return nil, fmt.Errorf("load frontend assets: %w", err)
	}
}

func registerFrontendRoutes(r chi.Router, frontend http.Handler) {
	if frontend == nil {
		return
	}
	r.Get("/", frontend.ServeHTTP)
	r.Head("/", frontend.ServeHTTP)
	r.Get("/*", frontend.ServeHTTP)
	r.Head("/*", frontend.ServeHTTP)
}

func initSentry(cfg config.Config) error {
	if cfg.SentryDSN == "" {
		return nil
	}
	return sentry.Init(sentry.ClientOptions{
		Dsn:         cfg.SentryDSN,
		Environment: cfg.SentryEnvironment,
		Release:     cfg.SentryRelease,
	})
}

func sentryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hub := sentry.CurrentHub().Clone()
			hub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetRequest(r)
				scope.SetTag("request_id", middleware.GetReqID(r.Context()))
			})
			ctx := sentry.SetHubOnContext(r.Context(), hub)
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				if recovered := recover(); recovered != nil {
					eventID := hub.RecoverWithContext(
						ctx,
						fmt.Errorf("panic: %v\n%s", recovered, debug.Stack()),
					)
					if eventID != nil {
						hub.Flush(2 * time.Second)
					}
					panic(recovered)
				}
			}()

			next.ServeHTTP(recorder, r.WithContext(ctx))

			if recorder.statusCode >= http.StatusInternalServerError {
				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetLevel(sentry.LevelError)
					scope.SetAttributes(
						attribute.Int("status_code", recorder.statusCode),
						attribute.String("method", r.Method),
						attribute.String("path", r.URL.Path),
					)
					scope.SetTag("http.status_code", fmt.Sprintf("%d", recorder.statusCode))
					hub.CaptureMessage("control-plane request returned 5xx")
				})
			}
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// maxBodySize returns middleware that limits the size of incoming request
// bodies. Requests that exceed the limit receive a 413 status when the body is
// read by the downstream handler.
func maxBodySize(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}
