package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxuuid "github.com/vgarvardt/pgx-google-uuid/v5"
)

type poolPinger interface {
	Ping(ctx context.Context) error
	Close()
}

var parsePoolConfig = pgxpool.ParseConfig

type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	HealthCheckPeriod time.Duration
}

func defaultNewPoolWithConfig(
	ctx context.Context,
	cfg *pgxpool.Config,
) (*pgxpool.Pool, poolPinger, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return pool, pool, nil
}

var newPoolWithConfig = defaultNewPoolWithConfig

func NewPool(
	ctx context.Context,
	databaseURL string,
	poolConfig PoolConfig,
) (*pgxpool.Pool, error) {
	cfg, err := parsePoolConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if poolConfig.MaxConns <= 0 {
		return nil, fmt.Errorf("max conns must be greater than 0")
	}
	if poolConfig.MinConns < 0 {
		return nil, fmt.Errorf("min conns must be greater than or equal to 0")
	}
	if poolConfig.MinConns > poolConfig.MaxConns {
		return nil, fmt.Errorf("min conns must be less than or equal to max conns")
	}
	if poolConfig.HealthCheckPeriod <= 0 {
		return nil, fmt.Errorf("health check period must be greater than 0")
	}
	cfg.MaxConns = poolConfig.MaxConns
	cfg.MinConns = poolConfig.MinConns
	cfg.HealthCheckPeriod = poolConfig.HealthCheckPeriod
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}

	pool, pinger, err := newPoolWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pinger.Ping(pingCtx); err != nil {
		pinger.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
