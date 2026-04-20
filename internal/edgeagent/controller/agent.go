package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
)

type CommandKind string

const CommandKindPing CommandKind = "ping"

const CommandKindExecuteQuery CommandKind = "execute_query"

const CommandKindIntrospectSchema CommandKind = "introspect_schema"

type OpenCommandStreamRequest struct {
	SessionID    string
	Hostname     string
	AgentVersion string
	StartedAt    time.Time
}

type HeartbeatRequest struct {
	SessionID string
	SentAt    time.Time
}

type SubmitPingResultRequest struct {
	SessionID   string
	CommandID   string
	CompletedAt time.Time
	RoundTripMS int64
}

type SubmitExecuteQueryResultRequest struct {
	SessionID    string
	CommandID    string
	CompletedAt  time.Time
	Columns      []string
	Rows         []map[string]any
	ElapsedMS    int64
	DatabaseUser string
	DatabaseName string
	Error        string
}

type SubmitIntrospectSchemaResultRequest struct {
	SessionID    string
	CommandID    string
	CompletedAt  time.Time
	Schema       introspect.SchemaBlob
	ElapsedMS    int64
	DatabaseUser string
	DatabaseName string
	Error        string
}

type ControlMessage struct {
	SessionID string
	CommandID string
	IssuedAt  time.Time
	Kind      CommandKind
	SQL       string
}

type CommandStream interface {
	Receive() bool
	Message() ControlMessage
	Err() error
}

type ControlPlaneClient interface {
	OpenCommandStream(
		ctx context.Context,
		req OpenCommandStreamRequest,
	) (CommandStream, error)
	Heartbeat(ctx context.Context, req HeartbeatRequest) error
	SubmitPingResult(ctx context.Context, req SubmitPingResultRequest) error
	SubmitExecuteQueryResult(
		ctx context.Context,
		req SubmitExecuteQueryResultRequest,
	) error
	SubmitIntrospectSchemaResult(
		ctx context.Context,
		req SubmitIntrospectSchemaResultRequest,
	) error
}

type QueryResult struct {
	Columns      []string
	Rows         []map[string]any
	ElapsedMS    int64
	DatabaseUser string
	DatabaseName string
}

type QueryExecutor interface {
	ExecuteQuery(ctx context.Context, sql string) (QueryResult, error)
}

type SchemaIntrospector interface {
	IntrospectSchema(
		ctx context.Context,
	) (introspect.SchemaBlob, int64, string, string, error)
}

type AgentServiceConfig struct {
	SessionID          string
	Hostname           string
	AgentVersion       string
	StartedAt          time.Time
	HeartbeatInterval  time.Duration
	ReconnectBase      time.Duration
	ReconnectMax       time.Duration
	Logger             *slog.Logger
	Now                func() time.Time
	Sleep              func(context.Context, time.Duration) error
	Rand               *rand.Rand
	QueryExecutor      QueryExecutor
	SchemaIntrospector SchemaIntrospector
}

type AgentService struct {
	client             ControlPlaneClient
	sessionID          string
	hostname           string
	agentVersion       string
	startedAt          time.Time
	heartbeatInterval  time.Duration
	reconnectBase      time.Duration
	reconnectMax       time.Duration
	logger             *slog.Logger
	now                func() time.Time
	sleep              func(context.Context, time.Duration) error
	rand               *rand.Rand
	queryExecutor      QueryExecutor
	schemaIntrospector SchemaIntrospector
}

func NewAgentService(
	client ControlPlaneClient,
	cfg AgentServiceConfig,
) (*AgentService, error) {
	if client == nil {
		return nil, errors.New("client is required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	hostname := cfg.Hostname
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("resolve hostname: %w", err)
		}
	}
	sessionID := cfg.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	startedAt := cfg.StartedAt
	if startedAt.IsZero() {
		startedAt = now().UTC()
	}
	heartbeatInterval := cfg.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 10 * time.Second
	}
	reconnectBase := cfg.ReconnectBase
	if reconnectBase <= 0 {
		reconnectBase = time.Second
	}
	reconnectMax := cfg.ReconnectMax
	if reconnectMax <= 0 {
		reconnectMax = 30 * time.Second
	}
	if reconnectMax < reconnectBase {
		return nil, errors.New("reconnect max must be >= reconnect base")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sleep := cfg.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	random := cfg.Rand
	if random == nil {
		random = rand.New(rand.NewSource(now().UnixNano()))
	}

	return &AgentService{
		client:             client,
		sessionID:          sessionID,
		hostname:           hostname,
		agentVersion:       cfg.AgentVersion,
		startedAt:          startedAt.UTC(),
		heartbeatInterval:  heartbeatInterval,
		reconnectBase:      reconnectBase,
		reconnectMax:       reconnectMax,
		logger:             logger,
		now:                now,
		sleep:              sleep,
		rand:               random,
		queryExecutor:      cfg.QueryExecutor,
		schemaIntrospector: cfg.SchemaIntrospector,
	}, nil
}

