package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ControlPlaneURL   string
	TenantToken       string
	AgentVersion      string
	HeartbeatInterval time.Duration
	ReconnectBase     time.Duration
	ReconnectMax      time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ControlPlaneURL:   os.Getenv("CONTROL_PLANE_URL"),
		TenantToken:       os.Getenv("TENANT_TOKEN"),
		AgentVersion:      getenv("AGENT_VERSION", "dev"),
		HeartbeatInterval: getenvDurationSeconds("HEARTBEAT_INTERVAL_SECONDS", 10),
		ReconnectBase:     getenvDurationSeconds("RECONNECT_BASE_SECONDS", 1),
		ReconnectMax:      getenvDurationSeconds("RECONNECT_MAX_SECONDS", 30),
	}
	if cfg.ControlPlaneURL == "" {
		return Config{}, fmt.Errorf("CONTROL_PLANE_URL is required")
	}
	if cfg.TenantToken == "" {
		return Config{}, fmt.Errorf("TENANT_TOKEN is required")
	}
	if cfg.HeartbeatInterval <= 0 {
		return Config{}, fmt.Errorf("HEARTBEAT_INTERVAL_SECONDS must be > 0")
	}
	if cfg.ReconnectBase <= 0 {
		return Config{}, fmt.Errorf("RECONNECT_BASE_SECONDS must be > 0")
	}
	if cfg.ReconnectMax < cfg.ReconnectBase {
		return Config{}, fmt.Errorf(
			"RECONNECT_MAX_SECONDS must be >= RECONNECT_BASE_SECONDS",
		)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvDurationSeconds(key string, fallback int) time.Duration {
	value := getenv(key, strconv.Itoa(fallback))
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return time.Duration(seconds) * time.Second
}
