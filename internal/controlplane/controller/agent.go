package controller

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

var (
	ErrInvalidSessionID = errors.New("session_id is required")
	ErrInvalidHostname  = errors.New("hostname is required")
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionNotActive = errors.New("session is not active")
	ErrCommandNotFound  = errors.New("command not found")
	ErrCommandRejected  = errors.New("command queue is full")
)

type AgentSessionManagerConfig struct {
	Now         func() time.Time
	StaleAfter  time.Duration
	PingTimeout time.Duration
}

type AgentSessionStream struct {
	Commands chan AgentCommand
	Done     <-chan struct{}
}

type AgentCommand struct {
	SessionID string
	CommandID string
	IssuedAt  time.Time
	Kind      AgentCommandKind
}

type AgentCommandKind string

const AgentCommandKindPing AgentCommandKind = "ping"

type AgentPingResult struct {
	SessionID   string
	CommandID   string
	CompletedAt time.Time
	RoundTripMS int64
}

type AgentSessionSnapshot struct {
	SessionID       string
	TenantID        uuid.UUID
	TokenID         uuid.UUID
	TokenLabel      string
	Hostname        string
	AgentVersion    string
	ConnectedAt     time.Time
	LastHeartbeatAt time.Time
	DisconnectedAt  *time.Time
	Status          string
}

type AgentSessionManager struct {
	mu          sync.Mutex
	now         func() time.Time
	staleAfter  time.Duration
	pingTimeout time.Duration
	byToken     map[uuid.UUID]*agentSession
	byID        map[string]*agentSession
}

type agentSession struct {
	snapshot AgentSessionSnapshot
	active   bool
	commands chan AgentCommand
	done     chan struct{}
	pending  map[string]chan AgentPingResult
}

func NewAgentSessionManager(
	cfg AgentSessionManagerConfig,
) *AgentSessionManager {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	staleAfter := cfg.StaleAfter
	if staleAfter <= 0 {
		staleAfter = 25 * time.Second
	}
	pingTimeout := cfg.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = 5 * time.Second
	}
	return &AgentSessionManager{
		now:         now,
		staleAfter:  staleAfter,
		pingTimeout: pingTimeout,
		byToken:     make(map[uuid.UUID]*agentSession),
		byID:        make(map[string]*agentSession),
	}
}

