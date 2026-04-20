package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	edgeconfig "github.com/bryanbaek/mission/internal/edgeagent/config"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	controlplane "github.com/bryanbaek/mission/internal/edgeagent/gateway/controlplane"
	mysqlgateway "github.com/bryanbaek/mission/internal/edgeagent/gateway/mysql"
)

func main() {
	if err := run(); err != nil {
		slog.Error("edge-agent exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelInfo},
		),
	)
	slog.SetDefault(logger)

	cfg, err := edgeconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	mysqlGateway, err := mysqlgateway.Open(ctx, cfg.MySQLDSN)
	if err != nil {
		return fmt.Errorf("open mysql gateway: %w", err)
	}
	defer mysqlGateway.Close()

	client := controlplane.NewClient(cfg.ControlPlaneURL, cfg.TenantToken, nil)
	service, err := edgecontroller.NewAgentService(
		client,
		edgecontroller.AgentServiceConfig{
			AgentVersion:      cfg.AgentVersion,
			HeartbeatInterval: cfg.HeartbeatInterval,
			ReconnectBase:     cfg.ReconnectBase,
			ReconnectMax:      cfg.ReconnectMax,
			Logger:            logger,
			QueryExecutor:     mysqlQueryExecutor{gateway: mysqlGateway},
		},
	)
	if err != nil {
		return fmt.Errorf("build service: %w", err)
	}

	slog.Info(
		"edge-agent starting",
		"control_plane_url",
		cfg.ControlPlaneURL,
		"agent_version",
		cfg.AgentVersion,
	)
	return service.Run(ctx)
}

type mysqlQueryExecutor struct {
	gateway *mysqlgateway.Gateway
}

func (e mysqlQueryExecutor) ExecuteQuery(
	ctx context.Context,
	sql string,
) (edgecontroller.QueryResult, error) {
	result, err := e.gateway.ExecuteQuery(ctx, sql)
	if err != nil {
		return edgecontroller.QueryResult{}, err
	}
	return edgecontroller.QueryResult{
		Columns:      result.Columns,
		Rows:         result.Rows,
		ElapsedMS:    result.ElapsedMS,
		DatabaseUser: result.DatabaseUser,
		DatabaseName: result.DatabaseName,
	}, nil
}
