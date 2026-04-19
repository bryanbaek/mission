package controlplane

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/bryanbaek/mission/gen/go/agent/v1"
	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
)

type recordingAgentService struct {
	lastAuthHeader string
	lastOpen       *agentv1.OpenCommandStreamRequest
	lastHeartbeat  *agentv1.HeartbeatRequest
	lastResult     *agentv1.SubmitCommandResultRequest
}

func (s *recordingAgentService) OpenCommandStream(
	ctx context.Context,
	req *connect.Request[agentv1.OpenCommandStreamRequest],
	stream *connect.ServerStream[agentv1.ControlMessage],
) error {
	s.lastAuthHeader = req.Header().Get("Authorization")
	s.lastOpen = req.Msg
	return stream.Send(&agentv1.ControlMessage{
		SessionId: "session-1",
		CommandId: "command-1",
		IssuedAt:  timestamppb.New(time.Unix(1_700_000_000, 0).UTC()),
		Payload:   &agentv1.ControlMessage_Ping{Ping: &agentv1.PingCommand{}},
	})
}

func (s *recordingAgentService) Heartbeat(
	ctx context.Context,
	req *connect.Request[agentv1.HeartbeatRequest],
) (*connect.Response[agentv1.HeartbeatResponse], error) {
	s.lastAuthHeader = req.Header().Get("Authorization")
	s.lastHeartbeat = req.Msg
	return connect.NewResponse(&agentv1.HeartbeatResponse{}), nil
}

func (s *recordingAgentService) SubmitCommandResult(
	ctx context.Context,
	req *connect.Request[agentv1.SubmitCommandResultRequest],
) (*connect.Response[agentv1.SubmitCommandResultResponse], error) {
	s.lastAuthHeader = req.Header().Get("Authorization")
	s.lastResult = req.Msg
	return connect.NewResponse(&agentv1.SubmitCommandResultResponse{}), nil
}

func TestClientRoundTripsRequests(t *testing.T) {
	t.Parallel()

	service := &recordingAgentService{}
	_, handler := agentv1connect.NewAgentServiceHandler(service)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewClient(server.URL, "token-123", server.Client())

	stream, err := client.OpenCommandStream(
		context.Background(),
		edgecontroller.OpenCommandStreamRequest{
			SessionID:    "session-1",
			Hostname:     "host-a",
			AgentVersion: "v1",
			StartedAt:    time.Unix(1_700_000_000, 0).UTC(),
		},
	)
	if err != nil {
		t.Fatalf("OpenCommandStream returned error: %v", err)
	}
	if !stream.Receive() {
		t.Fatalf("Receive returned false: %v", stream.Err())
	}

	command := stream.Message()
	if command.Kind != edgecontroller.CommandKindPing {
		t.Fatalf("Kind = %q, want ping", command.Kind)
	}
	if service.lastAuthHeader != "Bearer token-123" {
		t.Fatalf("Authorization = %q, want bearer token", service.lastAuthHeader)
	}
	if service.lastOpen.GetHostname() != "host-a" {
		t.Fatalf("Hostname = %q, want host-a", service.lastOpen.GetHostname())
	}

	if err := client.Heartbeat(
		context.Background(),
		edgecontroller.HeartbeatRequest{
			SessionID: "session-1",
			SentAt:    time.Unix(1_700_000_001, 0).UTC(),
		},
	); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if service.lastHeartbeat.GetSessionId() != "session-1" {
		t.Fatalf(
			"Heartbeat session = %q, want session-1",
			service.lastHeartbeat.GetSessionId(),
		)
	}

	if err := client.SubmitPingResult(
		context.Background(),
		edgecontroller.SubmitPingResultRequest{
			SessionID:   "session-1",
			CommandID:   "command-1",
			CompletedAt: time.Unix(1_700_000_002, 0).UTC(),
			RoundTripMS: 42,
		},
	); err != nil {
		t.Fatalf("SubmitPingResult returned error: %v", err)
	}
	if service.lastResult.GetPing().GetRoundTripMs() != 42 {
		t.Fatalf(
			"RoundTripMs = %d, want 42",
			service.lastResult.GetPing().GetRoundTripMs(),
		)
	}
}

func TestBearerInterceptorWrapStreamingHandler(t *testing.T) {
	t.Parallel()

	called := false
	interceptor := &bearerInterceptor{token: "token-123"}
	handler := interceptor.WrapStreamingHandler(
		func(context.Context, connect.StreamingHandlerConn) error {
			called = true
			return nil
		},
	)

	if err := handler(context.Background(), nil); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !called {
		t.Fatal("expected wrapped handler to run")
	}
}

func TestCommandKindUnknownPayload(t *testing.T) {
	t.Parallel()

	if got := commandKind(&agentv1.ControlMessage{}); got != "" {
		t.Fatalf("commandKind = %q, want empty", got)
	}
}