func (m *AgentSessionManager) RegisterSession(
	token model.TenantToken,
	sessionID, hostname, agentVersion string,
) (AgentSessionStream, error) {
	if sessionID == "" {
		return AgentSessionStream{}, ErrInvalidSessionID
	}
	if hostname == "" {
		return AgentSessionStream{}, ErrInvalidHostname
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.byToken[token.ID]; ok {
		m.closeSessionLocked(existing, now)
		if existing.snapshot.SessionID != sessionID {
			delete(m.byID, existing.snapshot.SessionID)
		}
	}

	session := &agentSession{
		snapshot: AgentSessionSnapshot{
			SessionID:       sessionID,
			TenantID:        token.TenantID,
			TokenID:         token.ID,
			TokenLabel:      token.Label,
			Hostname:        hostname,
			AgentVersion:    agentVersion,
			ConnectedAt:     now,
			LastHeartbeatAt: now,
			Status:          "online",
		},
		active:   true,
		commands: make(chan AgentCommand, 16),
		done:     make(chan struct{}),
		pending:  make(map[string]chan AgentPingResult),
	}

	m.byToken[token.ID] = session
	m.byID[sessionID] = session

	return AgentSessionStream{
		Commands: session.commands,
		Done:     session.done,
	}, nil
}

func (m *AgentSessionManager) MarkHeartbeat(
	tokenID uuid.UUID,
	sessionID string,
	at time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := m.sessionLocked(tokenID, sessionID)
	if err != nil {
		return err
	}

	session.snapshot.LastHeartbeatAt = at.UTC()
	return nil
}

func (m *AgentSessionManager) SubmitPingResult(
	tokenID uuid.UUID,
	sessionID, commandID string,
	completedAt time.Time,
	roundTripMS int64,
) error {
	var waiter chan AgentPingResult

	m.mu.Lock()
	session, err := m.sessionLocked(tokenID, sessionID)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	waiter, ok := session.pending[commandID]
	if !ok {
		m.mu.Unlock()
		return ErrCommandNotFound
	}
	delete(session.pending, commandID)
	m.mu.Unlock()

	waiter <- AgentPingResult{
		SessionID:   sessionID,
		CommandID:   commandID,
		CompletedAt: completedAt.UTC(),
		RoundTripMS: roundTripMS,
	}
	close(waiter)
	return nil
}

func (m *AgentSessionManager) Ping(
	ctx context.Context,
	sessionID string,
) (AgentPingResult, error) {
	if deadline, ok := ctx.Deadline(); !ok ||
		time.Until(deadline) > m.pingTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.pingTimeout)
		defer cancel()
	}

	command := AgentCommand{
		SessionID: sessionID,
		CommandID: uuid.NewString(),
		IssuedAt:  m.now().UTC(),
		Kind:      AgentCommandKindPing,
	}
	waiter := make(chan AgentPingResult, 1)

	m.mu.Lock()
	session, ok := m.byID[sessionID]
	switch {
	case !ok:
		m.mu.Unlock()
		return AgentPingResult{}, ErrSessionNotFound
	case !m.isActiveLocked(session):
		m.mu.Unlock()
		return AgentPingResult{}, ErrSessionNotActive
	}

	session.pending[command.CommandID] = waiter
	select {
	case session.commands <- command:
	default:
		delete(session.pending, command.CommandID)
		m.mu.Unlock()
		return AgentPingResult{}, ErrCommandRejected
	}
	m.mu.Unlock()

	select {
	case result, ok := <-waiter:
		if !ok {
			return AgentPingResult{}, ErrSessionNotActive
		}
		return result, nil
	case <-ctx.Done():
		m.mu.Lock()
		if current, ok := m.byID[sessionID]; ok && current == session {
			delete(current.pending, command.CommandID)
		}
		m.mu.Unlock()
		return AgentPingResult{}, ctx.Err()
	}
}

func (m *AgentSessionManager) DisconnectSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.byID[sessionID]
	if !ok {
		return
	}
	m.closeSessionLocked(session, m.now().UTC())
}

func (m *AgentSessionManager) ListSessions() []AgentSessionSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now().UTC()
	out := make([]AgentSessionSnapshot, 0, len(m.byToken))
	for _, session := range m.byToken {
		snapshot := session.snapshot
		if session.active && now.Sub(snapshot.LastHeartbeatAt) <= m.staleAfter {
			snapshot.Status = "online"
		} else {
			snapshot.Status = "offline"
		}
		out = append(out, snapshot)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ConnectedAt.After(out[j].ConnectedAt)
	})
	return out
}

func (m *AgentSessionManager) sessionLocked(
	tokenID uuid.UUID,
	sessionID string,
) (*agentSession, error) {
	session, ok := m.byID[sessionID]
	if !ok || session.snapshot.TokenID != tokenID {
		return nil, ErrSessionNotFound
	}
	if !m.isActiveLocked(session) {
		return nil, ErrSessionNotActive
	}
	return session, nil
}

func (m *AgentSessionManager) isActiveLocked(session *agentSession) bool {
	return session.active &&
		m.now().UTC().Sub(session.snapshot.LastHeartbeatAt) <= m.staleAfter
}

func (m *AgentSessionManager) closeSessionLocked(
	session *agentSession,
	disconnectedAt time.Time,
) {
	if session.active {
		session.active = false
		if session.snapshot.DisconnectedAt == nil {
			ts := disconnectedAt.UTC()
			session.snapshot.DisconnectedAt = &ts
		}
	}

	select {
	case <-session.done:
	default:
		close(session.done)
	}

	for commandID, waiter := range session.pending {
		delete(session.pending, commandID)
		close(waiter)
	}
}