func (s *AgentService) Run(ctx context.Context) error {
	backoff := s.reconnectBase

	for {
		connected, err := s.runSession(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if connected {
			backoff = s.reconnectBase
		}

		delay := s.jitter(backoff)
		s.logger.Warn(
			"agent session ended; reconnecting",
			"err",
			err,
			"delay",
			delay.String(),
			"session_id",
			s.sessionID,
		)

		if err := s.sleep(ctx, delay); err != nil {
			return nil
		}
		if !connected && backoff < s.reconnectMax {
			backoff *= 2
			if backoff > s.reconnectMax {
				backoff = s.reconnectMax
			}
		}
	}
}

func (s *AgentService) runSession(ctx context.Context) (bool, error) {
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := s.client.OpenCommandStream(
		sessionCtx,
		OpenCommandStreamRequest{
			SessionID:    s.sessionID,
			Hostname:     s.hostname,
			AgentVersion: s.agentVersion,
			StartedAt:    s.startedAt,
		},
	)
	if err != nil {
		return false, err
	}

	s.logger.Info(
		"command stream opened",
		"session_id",
		s.sessionID,
		"hostname",
		s.hostname,
	)

	heartbeatErrCh := make(chan error, 1)
	go func() {
		heartbeatErrCh <- s.runHeartbeatLoop(sessionCtx)
	}()

	for stream.Receive() {
		if err := s.handleCommand(sessionCtx, stream.Message()); err != nil {
			cancel()
			heartbeatErr := <-heartbeatErrCh
			if heartbeatErr != nil {
				return true, heartbeatErr
			}
			return true, err
		}
	}

	cancel()
	heartbeatErr := <-heartbeatErrCh
	if heartbeatErr != nil {
		return true, heartbeatErr
	}
	if err := stream.Err(); err != nil {
		return true, err
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	return true, errors.New("command stream closed")
}

func (s *AgentService) runHeartbeatLoop(ctx context.Context) error {
	if err := s.client.Heartbeat(ctx, HeartbeatRequest{
		SessionID: s.sessionID,
		SentAt:    s.now().UTC(),
	}); err != nil {
		return err
	}

	ticker := time.NewTicker(s.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.client.Heartbeat(ctx, HeartbeatRequest{
				SessionID: s.sessionID,
				SentAt:    s.now().UTC(),
			}); err != nil {
				return err
			}
		}
	}
}

func (s *AgentService) handleCommand(
	ctx context.Context,
	command ControlMessage,
) error {
	switch command.Kind {
	case "":
		return nil
	case CommandKindPing:
		issuedAt := command.IssuedAt
		if issuedAt.IsZero() {
			issuedAt = s.now().UTC()
		}
		roundTrip := s.now().UTC().Sub(issuedAt).Milliseconds()
		if roundTrip < 0 {
			roundTrip = 0
		}
		return s.client.SubmitPingResult(
			ctx,
			SubmitPingResultRequest{
				SessionID:   command.SessionID,
				CommandID:   command.CommandID,
				CompletedAt: s.now().UTC(),
				RoundTripMS: roundTrip,
			},
		)
	case CommandKindExecuteQuery:
		return s.handleExecuteQuery(ctx, command)
	case CommandKindIntrospectSchema:
		return s.handleIntrospectSchema(ctx, command)
	default:
		return fmt.Errorf("unsupported command kind %q", command.Kind)
	}
}

func (s *AgentService) handleExecuteQuery(
	ctx context.Context,
	command ControlMessage,
) error {
	if s.queryExecutor == nil {
		return s.client.SubmitExecuteQueryResult(
			ctx,
			SubmitExecuteQueryResultRequest{
				SessionID:   command.SessionID,
				CommandID:   command.CommandID,
				CompletedAt: s.now().UTC(),
				Error:       "query executor is not configured",
			},
		)
	}

	startedAt := s.now().UTC()
	result, err := s.queryExecutor.ExecuteQuery(ctx, command.SQL)
	if err != nil {
		return s.client.SubmitExecuteQueryResult(
			ctx,
			SubmitExecuteQueryResultRequest{
				SessionID:   command.SessionID,
				CommandID:   command.CommandID,
				CompletedAt: s.now().UTC(),
				ElapsedMS:   s.now().UTC().Sub(startedAt).Milliseconds(),
				Error:       err.Error(),
			},
		)
	}

	return s.client.SubmitExecuteQueryResult(
		ctx,
		SubmitExecuteQueryResultRequest{
			SessionID:    command.SessionID,
			CommandID:    command.CommandID,
			CompletedAt:  s.now().UTC(),
			Columns:      result.Columns,
			Rows:         result.Rows,
			ElapsedMS:    result.ElapsedMS,
			DatabaseUser: result.DatabaseUser,
			DatabaseName: result.DatabaseName,
		},
	)
}

func (s *AgentService) handleIntrospectSchema(
	ctx context.Context,
	command ControlMessage,
) error {
	if s.schemaIntrospector == nil {
		return s.client.SubmitIntrospectSchemaResult(
			ctx,
			SubmitIntrospectSchemaResultRequest{
				SessionID:   command.SessionID,
				CommandID:   command.CommandID,
				CompletedAt: s.now().UTC(),
				Error:       "schema introspector is not configured",
			},
		)
	}

	startedAt := s.now().UTC()
	schema, elapsedMS, databaseUser, databaseName, err := s.schemaIntrospector.IntrospectSchema(ctx)
	if err != nil {
		return s.client.SubmitIntrospectSchemaResult(
			ctx,
			SubmitIntrospectSchemaResultRequest{
				SessionID:   command.SessionID,
				CommandID:   command.CommandID,
				CompletedAt: s.now().UTC(),
				ElapsedMS:   s.now().UTC().Sub(startedAt).Milliseconds(),
				Error:       err.Error(),
			},
		)
	}

	return s.client.SubmitIntrospectSchemaResult(
		ctx,
		SubmitIntrospectSchemaResultRequest{
			SessionID:    command.SessionID,
			CommandID:    command.CommandID,
			CompletedAt:  s.now().UTC(),
			Schema:       schema,
			ElapsedMS:    elapsedMS,
			DatabaseUser: databaseUser,
			DatabaseName: databaseName,
		},
	)
}

func (s *AgentService) jitter(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	jitterWindow := base / 5
	if jitterWindow <= 0 {
		return base
	}
	return base + time.Duration(s.rand.Int63n(int64(jitterWindow)))
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
