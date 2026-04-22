package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                         string
	HTTPPort                    int
	ShutdownTimeout             time.Duration
	DatabaseURL                 string
	DBMaxConns                  int32
	DBMinConns                  int32
	DBHealthCheckPeriod         time.Duration
	LogLevel                    string
	ClerkSecretKey              string
	ClerkPublishableKey         string
	FrontendSentryDSN           string
	FrontendSentryEnvironment   string
	FrontendSentryRelease       string
	PublicControlPlaneURL       string
	AnthropicAPIKey             string
	OpenAIAPIKey                string
	DefaultLLMProvider          string
	SemanticLayerModel          string
	QueryModel                  string
	AnthropicSemanticLayerModel string
	OpenAISemanticLayerModel    string
	AnthropicQueryModel         string
	OpenAIQueryModel            string
	EdgeAgentVersion            string
	EdgeAgentImageRepo          string
	EdgeAgentImage              string
	SentryDSN                   string
	SentryEnvironment           string
	SentryRelease               string
	AnthropicPreflightModel     string
	OpenAIPreflightModel        string
}

func Load() (Config, error) {
	port, err := loadHTTPPort()
	if err != nil {
		return Config{}, err
	}
	shutdownSecs, err := strconv.Atoi(getenv("SHUTDOWN_TIMEOUT_SECONDS", "10"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT_SECONDS: %w", err)
	}
	dbMaxConns, err := strconv.Atoi(getenv("DB_MAX_CONNS", "4"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid DB_MAX_CONNS: %w", err)
	}
	dbMinConns, err := strconv.Atoi(getenv("DB_MIN_CONNS", "1"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid DB_MIN_CONNS: %w", err)
	}
	dbHealthCheckPeriodSecs, err := strconv.Atoi(
		getenv("DB_HEALTH_CHECK_PERIOD_SECONDS", "30"),
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"invalid DB_HEALTH_CHECK_PERIOD_SECONDS: %w",
			err,
		)
	}
	if dbMaxConns <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be greater than 0")
	}
	if dbMinConns < 0 {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must be greater than or equal to 0")
	}
	if dbMinConns > dbMaxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must be less than or equal to DB_MAX_CONNS")
	}
	if dbHealthCheckPeriodSecs <= 0 {
		return Config{}, fmt.Errorf(
			"DB_HEALTH_CHECK_PERIOD_SECONDS must be greater than 0",
		)
	}
	cfg := Config{
		Env:             getenv("ENV", "development"),
		HTTPPort:        port,
		ShutdownTimeout: time.Duration(shutdownSecs) * time.Second,
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DBMaxConns:      int32(dbMaxConns),
		DBMinConns:      int32(dbMinConns),
		DBHealthCheckPeriod: time.Duration(
			dbHealthCheckPeriodSecs,
		) * time.Second,
		LogLevel:       getenv("LOG_LEVEL", "info"),
		ClerkSecretKey: os.Getenv("CLERK_SECRET_KEY"),
		ClerkPublishableKey: firstNonEmpty(
			os.Getenv("VITE_CLERK_PUBLISHABLE_KEY"),
			os.Getenv("CLERK_PUBLISHABLE_KEY"),
		),
		FrontendSentryDSN: firstNonEmpty(
			os.Getenv("VITE_SENTRY_DSN"),
			os.Getenv("SENTRY_DSN"),
		),
		FrontendSentryEnvironment: firstNonEmpty(
			os.Getenv("VITE_SENTRY_ENVIRONMENT"),
			os.Getenv("SENTRY_ENVIRONMENT"),
			getenv("ENV", "development"),
		),
		FrontendSentryRelease: firstNonEmpty(
			os.Getenv("VITE_SENTRY_RELEASE"),
			os.Getenv("SENTRY_RELEASE"),
		),
		PublicControlPlaneURL: strings.TrimSpace(os.Getenv("PUBLIC_CONTROL_PLANE_URL")),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		DefaultLLMProvider:    getenv("DEFAULT_LLM_PROVIDER", "anthropic"),
		SemanticLayerModel:    getenv("SEMANTIC_LAYER_MODEL", "claude-sonnet-4-6"),
		QueryModel:            getenv("QUERY_MODEL", "claude-sonnet-4-6"),
		AnthropicSemanticLayerModel: strings.TrimSpace(
			os.Getenv("ANTHROPIC_SEMANTIC_LAYER_MODEL"),
		),
		OpenAISemanticLayerModel: strings.TrimSpace(
			os.Getenv("OPENAI_SEMANTIC_LAYER_MODEL"),
		),
		AnthropicQueryModel: strings.TrimSpace(
			os.Getenv("ANTHROPIC_QUERY_MODEL"),
		),
		OpenAIQueryModel: strings.TrimSpace(
			os.Getenv("OPENAI_QUERY_MODEL"),
		),
		EdgeAgentVersion: getenv("EDGE_AGENT_VERSION", "v0.1.0"),
		EdgeAgentImageRepo: getenv(
			"EDGE_AGENT_IMAGE_REPOSITORY",
			"registry.digitalocean.com/mission/edge-agent",
		),
		SentryDSN:         os.Getenv("SENTRY_DSN"),
		SentryEnvironment: getenv("SENTRY_ENVIRONMENT", getenv("ENV", "development")),
		SentryRelease:     os.Getenv("SENTRY_RELEASE"),
		AnthropicPreflightModel: getenv(
			"ANTHROPIC_PREFLIGHT_MODEL",
			"claude-3-5-haiku-latest",
		),
		OpenAIPreflightModel: getenv("OPENAI_PREFLIGHT_MODEL", "gpt-4.1-nano"),
	}
	cfg.EdgeAgentImage = resolveEdgeAgentImage(
		os.Getenv("EDGE_AGENT_IMAGE"),
		cfg.EdgeAgentImageRepo,
		cfg.EdgeAgentVersion,
	)
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.ClerkSecretKey == "" && cfg.Env != "development" {
		return Config{}, fmt.Errorf(
			"CLERK_SECRET_KEY is required in non-development environments",
		)
	}
	if cfg.PublicControlPlaneURL == "" && cfg.Env == "production" {
		return Config{}, fmt.Errorf("PUBLIC_CONTROL_PLANE_URL is required in production")
	}
	if cfg.PublicControlPlaneURL == "" {
		cfg.PublicControlPlaneURL = fmt.Sprintf("http://localhost:%d", cfg.HTTPPort)
	}
	return cfg, nil
}

func loadHTTPPort() (int, error) {
	raw := strings.TrimSpace(firstNonEmpty(os.Getenv("PORT"), os.Getenv("HTTP_PORT")))
	if raw == "" {
		raw = "8080"
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid PORT/HTTP_PORT: %w", err)
	}
	return port, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func resolveEdgeAgentImage(override, repository, version string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	repository = strings.TrimSpace(repository)
	version = strings.TrimSpace(version)
	switch {
	case repository == "" && version == "":
		return ""
	case repository == "":
		return version
	case version == "":
		return repository
	default:
		return repository + ":" + version
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
