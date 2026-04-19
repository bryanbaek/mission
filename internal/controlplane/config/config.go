package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env         string
	HTTPPort    int
	DatabaseURL string
	LogLevel    string
}

func Load() (Config, error) {
	port, err := strconv.Atoi(getenv("HTTP_PORT", "8080"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid HTTP_PORT: %w", err)
	}
	cfg := Config{
		Env:         getenv("ENV", "development"),
		HTTPPort:    port,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
