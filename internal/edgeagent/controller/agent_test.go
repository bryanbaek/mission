package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
	"github.com/bryanbaek/mission/internal/queryerror"
)

type fakeCommandStream struct {
	messages []ControlMessage
	err      error
	index    int
}

func (s *fakeCommandStream) Receive() bool {
	if s.index >= len(s.messages) {
		return false
	}
	s.index++
	return true
}

func (s *fakeCommandStream) Message() ControlMessage {
	return s.messages[s.index-1]
}

func (s *fakeCommandStream) Err() error {
	return s.err
}

type fakeControlPlaneClient struct {
	openFn         func(context.Context, OpenCommandStreamRequest) (CommandStream, error)
	beatFn         func(context.Context, HeartbeatRequest) error
	submitFn       func(context.Context, SubmitPingResultRequest) error
	submitQueryFn  func(context.Context, SubmitExecuteQueryResultRequest) error
	submitSchemaFn func(context.Context, SubmitIntrospectSchemaResultRequest) error
	submitConfigFn func(context.Context, SubmitConfigureDatabaseResultRequest) error
}

func (c fakeControlPlaneClient) OpenCommandStream(
	ctx context.Context,
	req OpenCommandStreamRequest,
) (CommandStream, error) {
	if c.openFn == nil {
		return nil, nil
	}
	return c.openFn(ctx, req)
}

func (c fakeControlPlaneClient) Heartbeat(
	ctx context.Context,
	req HeartbeatRequest,
) error {
	if c.beatFn == nil {
		return nil
	}
	return c.beatFn(ctx, req)
}

func (c fakeControlPlaneClient) SubmitPingResult(
	ctx context.Context,
	req SubmitPingResultRequest,
) error {
	if c.submitFn == nil {
		return nil
	}
	return c.submitFn(ctx, req)
}

func (c fakeControlPlaneClient) SubmitExecuteQueryResult(
	ctx context.Context,
	req SubmitExecuteQueryResultRequest,
) error {
	if c.submitQueryFn == nil {
		return nil
	}
	return c.submitQueryFn(ctx, req)
}

func (c fakeControlPlaneClient) SubmitIntrospectSchemaResult(
	ctx context.Context,
	req SubmitIntrospectSchemaResultRequest,
) error {
	if c.submitSchemaFn == nil {
		return nil
	}
	return c.submitSchemaFn(ctx, req)
}

func (c fakeControlPlaneClient) SubmitConfigureDatabaseResult(
	ctx context.Context,
	req SubmitConfigureDatabaseResultRequest,
) error {
	if c.submitConfigFn == nil {
		return nil
	}
	return c.submitConfigFn(ctx, req)
}

type fakeQueryExecutor struct {
	executeFn func(context.Context, string) (QueryResult, error)
}

func (e fakeQueryExecutor) ExecuteQuery(
	ctx context.Context,
	sql string,
) (QueryResult, error) {
	return e.executeFn(ctx, sql)
}

type fakeSchemaIntrospector struct {
	introspectFn func(context.Context) (introspect.SchemaBlob, int64, string, string, error)
}

