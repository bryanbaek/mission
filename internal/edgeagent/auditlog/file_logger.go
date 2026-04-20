package auditlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
)

type QueryEvent struct {
	CompletedAt       time.Time `json:"completed_at"`
	SessionID         string    `json:"session_id"`
	CommandID         string    `json:"command_id"`
	SQL               string    `json:"sql"`
	DatabaseUser      string    `json:"database_user,omitempty"`
	DatabaseName      string    `json:"database_name,omitempty"`
	ElapsedMS         int64     `json:"elapsed_ms"`
	RowCount          int       `json:"row_count"`
	ErrorCode         string    `json:"error_code,omitempty"`
	ErrorReason       string    `json:"error_reason,omitempty"`
	BlockedConstructs []string  `json:"blocked_constructs,omitempty"`
}

type FileLogger struct {
	path string
	mu   sync.Mutex
}

func NewFileLogger(path string) *FileLogger {
	return &FileLogger{path: path}
}

func (l *FileLogger) LogQueryEvent(
	_ context.Context,
	event edgecontroller.QueryAuditEvent,
) error {
	if l == nil || l.path == "" {
		return fmt.Errorf("audit log path is not configured")
	}

	body, err := json.Marshal(QueryEvent{
		CompletedAt:       event.CompletedAt,
		SessionID:         event.SessionID,
		CommandID:         event.CommandID,
		SQL:               event.SQL,
		DatabaseUser:      event.DatabaseUser,
		DatabaseName:      event.DatabaseName,
		ElapsedMS:         event.ElapsedMS,
		RowCount:          event.RowCount,
		ErrorCode:         event.ErrorCode,
		ErrorReason:       event.ErrorReason,
		BlockedConstructs: append([]string(nil), event.BlockedConstructs...),
	})
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create audit log directory: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}
