package mysqlgateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysql "github.com/go-sql-driver/mysql"

	"github.com/bryanbaek/mission/internal/queryerror"
)

func TestNormalizeDSN(t *testing.T) {
	t.Parallel()

	got, err := NormalizeDSN(
		"mission_ro:mission_ro@tcp(localhost:3306)/mission_app?parseTime=false&multiStatements=true&timeout=1s&readTimeout=1s&writeTimeout=1s",
	)
	if err != nil {
		t.Fatalf("NormalizeDSN returned error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if !cfg.ParseTime {
		t.Fatal("ParseTime = false, want true")
	}
	if cfg.MultiStatements {
		t.Fatal("MultiStatements = true, want false")
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %s, want 5s", cfg.Timeout)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Fatalf("ReadTimeout = %s, want 30s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 5*time.Second {
		t.Fatalf("WriteTimeout = %s, want 5s", cfg.WriteTimeout)
	}
	if cfg.Loc != time.UTC {
		t.Fatalf("Loc = %v, want UTC", cfg.Loc)
	}
}

func TestValidateGrants(t *testing.T) {
	t.Parallel()

	validGrants := []string{
		"GRANT USAGE ON *.* TO `mission_ro`@`%`",
		"GRANT SELECT, SHOW VIEW ON `mission_app`.* TO `mission_ro`@`%`",
	}
	if err := ValidateGrants(validGrants); err != nil {
		t.Fatalf("ValidateGrants returned error for valid grants: %v", err)
	}

	invalidCases := map[string][]string{
		"all privileges": {
			"GRANT ALL PRIVILEGES ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"insert": {
			"GRANT USAGE ON *.* TO `mission_rw`@`%`",
			"GRANT SELECT, INSERT ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"update": {
			"GRANT SELECT, UPDATE ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"delete": {
			"GRANT SELECT, DELETE ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"create": {
			"GRANT SELECT, CREATE ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"drop": {
			"GRANT SELECT, DROP ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"alter": {
			"GRANT SELECT, ALTER ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"execute": {
			"GRANT SELECT, EXECUTE ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"file": {
			"GRANT SELECT, FILE ON *.* TO `mission_rw`@`%`",
		},
		"trigger": {
			"GRANT SELECT, TRIGGER ON `mission_app`.* TO `mission_rw`@`%`",
		},
		"missing select": {
			"GRANT USAGE ON *.* TO `mission_ro`@`%`",
		},
	}

	for name, grants := range invalidCases {
		name := name
		grants := grants
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateGrants(grants); err == nil {
				t.Fatalf("ValidateGrants returned nil error for %s", name)
			}
		})
	}
}

func TestNormalizeValue(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 19, 12, 34, 56, 0, time.UTC)

	if got := normalizeValue(int64(1), nil); got != int64(1) {
		t.Fatalf("normalizeValue(int64(1), nil) = %#v, want 1", got)
	}
	if got := normalizeValue(nil, nil); got != nil {
		t.Fatalf("normalizeValue(nil, nil) = %#v, want nil", got)
	}
	if got := normalizeBytesValue([]byte("hello"), "VARCHAR", 0, false); got != "hello" {
		t.Fatalf("normalizeBytesValue(VARCHAR) = %#v, want hello", got)
	}
	if got := normalizeBytesValue([]byte("1"), "INT", 0, false); got != int64(1) {
		t.Fatalf("normalizeBytesValue(INT) = %#v, want 1", got)
	}
	if got := normalizeBytesValue([]byte("12.34"), "DECIMAL", 2, true); got != "12.34" {
		t.Fatalf("normalizeBytesValue(DECIMAL) = %#v, want 12.34", got)
	}
	if got := normalizeBytesValue([]byte("blob"), "BLOB", 0, false); got != "blob" {
		t.Fatalf("normalizeBytesValue(BLOB) = %#v, want blob", got)
	}
	if got := normalizeTimeValue(now, "TIMESTAMP"); got != "2026-04-19T12:34:56Z" {
		t.Fatalf("normalizeTimeValue(TIMESTAMP) = %#v, want RFC3339 timestamp", got)
	}
	if got := normalizeTimeValue(now, "DATE"); got != "2026-04-19" {
		t.Fatalf("normalizeTimeValue(DATE) = %#v, want 2026-04-19", got)
	}
}

func TestWithTimeoutCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	capped, cancel := withTimeoutCap(ctx, 30*time.Second)
	defer cancel()
	if _, ok := capped.Deadline(); !ok {
		t.Fatal("withTimeoutCap should add a deadline when one is missing")
	}

	existing, existingCancel := context.WithTimeout(ctx, time.Second)
	defer existingCancel()
	same, cancelSame := withTimeoutCap(existing, 30*time.Second)
	defer cancelSame()
	if _, ok := same.Deadline(); !ok {
		t.Fatal("withTimeoutCap should preserve an existing earlier deadline")
	}
}

func TestExecuteQueryRejectsBlockedSQLBeforeDB(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	gateway := &Gateway{db: db}
	_, err = gateway.ExecuteQuery(context.Background(), "SELECT 1 # injected")
	if err == nil {
		t.Fatal("ExecuteQuery returned nil error for blocked SQL")
	}

	var queryErr *queryerror.Error
	if !errors.As(err, &queryErr) {
		t.Fatalf("err = %v, want queryerror.Error", err)
	}
	if queryErr.Code != queryerror.CodePermissionDenied {
		t.Fatalf("Code = %q, want %q", queryErr.Code, queryerror.CodePermissionDenied)
	}
	if queryErr.Reason != "SQL comments are not allowed" {
		t.Fatalf("Reason = %q, want SQL comments are not allowed", queryErr.Reason)
	}
	if len(queryErr.BlockedConstructs) != 1 || queryErr.BlockedConstructs[0] != "comment" {
		t.Fatalf("BlockedConstructs = %#v, want [comment]", queryErr.BlockedConstructs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected database interaction: %v", err)
	}
}

func TestOpenIntegration(t *testing.T) {
	dsn := integrationDSN(t, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gateway, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = gateway.Close()
	})

	result, err := gateway.ExecuteQuery(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("ExecuteQuery returned error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(result.Rows) = %d, want 1", len(result.Rows))
	}
	if got := result.Rows[0]["1"]; got != int64(1) {
		t.Fatalf("result.Rows[0][\"1\"] = %#v, want 1", got)
	}
	if result.DatabaseUser == "" {
		t.Fatal("DatabaseUser should not be empty")
	}
	if result.DatabaseName != "mission_app" {
		t.Fatalf("DatabaseName = %q, want mission_app", result.DatabaseName)
	}
}

func TestOpenRejectsReadWriteIntegration(t *testing.T) {
	dsn := integrationDSN(t, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Open(ctx, dsn)
	if err == nil {
		t.Fatal("Open returned nil error for read-write user")
	}
}

func TestExecuteQueryTimeoutIntegration(t *testing.T) {
	dsn := integrationDSN(t, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gateway, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = gateway.Close()
	})

	_, err = gateway.ExecuteQuery(context.Background(), "SELECT SLEEP(31)")
	if err == nil {
		t.Fatal("ExecuteQuery returned nil error for long-running query")
	}
	if !errors.Is(err, context.DeadlineExceeded) &&
		!strings.Contains(strings.ToLower(err.Error()), "interrupted") &&
		!strings.Contains(strings.ToLower(err.Error()), "maximum statement execution time exceeded") {
		t.Fatalf("ExecuteQuery error = %v, want timeout-related failure", err)
	}
}
