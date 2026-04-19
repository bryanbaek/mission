package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("CONTROL_PLANE_URL", "")
	t.Setenv("TENANT_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error without required env")
	}

	t.Setenv("CONTROL_PLANE_URL", "http://localhost:8080")
	t.Setenv("TENANT_TOKEN", "secret")
	t.Setenv("AGENT_VERSION", "v1")
	t.Setenv("HEARTBEAT_INTERVAL_SECONDS", "12")
	t.Setenv("RECONNECT_BASE_SECONDS", "2")
	t.Setenv("RECONNECT_MAX_SECONDS", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ControlPlaneURL != "http://localhost:8080" {
		t.Fatalf("ControlPlaneURL = %q", cfg.ControlPlaneURL)
	}
	if cfg.AgentVersion != "v1" {
		t.Fatalf("AgentVersion = %q, want v1", cfg.AgentVersion)
	}
	if cfg.HeartbeatInterval != 12*time.Second {
		t.Fatalf("HeartbeatInterval = %s, want 12s", cfg.HeartbeatInterval)
	}
	if cfg.ReconnectBase != 2*time.Second {
		t.Fatalf("ReconnectBase = %s, want 2s", cfg.ReconnectBase)
	}
	if cfg.ReconnectMax != 10*time.Second {
		t.Fatalf("ReconnectMax = %s, want 10s", cfg.ReconnectMax)
	}
}

func TestGetenvDurationSeconds(t *testing.T) {
	t.Setenv("SECONDS", "bad")
	if got := getenvDurationSeconds("SECONDS", 5); got != -1 {
		t.Fatalf("duration = %s, want -1", got)
	}

	t.Setenv("SECONDS", "7")
	if got := getenvDurationSeconds("SECONDS", 5); got != 7*time.Second {
		t.Fatalf("duration = %s, want 7s", got)
	}
}
