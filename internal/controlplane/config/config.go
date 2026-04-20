package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env                string
	HTTPPort           int
	DatabaseURL        string
	LogLevel           string
	ClerkSecretKey     string
	AnthropicAPIKey    string
	OpenAIAPIKey       string
	DefaultLLMProvider string
	SemanticLayerModel string
	QueryModel         string
}

func Load() (Config, error) {
	port, err := strconv.Atoi(getenv("HTTP_PORT", "8080"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid HTTP_PORT: %w", err)
	}
	cfg := Config{
		Env:                getenv("ENV", "development"),
		HTTPPort:           port,
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		LogLevel:           getenv("LOG_LEVEL", "info"),
		ClerkSecretKey:     os.Getenv("CLERK_SECRET_KEY"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		DefaultLLMProvider: getenv("DEFAULT_LLM_PROVIDER", "anthropic"),
		SemanticLayerModel: getenv("SEMANTIC_LAYER_MODEL", "claude-sonnet-4-6"),
		QueryModel:         getenv("QUERY_MODEL", "claude-sonnet-4-6"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.ClerkSecretKey == "" && cfg.Env != "development" {
		return Config{}, fmt.Errorf(
			"CLERK_SECRET_KEY is required in non-development environments",
		)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
