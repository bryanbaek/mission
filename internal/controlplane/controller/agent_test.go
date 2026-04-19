package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

func TestNewAgentSessionManagerDefaults(t *testing.T) {
	t.Parallel()

	manager := NewAgentSessionManager(AgentSessionManagerConfig{})

	if manager.now == nil {
		t.Fatal("expected default clock")
	}
	if manager.staleAfter != 25*time.Second {
		t.Fatalf("staleAfter = %s, want 25s", manager.staleAfter)
	}
	if manager.pingTimeout != 5*time.Second {
		t.Fatalf("pingTimeout = %s, want 5s", manager.pingTimeout)
	}
}

func TestRegisterSessionValidatesAndReplacesTokenSession(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := NewAgentSessionManager(AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	token := model.TenantToken{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-1",
	}

	_, err := manager.RegisterSession(token, "", "host-a", "v1")
	if !errors.Is(err, ErrInvalidSessionID) {
		t.Fatalf("err = %v, want ErrInvalidSessionID", err)
	}

	_, err = manager.RegisterSession(token, "session-1", "", "v1")
	if !errors.Is(err, ErrInvalidHostname) {
		t.Fatalf("err = %v, want ErrInvalidHostname", err)
	}

	first, err := manager.RegisterSession(token, "session-1", "host-a", "v1")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	now = now.Add(time.Second)
	second, err := manager.RegisterSession(token, "session-2", "host-b", "v2")
	if err != nil {
		t.Fatalf("second RegisterSession returned error: %v", err)
	}

	select {
	case <-first.Done:
	default:
		t.Fatal("expected first session stream to close")
	}
	select {
	case <-second.Done:
		t.Fatal("second session should remain open")
	default:
	}

	snapshots := manager.ListSessions()
	if len(snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(snapshots))
	}
	if snapshots[0].SessionID != "session-2" {
		t.Fatalf("SessionID = %q, want session-2", snapshots[0].SessionID)
	}
	if snapshots[0].Hostname != "host-b" {
		t.Fatalf("Hostname = %q, want host-b", snapshots[0].Hostname)
	}
}

func TestAgentSessionManagerPingRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := NewAgentSessionManager(AgentSessionManagerConfig{
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

	resultCh := make(chan AgentPingResult, 1)
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
	if command.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want session-1", command.SessionID)
	}
	if command.CommandID == "" {
		t.Fatal("expected command ID")
	}
	if command.Kind != AgentCommandKindPing {
		t.Fatalf("Kind = %q, want ping", command.Kind)
	}

	completedAt := now.Add(3 * time.Second)
	if err := manager.SubmitPingResult(
		token.ID,
		"session-1",
		command.CommandID,
		completedAt,
		42,
	); err != nil {
		t.Fatalf("SubmitPingResult returned error: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("Ping returned error: %v", err)
	case result := <-resultCh:
		if result.CommandID != command.CommandID {
			t.Fatalf("CommandID = %q, want %q", result.CommandID, command.CommandID)
		}
		if result.CompletedAt != completedAt {
			t.Fatalf("CompletedAt = %s, want %s", result.CompletedAt, completedAt)
		}
		if result.RoundTripMS != 42 {
			t.Fatalf("RoundTripMS = %d, want 42", result.RoundTripMS)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ping result")
	}
}

func TestAgentSessionManagerErrors(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := NewAgentSessionManager(AgentSessionManagerConfig{
		Now:         func() time.Time { return now },
		StaleAfter:  time.Second,
		PingTimeout: 50 * time.Millisecond,
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

	if _, err := manager.Ping(context.Background(), "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}

	if err := manager.SubmitPingResult(
		token.ID,
		"session-1",
		"missing-command",
		now,
		1,
	); !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("err = %v, want ErrCommandNotFound", err)
	}

	now = now.Add(500 * time.Millisecond)
	if err := manager.MarkHeartbeat(token.ID, "session-1", now); err != nil {
		t.Fatalf("MarkHeartbeat returned error: %v", err)
	}

	now = now.Add(2 * time.Second)
	if _, err := manager.Ping(context.Background(), "session-1"); !errors.Is(err, ErrSessionNotActive) {
		t.Fatalf("err = %v, want ErrSessionNotActive", err)
	}

	now = now.Add(time.Second)
	stream, err = manager.RegisterSession(token, "session-2", "host-b", "v2")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}
	session := manager.byID["session-2"]
	for i := 0; i < cap(session.commands); i++ {
		session.commands <- AgentCommand{CommandID: uuid.NewString()}
	}
	if _, err := manager.Ping(context.Background(), "session-2"); !errors.Is(err, ErrCommandRejected) {
		t.Fatalf("err = %v, want ErrCommandRejected", err)
	}

	now = now.Add(time.Second)
	stream, err = manager.RegisterSession(token, "session-3", "host-c", "v3")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := manager.Ping(ctx, "session-3"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context deadline exceeded", err)
	}

	now = now.Add(time.Second)
	stream, err = manager.RegisterSession(token, "session-4", "host-d", "v4")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, pingErr := manager.Ping(context.Background(), "session-4")
		errCh <- pingErr
	}()
	<-stream.Commands
	manager.DisconnectSession("session-4")

	select {
	case pingErr := <-errCh:
		if !errors.Is(pingErr, ErrSessionNotActive) {
			t.Fatalf("err = %v, want ErrSessionNotActive", pingErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for disconnect result")
	}
}

func TestAgentSessionManagerListSessionsSortedAndOffline(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	manager := NewAgentSessionManager(AgentSessionManagerConfig{
		Now:        func() time.Time { return now },
		StaleAfter: time.Second,
	})

	firstToken := model.TenantToken{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-1",
	}
	secondToken := model.TenantToken{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-2",
	}

	if _, err := manager.RegisterSession(firstToken, "session-1", "host-a", "v1"); err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := manager.RegisterSession(secondToken, "session-2", "host-b", "v2"); err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	now = now.Add(2 * time.Second)
	snapshots := manager.ListSessions()
	if len(snapshots) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snapshots))
	}
	if snapshots[0].SessionID != "session-2" {
		t.Fatalf("first SessionID = %q, want session-2", snapshots[0].SessionID)
	}
	if snapshots[0].Status != "offline" {
		t.Fatalf("first status = %q, want offline", snapshots[0].Status)
	}
	if snapshots[1].SessionID != "session-1" {
		t.Fatalf("second SessionID = %q, want session-1", snapshots[1].SessionID)
	}
}
