package auditlog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
)

func TestFileLoggerWritesRedactedQueryEvent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "audit", "query-events.jsonl")
	logger := NewFileLogger(path)
	err := logger.LogQueryEvent(context.Background(), edgecontroller.QueryAuditEvent{
		CompletedAt:       time.Unix(1_700_000_000, 0).UTC(),
		SessionID:         "session-1",
		CommandID:         "command-1",
		SQL:               "SELECT station_id FROM readings",
		DatabaseUser:      "mission_ro@%",
		DatabaseName:      "mission_app",
		ElapsedMS:         12,
		RowCount:          3,
		ErrorCode:         "PERMISSION_DENIED",
		ErrorReason:       "SQL comments are not allowed",
		BlockedConstructs: []string{"comment"},
	})
	if err != nil {
		t.Fatalf("LogQueryEvent returned error: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	if strings.Contains(string(body), "\"rows\"") {
		t.Fatalf("audit log should not contain raw rows: %s", body)
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if event["row_count"] != float64(3) {
		t.Fatalf("row_count = %#v, want 3", event["row_count"])
	}
	if event["sql"] != "SELECT station_id FROM readings" {
		t.Fatalf("sql = %#v", event["sql"])
	}
	if _, ok := event["rows"]; ok {
		t.Fatalf("unexpected rows key in audit event: %#v", event)
	}
}

func TestFileLoggerRequiresPath(t *testing.T) {
	t.Parallel()

	logger := NewFileLogger("")
	if err := logger.LogQueryEvent(context.Background(), edgecontroller.QueryAuditEvent{}); err == nil {
		t.Fatal("LogQueryEvent returned nil error without a configured path")
	}
}
