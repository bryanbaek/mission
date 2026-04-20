package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/bryanbaek/mission/gen/go/agent/v1"
	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

func routeRequest(
	method string,
	target string,
	sessionID string,
) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("sessionID", sessionID)
	return req.WithContext(context.WithValue(
		req.Context(),
		chi.RouteCtxKey,
		routeCtx,
	))
}

func TestAgentHandlerOpenCommandStream(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	handler := NewAgentHandler(manager)
	agent := auth.Agent{
		TokenID:  uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-1",
	}

	path, svc := agentv1connect.NewAgentServiceHandler(handler)
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(auth.WithAgent(r.Context(), agent))
			svc.ServeHTTP(w, r)
		},
	))
	defer server.Close()

	client := agentv1connect.NewAgentServiceClient(http.DefaultClient, server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.OpenCommandStream(
		ctx,
		connect.NewRequest(&agentv1.OpenCommandStreamRequest{
			SessionId:    "session-1",
			Hostname:     "host-a",
			AgentVersion: "v1",
		}),
	)
	if err != nil {
		t.Fatalf("OpenCommandStream returned error: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, pingErr := manager.Ping(context.Background(), "session-1")
		errCh <- pingErr
	}()

	if !stream.Receive() {
		t.Fatalf("Receive returned false: %v", stream.Err())
	}

	msg := stream.Msg()
	if msg.GetSessionId() != "session-1" {
		t.Fatalf("SessionId = %q, want session-1", msg.GetSessionId())
	}
	if msg.GetPing() != nil {
		t.Fatal("expected initial stream-open ack without payload")
	}

	if !stream.Receive() {
		t.Fatalf("Receive returned false waiting for ping: %v", stream.Err())
	}

	msg = stream.Msg()
	if msg.GetCommandId() == "" {
		t.Fatal("expected command ID on streamed command")
	}

	cancel()
	manager.DisconnectSession("session-1")

	select {
	case pingErr := <-errCh:
		if !errors.Is(pingErr, controller.ErrSessionNotActive) {
			t.Fatalf("ping err = %v, want ErrSessionNotActive", pingErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ping result")
	}

	if path != "/agent.v1.AgentService/" {
		t.Fatalf("path = %q, want /agent.v1.AgentService/", path)
	}
}

func TestAgentHandlerHeartbeatAndSubmitResult(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	handler := NewAgentHandler(manager)
	token := model.TenantToken{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-1",
	}
	stream, err := manager.RegisterSession(token, "session-1", "host-a", "v1")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	agentCtx := auth.WithAgent(context.Background(), auth.Agent{
		TokenID:  token.ID,
		TenantID: token.TenantID,
		Label:    token.Label,
	})

	sentAt := now.Add(2 * time.Second)
	_, err = handler.Heartbeat(
		agentCtx,
		connect.NewRequest(&agentv1.HeartbeatRequest{
			SessionId: "session-1",
			SentAt:    timestamppb.New(sentAt),
		}),
	)
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	snapshots := manager.ListSessions()
	if got := snapshots[0].LastHeartbeatAt; !got.Equal(sentAt) {
		t.Fatalf("LastHeartbeatAt = %s, want %s", got, sentAt)
	}

	resultCh := make(chan controller.AgentPingResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, pingErr := manager.Ping(context.Background(), "session-1")
		if pingErr != nil {
			errCh <- pingErr
			return
		}
		resultCh <- result
	}()

	command := <-stream.Commands
	completedAt := now.Add(3 * time.Second)
	_, err = handler.SubmitCommandResult(
		agentCtx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   "session-1",
			CommandId:   command.CommandID,
			CompletedAt: timestamppb.New(completedAt),
			Result: &agentv1.SubmitCommandResultRequest_Ping{
				Ping: &agentv1.PingResult{RoundTripMs: 24},
			},
		}),
	)
	if err != nil {
		t.Fatalf("SubmitCommandResult returned error: %v", err)
	}

	select {
	case pingErr := <-errCh:
		t.Fatalf("Ping returned error: %v", pingErr)
	case result := <-resultCh:
		if result.RoundTripMS != 24 {
			t.Fatalf("RoundTripMS = %d, want 24", result.RoundTripMS)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ping result")
	}

	_, err = handler.SubmitCommandResult(
		agentCtx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId: "session-1",
			CommandId: "missing",
		}),
	)
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) ||
		connectErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("err = %v, want invalid argument", err)
	}
}