func (i fakeSchemaIntrospector) IntrospectSchema(
	ctx context.Context,
) (introspect.SchemaBlob, int64, string, string, error) {
	return i.introspectFn(ctx)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewAgentService(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	service, err := NewAgentService(
		fakeControlPlaneClient{},
		AgentServiceConfig{
			Hostname: "host-a",
			Now:      func() time.Time { return now },
			Logger:   discardLogger(),
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}
	if service.hostname != "host-a" {
		t.Fatalf("hostname = %q, want host-a", service.hostname)
	}
	if service.sessionID == "" {
		t.Fatal("expected generated session ID")
	}
	if service.startedAt != now {
		t.Fatalf("startedAt = %s, want %s", service.startedAt, now)
	}

	_, err = NewAgentService(
		nil,
		AgentServiceConfig{Hostname: "host-a"},
	)
	if err == nil {
		t.Fatal("NewAgentService returned nil error for nil client")
	}

	_, err = NewAgentService(
		fakeControlPlaneClient{},
		AgentServiceConfig{
			Hostname:      "host-a",
			ReconnectBase: 5 * time.Second,
			ReconnectMax:  time.Second,
		},
	)
	if err == nil {
		t.Fatal("NewAgentService returned nil error for invalid reconnect bounds")
	}
}

func TestAgentServiceHandleCommand(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	var submitted SubmitPingResultRequest
	service, err := NewAgentService(
		fakeControlPlaneClient{
			openFn: func(context.Context, OpenCommandStreamRequest) (CommandStream, error) {
				return nil, nil
			},
			beatFn: func(context.Context, HeartbeatRequest) error { return nil },
			submitFn: func(
				_ context.Context,
				req SubmitPingResultRequest,
			) error {
				submitted = req
				return nil
			},
			submitQueryFn: func(context.Context, SubmitExecuteQueryResultRequest) error {
				return nil
			},
		},
		AgentServiceConfig{
			Hostname: "host-a",
			Now: func() time.Time {
				return now
			},
			Logger: discardLogger(),
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	err = service.handleCommand(context.Background(), ControlMessage{
		SessionID: "session-1",
		CommandID: "command-1",
		IssuedAt:  now.Add(-2 * time.Second),
		Kind:      CommandKindPing,
	})
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}
	if submitted.CommandID != "command-1" {
		t.Fatalf("CommandID = %q, want command-1", submitted.CommandID)
	}
	if submitted.RoundTripMS != 2000 {
		t.Fatalf("RoundTripMS = %d, want 2000", submitted.RoundTripMS)
	}

	err = service.handleCommand(context.Background(), ControlMessage{Kind: "noop"})
	if err == nil {
		t.Fatal("handleCommand returned nil error for unsupported command")
	}
}

func TestAgentServiceRunHeartbeatLoop(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	var beats []HeartbeatRequest
	service, err := NewAgentService(
		fakeControlPlaneClient{
			openFn: func(context.Context, OpenCommandStreamRequest) (CommandStream, error) {
				return nil, nil
			},
			beatFn: func(_ context.Context, req HeartbeatRequest) error {
				beats = append(beats, req)
				return nil
			},
			submitFn: func(context.Context, SubmitPingResultRequest) error { return nil },
			submitQueryFn: func(context.Context, SubmitExecuteQueryResultRequest) error {
				return nil
			},
		},
		AgentServiceConfig{
			Hostname:          "host-a",
			HeartbeatInterval: 10 * time.Millisecond,
			Now: func() time.Time {
				return now
			},
			Logger: discardLogger(),
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- service.runHeartbeatLoop(ctx)
	}()

	for len(beats) < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("runHeartbeatLoop returned error: %v", err)
	}
}

func TestAgentServiceRunSessionAndRun(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	sleepCalls := 0
	stream := &fakeCommandStream{
		messages: []ControlMessage{{
			SessionID: "session-1",
			CommandID: "command-1",
			IssuedAt:  now,
			Kind:      CommandKindPing,
		}},
	}
	client := fakeControlPlaneClient{
		openFn: func(
			_ context.Context,
			_ OpenCommandStreamRequest,
		) (CommandStream, error) {
			return stream, nil
		},
		beatFn: func(context.Context, HeartbeatRequest) error { return nil },
		submitFn: func(
			_ context.Context,
			_ SubmitPingResultRequest,
		) error {
			return nil
		},
		submitQueryFn: func(context.Context, SubmitExecuteQueryResultRequest) error {
			return nil
		},
	}

	service, err := NewAgentService(
		client,
		AgentServiceConfig{
			Hostname:          "host-a",
			HeartbeatInterval: time.Hour,
			Logger:            discardLogger(),
			Now: func() time.Time {
				return now
			},
			Sleep: func(ctx context.Context, _ time.Duration) error {
				sleepCalls++
				return context.Canceled
			},
			Rand: rand.New(rand.NewSource(1)),
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	connected, err := service.runSession(context.Background())
	if !connected {
		t.Fatal("runSession should report connected")
	}
	if err == nil {
		t.Fatal("runSession returned nil error for closed stream")
	}

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if sleepCalls != 1 {
		t.Fatalf("sleepCalls = %d, want 1", sleepCalls)
	}
}

func TestJitterAndSleepContext(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	service, err := NewAgentService(
		fakeControlPlaneClient{},
		AgentServiceConfig{
			Hostname: "host-a",
			Now: func() time.Time {
				return now
			},
			Logger: discardLogger(),
			Rand:   rand.New(rand.NewSource(1)),
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	if got := service.jitter(0); got != 0 {
		t.Fatalf("jitter(0) = %s, want 0", got)
	}
	if got := service.jitter(50 * time.Millisecond); got < 50*time.Millisecond {
		t.Fatalf("jitter = %s, want >= 50ms", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context canceled", err)
	}
}

func TestAgentServiceHandleExecuteQuery(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	var submitted SubmitExecuteQueryResultRequest
	service, err := NewAgentService(
		fakeControlPlaneClient{
			openFn: func(context.Context, OpenCommandStreamRequest) (CommandStream, error) {
				return nil, nil
			},
			beatFn:   func(context.Context, HeartbeatRequest) error { return nil },
			submitFn: func(context.Context, SubmitPingResultRequest) error { return nil },
			submitQueryFn: func(
				_ context.Context,
				req SubmitExecuteQueryResultRequest,
			) error {
				submitted = req
				return nil
			},
		},
		AgentServiceConfig{
			Hostname: "host-a",
			Now: func() time.Time {
				return now
			},
			Logger: discardLogger(),
			QueryExecutor: fakeQueryExecutor{
				executeFn: func(_ context.Context, sql string) (QueryResult, error) {
					if sql != "SELECT 1" {
						t.Fatalf("sql = %q, want SELECT 1", sql)
					}
					return QueryResult{
						Columns:      []string{"1"},
						Rows:         []map[string]any{{"1": int64(1)}},
						ElapsedMS:    12,
						DatabaseUser: "mission_ro@%",
						DatabaseName: "mission_app",
					}, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	err = service.handleCommand(context.Background(), ControlMessage{
		SessionID: "session-1",
		CommandID: "command-1",
		Kind:      CommandKindExecuteQuery,
		SQL:       "SELECT 1",
	})
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}
	if submitted.Error != "" {
		t.Fatalf("submitted error = %q, want empty", submitted.Error)
	}
	if len(submitted.Rows) != 1 || submitted.Rows[0]["1"] != int64(1) {
		t.Fatalf("submitted rows = %#v, want one row with 1", submitted.Rows)
	}
	if submitted.DatabaseName != "mission_app" {
		t.Fatalf("DatabaseName = %q, want mission_app", submitted.DatabaseName)
	}
}

func TestAgentServiceHandleExecuteQueryError(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	var submitted SubmitExecuteQueryResultRequest
	service, err := NewAgentService(
		fakeControlPlaneClient{
			openFn: func(context.Context, OpenCommandStreamRequest) (CommandStream, error) {
				return nil, nil
			},
			beatFn:   func(context.Context, HeartbeatRequest) error { return nil },
			submitFn: func(context.Context, SubmitPingResultRequest) error { return nil },
			submitQueryFn: func(
				_ context.Context,
				req SubmitExecuteQueryResultRequest,
			) error {
				submitted = req
				return nil
			},
		},
		AgentServiceConfig{
			Hostname: "host-a",
			Now: func() time.Time {
				return now
			},
			Logger: discardLogger(),
			QueryExecutor: fakeQueryExecutor{
				executeFn: func(context.Context, string) (QueryResult, error) {
					return QueryResult{}, errors.New("syntax error")
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	err = service.handleCommand(context.Background(), ControlMessage{
		SessionID: "session-1",
		CommandID: "command-1",
		Kind:      CommandKindExecuteQuery,
		SQL:       "SELECT nope",
	})
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}
	if submitted.Error != "syntax error" {
		t.Fatalf("submitted error = %q, want syntax error", submitted.Error)
	}
}

func TestAgentServiceHandleExecuteQueryPermissionDenied(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	var submitted SubmitExecuteQueryResultRequest
	var logs bytes.Buffer
	service, err := NewAgentService(
		fakeControlPlaneClient{
			submitQueryFn: func(
				_ context.Context,
				req SubmitExecuteQueryResultRequest,
			) error {
				submitted = req
				return nil
			},
		},
		AgentServiceConfig{
			Hostname: "host-a",
			Now: func() time.Time {
				return now
			},
			Logger: slog.New(slog.NewTextHandler(&logs, nil)),
			QueryExecutor: fakeQueryExecutor{
				executeFn: func(context.Context, string) (QueryResult, error) {
					return QueryResult{}, queryerror.PermissionDenied(
						"SQL comments are not allowed",
						[]string{"comment"},
					)
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	err = service.handleCommand(context.Background(), ControlMessage{
		SessionID: "session-1",
		CommandID: "command-1",
		Kind:      CommandKindExecuteQuery,
		SQL:       "SELECT 1 # injected",
	})
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	if submitted.ErrorCode != queryerror.CodePermissionDenied {
		t.Fatalf(
			"ErrorCode = %q, want %q",
			submitted.ErrorCode,
			queryerror.CodePermissionDenied,
		)
	}
	if submitted.ErrorReason != "SQL comments are not allowed" {
		t.Fatalf("ErrorReason = %q, want SQL comments are not allowed", submitted.ErrorReason)
	}
	if len(submitted.BlockedConstructs) != 1 || submitted.BlockedConstructs[0] != "comment" {
		t.Fatalf("BlockedConstructs = %#v, want [comment]", submitted.BlockedConstructs)
	}
	if submitted.Error != "SQL comments are not allowed" {
		t.Fatalf("Error = %q, want SQL comments are not allowed", submitted.Error)
	}

	logOutput := logs.String()
	for _, want := range []string{
		"query rejected by sqlguard",
		"session_id=session-1",
		"command_id=command-1",
		"reason=\"SQL comments are not allowed\"",
		"blocked_constructs=[comment]",
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("log output %q missing %q", logOutput, want)
		}
	}
}

func TestAgentServiceHandleIntrospectSchema(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	var submitted SubmitIntrospectSchemaResultRequest
	service, err := NewAgentService(
		fakeControlPlaneClient{
			submitSchemaFn: func(
				_ context.Context,
				req SubmitIntrospectSchemaResultRequest,
			) error {
				submitted = req
				return nil
			},
		},
		AgentServiceConfig{
			Hostname: "host-a",
			Now: func() time.Time {
				return now
			},
			Logger: discardLogger(),
			SchemaIntrospector: fakeSchemaIntrospector{
				introspectFn: func(context.Context) (introspect.SchemaBlob, int64, string, string, error) {
					return introspect.SchemaBlob{
						DatabaseName: "mission_app",
						Tables: []introspect.SchemaTable{{
							TableSchema: "mission_app",
							TableName:   "customers",
							TableType:   "BASE TABLE",
						}},
					}, 21, "mission_ro@%", "mission_app", nil
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	err = service.handleCommand(context.Background(), ControlMessage{
		SessionID: "session-1",
		CommandID: "command-1",
		Kind:      CommandKindIntrospectSchema,
	})
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}
	if submitted.DatabaseName != "mission_app" {
		t.Fatalf("DatabaseName = %q, want mission_app", submitted.DatabaseName)
	}
	if submitted.ElapsedMS != 21 {
		t.Fatalf("ElapsedMS = %d, want 21", submitted.ElapsedMS)
	}
	if len(submitted.Schema.Tables) != 1 {
		t.Fatalf("table count = %d, want 1", len(submitted.Schema.Tables))
	}
}
