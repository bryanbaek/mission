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
	mysqlgateway "github.com/bryanbaek/mission/internal/edgeagent/gateway/mysql"
	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
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

	mysqlGateway, err := mysqlgateway.Open(ctx, cfg.MySQLDSN)
	if err != nil {
		return fmt.Errorf("open mysql gateway: %w", err)
	}
	defer func() {
		if closeErr := mysqlGateway.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close mysql gateway: %w", closeErr))
		}
	}()

	client := controlplane.NewClient(
		cfg.ControlPlaneURL,
		cfg.TenantToken,
		httpClientForURL(cfg.ControlPlaneURL),
	)
	mysqlRuntime := mysqlRuntime{gateway: mysqlGateway}
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

type mysqlRuntime struct {
	gateway *mysqlgateway.Gateway
}

func (e mysqlRuntime) ExecuteQuery(
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

func (e mysqlRuntime) IntrospectSchema(
	ctx context.Context,
) (
	introspect.SchemaBlob,
	int64,
	string,
	string,
	error,
) {
	return e.gateway.IntrospectSchema(ctx)
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
