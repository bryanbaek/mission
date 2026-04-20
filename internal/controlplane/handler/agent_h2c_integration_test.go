package handler

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	agentv1connect "github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	controlplane "github.com/bryanbaek/mission/internal/edgeagent/gateway/controlplane"
)

type fixedAgentTokenStore struct {
	token string
	row   model.TenantToken
}

func (s fixedAgentTokenStore) LookupActiveByHash(
	_ context.Context,
	hash []byte,
) (model.TenantToken, error) {
	sum := sha256.Sum256([]byte(s.token))
	if string(hash) != string(sum[:]) {
		return model.TenantToken{}, repository.ErrNotFound
	}
	return s.row, nil
}

func (s fixedAgentTokenStore) TouchLastUsed(
	_ context.Context,
	_ uuid.UUID,
) error {
	return nil
}

func TestAgentHandlerH2CStreamStaysOpenUntilSessionEnds(t *testing.T) {
	t.Parallel()

	manager := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{})
	handler := NewAgentHandler(manager)
	token := model.TenantToken{
		ID:       mustUUID(t, "11111111-1111-1111-1111-111111111111"),
		TenantID: mustUUID(t, "22222222-2222-2222-2222-222222222222"),
		Label:    "edge-h2c",
	}

	path, svc := agentv1connect.NewAgentServiceHandler(handler)
	mux := http.NewServeMux()
	mux.Handle(path, auth.RequireAgentToken(fixedAgentTokenStore{
		token: "test-token",
		row:   token,
	})(svc))

	server := &http.Server{
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("shutdown server: %v", err)
		}
		if err := <-serverErrCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("serve: %v", err)
		}
	}()

	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(
				ctx context.Context,
				network, addr string,
				_ *tls.Config,
			) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}

	client := controlplane.NewClient(
		"http://"+listener.Addr().String(),
		"test-token",
		httpClient,
	)

	stream, err := client.OpenCommandStream(
		context.Background(),
		edgecontroller.OpenCommandStreamRequest{
			SessionID:    "session-1",
			Hostname:     "host-a",
			AgentVersion: "v1",
			StartedAt:    time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatalf("OpenCommandStream returned error: %v", err)
	}

	if !stream.Receive() {
		t.Fatalf("Receive returned false for ack: %v", stream.Err())
	}
	if got := stream.Message().SessionID; got != "session-1" {
		t.Fatalf("ack session_id = %q, want session-1", got)
	}

	receiveResult := make(chan struct {
		ok  bool
		err error
	}, 1)
	go func() {
		ok := stream.Receive()
		receiveResult <- struct {
			ok  bool
			err error
		}{ok: ok, err: stream.Err()}
	}()

	select {
	case result := <-receiveResult:
		t.Fatalf("stream closed early: ok=%v err=%v", result.ok, result.err)
	case <-time.After(100 * time.Millisecond):
	}

	manager.DisconnectSession("session-1")

	select {
	case result := <-receiveResult:
		if result.ok {
			t.Fatal("expected closed stream after disconnect")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream to close")
	}
}

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", raw, err)
	}
	return id
}
