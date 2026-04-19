package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("ENV", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Env != "development" {
		t.Fatalf("Env = %q, want development", cfg.Env)
	}
	if cfg.HTTPPort != 8080 {
		t.Fatalf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoadInvalidHTTPPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mission:mission@localhost:5432/mission")
	t.Setenv("HTTP_PORT", "bad-port")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for invalid HTTP_PORT")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("HTTP_PORT", "8080")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error when DATABASE_URL is missing")
	}
}

func TestGetenvUsesFallback(t *testing.T) {
	t.Setenv("EXAMPLE_KEY", "")
	if got := getenv("EXAMPLE_KEY", "fallback"); got != "fallback" {
		t.Fatalf("getenv fallback = %q, want fallback", got)
	}

	t.Setenv("EXAMPLE_KEY", "configured")
	if got := getenv("EXAMPLE_KEY", "fallback"); got != "configured" {
		t.Fatalf("getenv configured = %q, want configured", got)
	}
}
