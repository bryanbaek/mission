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

	client := controlplane.NewClient(cfg.ControlPlaneURL, cfg.TenantToken, nil)
	service, err := edgecontroller.NewAgentService(
		client,
		edgecontroller.AgentServiceConfig{
			AgentVersion:      cfg.AgentVersion,
			HeartbeatInterval: cfg.HeartbeatInterval,
			ReconnectBase:     cfg.ReconnectBase,
			ReconnectMax:      cfg.ReconnectMax,
			Logger:            logger,
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
