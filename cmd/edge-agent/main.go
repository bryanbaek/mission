package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/net/http2"

	edgeconfig "github.com/bryanbaek/mission/internal/edgeagent/config"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	controlplane "github.com/bryanbaek/mission/internal/edgeagent/gateway/controlplane"
	edgeruntime "github.com/bryanbaek/mission/internal/edgeagent/runtime"
)

func main() {
	if err := run(); err != nil {
		slog.Error("edge-agent exited with error", "err", err)
		os.Exit(1)
	}
}

func run() (err error) {
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

	mysqlRuntime, err := edgeruntime.NewMySQLRuntime(
		ctx,
		cfg.MySQLDSNFile,
		cfg.MySQLDSN,
	)
	if err != nil {
		return fmt.Errorf("open mysql runtime: %w", err)
	}
	defer func() {
		if closeErr := mysqlRuntime.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close mysql runtime: %w", closeErr))
		}
	}()

	client := controlplane.NewClient(
		cfg.ControlPlaneURL,
		cfg.TenantToken,
		httpClientForURL(cfg.ControlPlaneURL),
	)
	service, err := edgecontroller.NewAgentService(
		client,
		edgecontroller.AgentServiceConfig{
			AgentVersion:       cfg.AgentVersion,
			HeartbeatInterval:  cfg.HeartbeatInterval,
			ReconnectBase:      cfg.ReconnectBase,
			ReconnectMax:       cfg.ReconnectMax,
			Logger:             logger,
			QueryExecutor:      mysqlRuntime,
			SchemaIntrospector: mysqlRuntime,
			DatabaseConfigurer: mysqlRuntime,
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

// httpClientForURL returns an HTTP client appropriate for the given URL scheme.
// For http:// (no TLS) the client uses an HTTP/2 cleartext (h2c) transport so
// that Connect server-streaming RPCs work without a TLS certificate. For
// https:// the default client is returned; HTTP/2 is negotiated via TLS ALPN.
func httpClientForURL(rawURL string) *http.Client {
	if strings.HasPrefix(rawURL, "https://") {
		return http.DefaultClient
	}
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(
				ctx context.Context,
				network, addr string,
				_ *tls.Config,
			) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
}
