package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env                       string
	HTTPPort                  int
	DatabaseURL               string
	LogLevel                  string
	ClerkSecretKey            string
	ClerkPublishableKey       string
	FrontendSentryDSN         string
	FrontendSentryEnvironment string
	FrontendSentryRelease     string
	PublicControlPlaneURL     string
	AnthropicAPIKey           string
	OpenAIAPIKey              string
	DefaultLLMProvider        string
	SemanticLayerModel        string
	QueryModel                string
	EdgeAgentVersion          string
	EdgeAgentImageRepo        string
	EdgeAgentImage            string
	SentryDSN                 string
	SentryEnvironment         string
	SentryRelease             string
	AnthropicPreflightModel   string
	OpenAIPreflightModel      string
}

func Load() (Config, error) {
	port, err := loadHTTPPort()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Env:                 getenv("ENV", "development"),
		HTTPPort:            port,
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		LogLevel:            getenv("LOG_LEVEL", "info"),
		ClerkSecretKey:      os.Getenv("CLERK_SECRET_KEY"),
		ClerkPublishableKey: firstNonEmpty(os.Getenv("VITE_CLERK_PUBLISHABLE_KEY"), os.Getenv("CLERK_PUBLISHABLE_KEY")),
		FrontendSentryDSN:   firstNonEmpty(os.Getenv("VITE_SENTRY_DSN"), os.Getenv("SENTRY_DSN")),
		FrontendSentryEnvironment: firstNonEmpty(
			os.Getenv("VITE_SENTRY_ENVIRONMENT"),
			os.Getenv("SENTRY_ENVIRONMENT"),
			getenv("ENV", "development"),
		),
		FrontendSentryRelease:   firstNonEmpty(os.Getenv("VITE_SENTRY_RELEASE"), os.Getenv("SENTRY_RELEASE")),
		PublicControlPlaneURL:   strings.TrimSpace(os.Getenv("PUBLIC_CONTROL_PLANE_URL")),
		AnthropicAPIKey:         os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:            os.Getenv("OPENAI_API_KEY"),
		DefaultLLMProvider:      getenv("DEFAULT_LLM_PROVIDER", "anthropic"),
		SemanticLayerModel:      getenv("SEMANTIC_LAYER_MODEL", "claude-sonnet-4-6"),
		QueryModel:              getenv("QUERY_MODEL", "claude-sonnet-4-6"),
		EdgeAgentVersion:        getenv("EDGE_AGENT_VERSION", "v0.1.0"),
		EdgeAgentImageRepo:      getenv("EDGE_AGENT_IMAGE_REPOSITORY", "registry.digitalocean.com/mission/edge-agent"),
		SentryDSN:               os.Getenv("SENTRY_DSN"),
		SentryEnvironment:       getenv("SENTRY_ENVIRONMENT", getenv("ENV", "development")),
		SentryRelease:           os.Getenv("SENTRY_RELEASE"),
		AnthropicPreflightModel: getenv("ANTHROPIC_PREFLIGHT_MODEL", "claude-3-5-haiku-latest"),
		OpenAIPreflightModel:    getenv("OPENAI_PREFLIGHT_MODEL", "gpt-4.1-nano"),
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
