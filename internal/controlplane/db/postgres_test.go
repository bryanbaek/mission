package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fakePoolClient struct {
	pingErr error
	ctx     context.Context
	closed  bool
}

func (f *fakePoolClient) Ping(ctx context.Context) error {
	f.ctx = ctx
	return f.pingErr
}

func (f *fakePoolClient) Close() {
	f.closed = true
}

func restorePoolSeams(t *testing.T) {
	t.Helper()

	origParse := parsePoolConfig
	origNewPool := newPoolWithConfig
	t.Cleanup(func() {
		parsePoolConfig = origParse
		newPoolWithConfig = origNewPool
	})
}

func TestNewPoolReturnsParseError(t *testing.T) {
	t.Parallel()

	_, err := NewPool(context.Background(), "://bad-url")
	if err == nil {
		t.Fatal("NewPool returned nil error for invalid URL")
	}
	if !strings.Contains(err.Error(), "parse database url") {
		t.Fatalf("error = %v, want wrapped parse database url error", err)
	}
}

func TestDefaultNewPoolWithConfigReturnsPoolAndPinger(t *testing.T) {
	t.Parallel()

	cfg, err := pgxpool.ParseConfig("postgres://mission:mission@localhost:5432/mission")
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}

	pool, pinger, err := defaultNewPoolWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("defaultNewPoolWithConfig returned error: %v", err)
	}
	if pool == nil {
		t.Fatal("defaultNewPoolWithConfig returned nil pool")
	}
	if pinger == nil {
		t.Fatal("defaultNewPoolWithConfig returned nil pinger")
	}

	pinger.Close()
}

func TestNewPoolConfiguresPoolAndPings(t *testing.T) {
	restorePoolSeams(t)

	cfg := &pgxpool.Config{}
	client := &fakePoolClient{}
	dummyPool := new(pgxpool.Pool)

	parsePoolConfig = func(databaseURL string) (*pgxpool.Config, error) {
		if databaseURL != "postgres://mission:mission@localhost:5432/mission" {
			t.Fatalf("databaseURL = %q, want expected URL", databaseURL)
		}
		return cfg, nil
	}
	newPoolWithConfig = func(ctx context.Context, gotCfg *pgxpool.Config) (*pgxpool.Pool, poolPinger, error) {
		if gotCfg != cfg {
			t.Fatal("NewPool did not use parsed config")
		}
		return dummyPool, client, nil
	}

	pool, err := NewPool(context.Background(), "postgres://mission:mission@localhost:5432/mission")
	if err != nil {
		t.Fatalf("NewPool returned error: %v", err)
	}
	if pool != dummyPool {
		t.Fatal("NewPool returned unexpected pool pointer")
	}
	if cfg.MaxConns != 10 {
		t.Fatalf("MaxConns = %d, want 10", cfg.MaxConns)
	}
	if cfg.MinConns != 1 {
		t.Fatalf("MinConns = %d, want 1", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Fatalf("MaxConnLifetime = %s, want 1h", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Fatalf("MaxConnIdleTime = %s, want 30m", cfg.MaxConnIdleTime)
	}
	if cfg.AfterConnect == nil {
		t.Fatal("AfterConnect was not configured")
	}

	deadline, ok := client.ctx.Deadline()
	if !ok {
		t.Fatal("Ping context did not have deadline")
	}
	until := time.Until(deadline)
	if until < 4*time.Second || until > 6*time.Second {
		t.Fatalf("ping deadline in %s, want roughly 5s", until)
	}
}

func TestNewPoolReturnsCreateError(t *testing.T) {
	restorePoolSeams(t)

	parsePoolConfig = func(string) (*pgxpool.Config, error) {
		return &pgxpool.Config{}, nil
	}
	newPoolWithConfig = func(context.Context, *pgxpool.Config) (*pgxpool.Pool, poolPinger, error) {
		return nil, nil, errors.New("create failed")
	}

	_, err := NewPool(context.Background(), "postgres://mission")
	if err == nil {
		t.Fatal("NewPool returned nil error")
	}
	if !strings.Contains(err.Error(), "create pool") {
		t.Fatalf("error = %v, want wrapped create pool error", err)
	}
}

func TestNewPoolClosesPoolWhenPingFails(t *testing.T) {
	restorePoolSeams(t)

	client := &fakePoolClient{pingErr: errors.New("database down")}

	parsePoolConfig = func(string) (*pgxpool.Config, error) {
		return &pgxpool.Config{}, nil
	}
	newPoolWithConfig = func(context.Context, *pgxpool.Config) (*pgxpool.Pool, poolPinger, error) {
		return new(pgxpool.Pool), client, nil
	}

	pool, err := NewPool(context.Background(), "postgres://mission")
	if err == nil {
		t.Fatal("NewPool returned nil error")
	}
	if pool != nil {
		t.Fatal("NewPool returned non-nil pool on ping failure")
	}
	if !strings.Contains(err.Error(), "ping database") {
		t.Fatalf("error = %v, want wrapped ping database error", err)
	}
	if !client.closed {
		t.Fatal("pool was not closed after ping failure")
	}
}