func TestAgentHandlerExecuteQueryResult(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	tenantID := uuid.New()
	manager := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	handler := NewAgentHandler(manager)
	token := model.TenantToken{
		ID:       uuid.New(),
		TenantID: tenantID,
		Label:    "edge-1",
	}
	stream, err := manager.RegisterSession(token, "session-1", "host-a", "v1")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	agentCtx := auth.WithAgent(context.Background(), auth.Agent{
		TokenID:  token.ID,
		TenantID: token.TenantID,
		Label:    token.Label,
	})

	resultCh := make(chan controller.AgentExecuteQueryResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, queryErr := manager.ExecuteQuery(
			context.Background(),
			tenantID,
			"SELECT 1",
		)
		if queryErr != nil {
			errCh <- queryErr
			return
		}
		resultCh <- result
	}()

	command := <-stream.Commands
	if command.Kind != controller.AgentCommandKindExecuteQuery {
		t.Fatalf("Kind = %q, want execute_query", command.Kind)
	}

	_, err = handler.SubmitCommandResult(
		agentCtx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   "session-1",
			CommandId:   command.CommandID,
			CompletedAt: timestamppb.New(now.Add(time.Second)),
			Result: &agentv1.SubmitCommandResultRequest_ExecuteQuery{
				ExecuteQuery: &agentv1.ExecuteQueryResult{
					Columns:      []string{"1"},
					Rows:         []*agentv1.ExecuteQueryRow{{Values: map[string]*structpb.Value{"1": structpb.NewNumberValue(1)}}},
					ElapsedMs:    14,
					DatabaseUser: "mission_ro@%",
					DatabaseName: "mission_app",
				},
			},
		}),
	)
	if err != nil {
		t.Fatalf("SubmitCommandResult returned error: %v", err)
	}

	select {
	case queryErr := <-errCh:
		t.Fatalf("ExecuteQuery returned error: %v", queryErr)
	case result := <-resultCh:
		if result.ElapsedMS != 14 {
			t.Fatalf("ElapsedMS = %d, want 14", result.ElapsedMS)
		}
		if got := result.Rows[0]["1"]; got != float64(1) {
			t.Fatalf("row value = %#v, want 1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for query result")
	}
}

