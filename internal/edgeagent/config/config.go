package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ControlPlaneURL   string
	TenantToken       string
	AgentVersion      string
	HeartbeatInterval time.Duration
	ReconnectBase     time.Duration
	ReconnectMax      time.Duration
	MySQLDSNFile      string
	MySQLDSN          string
}

func Load() (Config, error) {
	mysqlDSNFile := os.Getenv("MYSQL_DSN_FILE")
	if mysqlDSNFile == "" && os.Getenv("MYSQL_DSN") == "" {
		mysqlDSNFile = "/etc/agent/mysql.dsn"
	}

	var mysqlDSN string
	if direct := os.Getenv("MYSQL_DSN"); direct != "" {
		mysqlDSN = direct
	} else {
		loaded, err := loadOptionalDSNFile(mysqlDSNFile)
		if err != nil {
			return Config{}, err
		}
		mysqlDSN = loaded
	}

	cfg := Config{
		ControlPlaneURL:   os.Getenv("CONTROL_PLANE_URL"),
		TenantToken:       os.Getenv("TENANT_TOKEN"),
		AgentVersion:      getenv("AGENT_VERSION", "dev"),
		HeartbeatInterval: getenvDurationSeconds("HEARTBEAT_INTERVAL_SECONDS", 10),
		ReconnectBase:     getenvDurationSeconds("RECONNECT_BASE_SECONDS", 1),
		ReconnectMax:      getenvDurationSeconds("RECONNECT_MAX_SECONDS", 30),
		MySQLDSNFile:      mysqlDSNFile,
		MySQLDSN:          mysqlDSN,
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

func loadDSNFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read MYSQL_DSN_FILE %q: %w", path, err)
	}
	dsn := strings.TrimSpace(string(body))
	if dsn == "" {
		return "", fmt.Errorf("MYSQL_DSN_FILE %q is empty", path)
	}
	return dsn, nil
}

func loadOptionalDSNFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("read MYSQL_DSN_FILE %q: %w", path, err)
	}
	dsn := strings.TrimSpace(string(body))
	if dsn == "" {
		return "", nil
	}
	return dsn, nil
}

func DSNDirectory(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
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
