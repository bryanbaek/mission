package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	dsnFile := filepath.Join(t.TempDir(), "mysql.dsn")
	if err := os.WriteFile(
		dsnFile,
		[]byte("mission_ro:mission_ro@tcp(localhost:3306)/mission_app\n"),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("CONTROL_PLANE_URL", "")
	t.Setenv("TENANT_TOKEN", "")
	t.Setenv("MYSQL_DSN_FILE", dsnFile)

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
	t.Setenv("MYSQL_DSN_FILE", dsnFile)

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
	if cfg.MySQLDSNFile != dsnFile {
		t.Fatalf("MySQLDSNFile = %q, want %q", cfg.MySQLDSNFile, dsnFile)
	}
	if cfg.MySQLDSN != "mission_ro:mission_ro@tcp(localhost:3306)/mission_app" {
		t.Fatalf("MySQLDSN = %q, unexpected value", cfg.MySQLDSN)
	}
}

func TestLoadMySQLDSNDirectEnvVar(t *testing.T) {
	// MYSQL_DSN env var should be used directly without reading a file.
	t.Setenv("CONTROL_PLANE_URL", "http://localhost:8080")
	t.Setenv("TENANT_TOKEN", "secret")
	t.Setenv("MYSQL_DSN", "root:pw@tcp(localhost:3306)/db")
	t.Setenv("MYSQL_DSN_FILE", "") // explicitly absent

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MySQLDSN != "root:pw@tcp(localhost:3306)/db" {
		t.Fatalf("MySQLDSN = %q, want inline value", cfg.MySQLDSN)
	}
	if cfg.MySQLDSNFile != "" {
		t.Fatalf("MySQLDSNFile = %q, want empty when MYSQL_DSN is set", cfg.MySQLDSNFile)
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

func TestLoadDSNFile(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.dsn")
	if _, err := loadDSNFile(missingPath); err == nil {
		t.Fatal("loadDSNFile returned nil error for missing file")
	}

	blankPath := filepath.Join(t.TempDir(), "blank.dsn")
	if err := os.WriteFile(blankPath, []byte(" \n\t "), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := loadDSNFile(blankPath); err == nil {
		t.Fatal("loadDSNFile returned nil error for blank file")
	}

	validPath := filepath.Join(t.TempDir(), "valid.dsn")
	if err := os.WriteFile(
		validPath,
		[]byte("mission_ro:mission_ro@tcp(localhost:3306)/mission_app\n"),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	got, err := loadDSNFile(validPath)
	if err != nil {
		t.Fatalf("loadDSNFile returned error: %v", err)
	}
	if got != "mission_ro:mission_ro@tcp(localhost:3306)/mission_app" {
		t.Fatalf("loadDSNFile = %q, unexpected value", got)
	}
}