func TestAgentHandlerIntrospectSchemaResult(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	tenantID := uuid.New()
	manager := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	handler := NewAgentHandler(manager)
	token := model.TenantToken{
		ID:       uuid.New(),
		TenantID: tenantID,
		Label:    "edge-1",
	}
	stream, err := manager.RegisterSession(token, "session-1", "host-a", "v1")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	agentCtx := auth.WithAgent(context.Background(), auth.Agent{
		TokenID:  token.ID,
		TenantID: token.TenantID,
		Label:    token.Label,
	})

	resultCh := make(chan controller.AgentIntrospectSchemaResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, captureErr := manager.IntrospectSchema(
			context.Background(),
			tenantID,
		)
		if captureErr != nil {
			errCh <- captureErr
			return
		}
		resultCh <- result
	}()

	command := <-stream.Commands
	if command.Kind != controller.AgentCommandKindIntrospectSchema {
		t.Fatalf("Kind = %q, want introspect_schema", command.Kind)
	}

	_, err = handler.SubmitCommandResult(
		agentCtx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   "session-1",
			CommandId:   command.CommandID,
			CompletedAt: timestamppb.New(now.Add(time.Second)),
			Result: &agentv1.SubmitCommandResultRequest_IntrospectSchema{
				IntrospectSchema: &agentv1.IntrospectSchemaResult{
					Schema: &agentv1.SchemaBlob{
						DatabaseName: "mission_app",
						Tables: []*agentv1.SchemaTable{{
							TableSchema: "mission_app",
							TableName:   "customers",
							TableType:   "BASE TABLE",
						}},
					},
					ElapsedMs:    19,
					DatabaseUser: "mission_ro@%",
					DatabaseName: "mission_app",
				},
			},
		}),
	)
	if err != nil {
		t.Fatalf("SubmitCommandResult returned error: %v", err)
	}

	select {
	case captureErr := <-errCh:
		t.Fatalf("IntrospectSchema returned error: %v", captureErr)
	case result := <-resultCh:
		if result.ElapsedMS != 19 {
			t.Fatalf("ElapsedMS = %d, want 19", result.ElapsedMS)
		}
		if got := result.Schema.Tables[0].TableName; got != "customers" {
			t.Fatalf("table name = %q, want customers", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for schema result")
	}
}

func TestAgentHandlerHelpersAndDebugEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	token := model.TenantToken{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-1",
	}
	stream, err := manager.RegisterSession(token, "session-1", "host-a", "v1")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	message := commandToProto(controller.AgentCommand{
		SessionID: "session-1",
		CommandID: "command-1",
		IssuedAt:  now,
		Kind:      controller.AgentCommandKindPing,
	})
	if message.GetPing() == nil {
		t.Fatal("expected ping payload")
	}

	queryMessage := commandToProto(controller.AgentCommand{
		SessionID: "session-1",
		CommandID: "command-2",
		IssuedAt:  now,
		Kind:      controller.AgentCommandKindExecuteQuery,
		SQL:       "SELECT 1",
	})
	if queryMessage.GetExecuteQuery().GetSql() != "SELECT 1" {
		t.Fatalf("query sql = %q, want SELECT 1", queryMessage.GetExecuteQuery().GetSql())
	}

	schemaMessage := commandToProto(controller.AgentCommand{
		SessionID: "session-1",
		CommandID: "command-3",
		IssuedAt:  now,
		Kind:      controller.AgentCommandKindIntrospectSchema,
	})
	if schemaMessage.GetIntrospectSchema() == nil {
		t.Fatal("expected introspect_schema payload")
	}

	var connectErr *connect.Error
	if err := connectErrorForSession(controller.ErrInvalidHostname); !errors.As(err, &connectErr) {
		t.Fatalf("err = %v, want connect error", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid argument", connectErr.Code())
	}
	if err := connectErrorForSession(repository.ErrNotFound); !errors.As(err, &connectErr) {
		t.Fatalf("err = %v, want connect error", err)
	}
	if connectErr.Code() != connect.CodeInternal {
		t.Fatalf("code = %v, want internal", connectErr.Code())
	}
	if err := connectErrorForSession(controller.ErrTenantNotConnected); !errors.As(err, &connectErr) {
		t.Fatalf("err = %v, want connect error", err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want failed precondition", connectErr.Code())
	}

	debugHandler := NewAgentDebugHandler(manager)

	rec := httptest.NewRecorder()
	debugHandler.ListSessions(rec, httptest.NewRequest(http.MethodGet, "/agents", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var listBody map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := len(listBody["agents"]); got != 1 {
		t.Fatalf("agent count = %d, want 1", got)
	}

	errCh := make(chan error, 1)
	go func() {
		command := <-stream.Commands
		errCh <- manager.SubmitPingResult(
			token.ID,
			"session-1",
			command.CommandID,
			now.Add(time.Second),
			99,
		)
	}()

	pingRec := httptest.NewRecorder()
	debugHandler.PingSession(
		pingRec,
		routeRequest(
			http.MethodPost,
			"/api/debug/agents/session-1/ping",
			"session-1",
		),
	)
	if pingRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", pingRec.Code)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("SubmitPingResult returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for debug ping")
	}

	notFoundRec := httptest.NewRecorder()
	debugHandler.PingSession(
		notFoundRec,
		routeRequest(http.MethodPost, "/api/debug/agents/missing/ping", "missing"),
	)
	if notFoundRec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", notFoundRec.Code)
	}
}
