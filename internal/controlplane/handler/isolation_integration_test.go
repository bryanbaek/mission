package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	cpcontroller "github.com/bryanbaek/mission/internal/controlplane/controller"
	cpdb "github.com/bryanbaek/mission/internal/controlplane/db"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	"github.com/bryanbaek/mission/internal/edgeagent/auditlog"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	controlplane "github.com/bryanbaek/mission/internal/edgeagent/gateway/controlplane"
	mysqlgateway "github.com/bryanbaek/mission/internal/edgeagent/gateway/mysql"
)

func TestTenantIsolationSmokeIntegration(t *testing.T) {
	ctx := context.Background()

	pool := openPostgresOrSkip(t)
	t.Cleanup(pool.Close)
	if err := cpdb.Migrate(postgresURL()); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	adminDB := openMySQLOrSkipIntegration(t, mysqlAdminDSNIntegration())
	t.Cleanup(func() {
		if err := adminDB.Close(); err != nil {
			t.Errorf("adminDB.Close returned error: %v", err)
		}
	})

	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	commerceDB := provisionIsolationDatabase(
		t,
		adminDB,
		fmt.Sprintf("iso_commerce_%s", suffix),
		fmt.Sprintf("isoc_%s", suffix[:6]),
		fmt.Sprintf("mission_c_%s", suffix[:6]),
		filepath.Join("..", "..", "..", "tests", "fixtures", "schema_introspection.sql"),
		`
INSERT INTO customers (name, customer_code, profile, notes, created_at) VALUES
  ('Apex Cooling', 'CUST-001', NULL, NULL, '2026-04-01 09:00:00'),
  ('Blue River Labs', 'CUST-002', NULL, NULL, '2026-04-02 10:00:00');
`,
	)
	hrDB := provisionIsolationDatabase(
		t,
		adminDB,
		fmt.Sprintf("iso_hr_%s", suffix),
		fmt.Sprintf("isoh_%s", suffix[:6]),
		fmt.Sprintf("mission_h_%s", suffix[:6]),
		filepath.Join("..", "..", "..", "tests", "fixtures", "hr_schema.sql"),
		`
INSERT INTO departments (name, cost_center, created_at) VALUES
  ('Operations', 'OPS-100', '2026-04-01 09:00:00'),
  ('Finance', 'FIN-200', '2026-04-01 09:05:00');
INSERT INTO positions (department_id, title, level_code, created_at) VALUES
  (1, 'Operations Lead', 'L3', '2026-04-01 09:10:00'),
  (2, 'Finance Analyst', 'L2', '2026-04-01 09:15:00');
INSERT INTO employees (department_id, position_id, manager_id, full_name, employment_status, hired_at) VALUES
  (1, 1, NULL, 'Kim Minseo', 'active', '2025-01-10 09:00:00'),
  (1, 1, 1, 'Park Jisoo', 'active', '2025-02-14 09:00:00'),
  (2, 2, 1, 'Lee Haneul', 'active', '2025-03-21 09:00:00');
`,
	)

	tenantRepo := repository.NewTenantRepository(pool)
	tokenRepo := repository.NewTenantTokenRepository(pool)
	schemaRepo := repository.NewTenantSchemaRepository(pool)
	semanticRepo := repository.NewTenantSemanticLayerRepository(pool)
	queryRunRepo := repository.NewTenantQueryRunRepository(pool)
	queryFeedbackRepo := repository.NewTenantQueryFeedbackRepository(pool)
	canonicalExampleRepo := repository.NewTenantCanonicalQueryExampleRepository(pool)
	tenantCtrl := cpcontroller.NewTenantController(tenantRepo, tokenRepo)
	agentSessions := cpcontroller.NewAgentSessionManager(cpcontroller.AgentSessionManagerConfig{})
	schemaCtrl := cpcontroller.NewSchemaController(
		schemaRepo,
		agentSessions,
		cpcontroller.SchemaControllerConfig{},
	)
	queryCtrl := cpcontroller.NewQueryController(
		tenantCtrl,
		schemaRepo,
		semanticRepo,
		queryRunRepo,
		queryFeedbackRepo,
		canonicalExampleRepo,
		agentSessions,
		isolationFakeCompleter{},
		cpcontroller.QueryControllerConfig{
			Model:            "fake-isolation-model",
			MaxTokens:        512,
			SummaryModel:     "fake-isolation-model",
			SummaryMaxTokens: 256,
			MaxSummaryRows:   10,
		},
	)

	tenantA, err := tenantCtrl.Create(
		ctx,
		"user_123",
		"iso-commerce-"+suffix[:6],
		"Isolation Commerce",
	)
	if err != nil {
		t.Fatalf("Create tenantA returned error: %v", err)
	}
	tenantB, err := tenantCtrl.Create(
		ctx,
		"user_123",
		"iso-hr-"+suffix[:6],
		"Isolation HR",
	)
	if err != nil {
		t.Fatalf("Create tenantB returned error: %v", err)
	}
	t.Cleanup(func() {
		cleanupTenantRows(t, pool, tenantA.ID, tenantB.ID)
	})

	_, tokenAPlain, err := tenantCtrl.IssueAgentToken(ctx, tenantA.ID, "isolation-a")
	if err != nil {
		t.Fatalf("IssueAgentToken tenantA returned error: %v", err)
	}
	_, tokenBPlain, err := tenantCtrl.IssueAgentToken(ctx, tenantB.ID, "isolation-b")
	if err != nil {
		t.Fatalf("IssueAgentToken tenantB returned error: %v", err)
	}

	agentHandler := NewAgentHandler(agentSessions)
	agentPath, agentSvc := agentv1connect.NewAgentServiceHandler(agentHandler)
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(auth.RequireAgentToken(tokenRepo))
		r.Mount(agentPath, agentSvc)
	})
	server := httptest.NewServer(router)
	defer server.Close()

	commerceGateway, err := mysqlgateway.Open(ctx, commerceDB.DSN)
	if err != nil {
		t.Fatalf("mysqlgateway.Open commerce returned error: %v", err)
	}
	defer func() {
		if err := commerceGateway.Close(); err != nil {
			t.Errorf("commerceGateway.Close returned error: %v", err)
		}
	}()

	hrGateway, err := mysqlgateway.Open(ctx, hrDB.DSN)
	if err != nil {
		t.Fatalf("mysqlgateway.Open hr returned error: %v", err)
	}
	defer func() {
		if err := hrGateway.Close(); err != nil {
			t.Errorf("hrGateway.Close returned error: %v", err)
		}
	}()

	tempDir := t.TempDir()
	commerceAuditPath := filepath.Join(tempDir, "commerce-audit.jsonl")
	hrAuditPath := filepath.Join(tempDir, "hr-audit.jsonl")

	commerceService := mustIsolationAgentService(
		t,
		server,
		tokenAPlain,
		"iso-commerce-host",
		integrationRuntime{gateway: commerceGateway},
		commerceAuditPath,
	)
	hrService := mustIsolationAgentService(
		t,
		server,
		tokenBPlain,
		"iso-hr-host",
		integrationRuntime{gateway: hrGateway},
		hrAuditPath,
	)

	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	serviceErrCh := make(chan error, 2)
	go func() { serviceErrCh <- commerceService.Run(runCtx) }()
	go func() { serviceErrCh <- hrService.Run(runCtx) }()

	waitForAgentConnection(t, agentSessions, tenantA.ID)
	waitForAgentConnection(t, agentSessions, tenantB.ID)

	captureA, err := schemaCtrl.Capture(ctx, tenantA.ID)
	if err != nil {
		t.Fatalf("schemaCtrl.Capture tenantA returned error: %v", err)
	}
	captureB, err := schemaCtrl.Capture(ctx, tenantB.ID)
	if err != nil {
		t.Fatalf("schemaCtrl.Capture tenantB returned error: %v", err)
	}
	if captureA.SchemaHash == captureB.SchemaHash {
		t.Fatalf("schema hashes must differ, both were %q", captureA.SchemaHash)
	}

	question := "이번 달 핵심 수치를 한 줄로 알려줘"
	resultA, err := queryCtrl.AskQuestion(ctx, tenantA.ID, "user_123", question)
	if err != nil {
		t.Fatalf("queryCtrl.AskQuestion tenantA returned error: %v", err)
	}
	resultB, err := queryCtrl.AskQuestion(ctx, tenantB.ID, "user_123", question)
	if err != nil {
		t.Fatalf("queryCtrl.AskQuestion tenantB returned error: %v", err)
	}

	if resultA.SQLExecuted == resultB.SQLExecuted {
		t.Fatalf("executed SQL must differ, both were %q", resultA.SQLExecuted)
	}
	if resultA.SummaryKo == resultB.SummaryKo {
		t.Fatalf("summaries must differ, both were %q", resultA.SummaryKo)
	}
	if got := fmt.Sprint(resultA.Rows[0]["customer_count"]); got != "2" {
		t.Fatalf("tenantA customer_count = %q, want 2", got)
	}
	if got := fmt.Sprint(resultB.Rows[0]["employee_count"]); got != "3" {
		t.Fatalf("tenantB employee_count = %q, want 3", got)
	}
	if !strings.Contains(resultA.SQLExecuted, "customers") {
		t.Fatalf("tenantA executed SQL = %q, want customers table", resultA.SQLExecuted)
	}
	if !strings.Contains(resultB.SQLExecuted, "employees") {
		t.Fatalf("tenantB executed SQL = %q, want employees table", resultB.SQLExecuted)
	}

	commerceAudit := readIsolationAuditEvent(t, commerceAuditPath)
	hrAudit := readIsolationAuditEvent(t, hrAuditPath)
	if commerceAudit.DatabaseName != commerceDB.Name {
		t.Fatalf("commerce audit database_name = %q, want %q", commerceAudit.DatabaseName, commerceDB.Name)
	}
	if hrAudit.DatabaseName != hrDB.Name {
		t.Fatalf("hr audit database_name = %q, want %q", hrAudit.DatabaseName, hrDB.Name)
	}
	if !strings.Contains(commerceAudit.SQL, "customers") {
		t.Fatalf("commerce audit SQL = %q, want customers query", commerceAudit.SQL)
	}
	if !strings.Contains(hrAudit.SQL, "employees") {
		t.Fatalf("hr audit SQL = %q, want employees query", hrAudit.SQL)
	}
	if strings.Contains(commerceAudit.SQL, "employees") || commerceAudit.DatabaseName == hrDB.Name {
		t.Fatalf("commerce audit leaked hr data: %+v", commerceAudit)
	}
	if strings.Contains(hrAudit.SQL, "customers") || hrAudit.DatabaseName == commerceDB.Name {
		t.Fatalf("hr audit leaked commerce data: %+v", hrAudit)
	}

	cancelRun()
	for range 2 {
		select {
		case err := <-serviceErrCh:
			if err != nil {
				t.Fatalf("edge agent returned error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for edge agents to stop")
		}
	}
}

type isolationDatabase struct {
	Name string
	DSN  string
}

type isolationAuditEvent struct {
	SessionID    string `json:"session_id"`
	CommandID    string `json:"command_id"`
	SQL          string `json:"sql"`
	DatabaseName string `json:"database_name"`
}

type isolationFakeCompleter struct{}

func (isolationFakeCompleter) Complete(
	_ context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	prompt := req.Messages[0].Content
	if req.OutputFormat != nil {
		switch {
		case strings.Contains(prompt, "\"customers\""):
			return llm.CompletionResponse{
				Content:  `{"reasoning":"commerce tenant schema","sql":"SELECT COUNT(*) AS customer_count FROM customers","notes":""}`,
				Provider: "fake",
				Model:    "fake-isolation-model",
			}, nil
		case strings.Contains(prompt, "\"employees\""):
			return llm.CompletionResponse{
				Content:  `{"reasoning":"hr tenant schema","sql":"SELECT COUNT(*) AS employee_count FROM employees","notes":""}`,
				Provider: "fake",
				Model:    "fake-isolation-model",
			}, nil
		default:
			return llm.CompletionResponse{}, fmt.Errorf("unexpected sql-generation prompt: %s", prompt)
		}
	}

	switch {
	case strings.Contains(prompt, "customer_count"):
		return llm.CompletionResponse{
			Content:  "고객 데이터 기준 전체 고객 수는 2명입니다.",
			Provider: "fake",
			Model:    "fake-isolation-model",
		}, nil
	case strings.Contains(prompt, "employee_count"):
		return llm.CompletionResponse{
			Content:  "인사 데이터 기준 전체 직원 수는 3명입니다.",
			Provider: "fake",
			Model:    "fake-isolation-model",
		}, nil
	default:
		return llm.CompletionResponse{}, fmt.Errorf("unexpected summary prompt: %s", prompt)
	}
}

func provisionIsolationDatabase(
	t *testing.T,
	adminDB *sql.DB,
	databaseName, username, password, fixturePath, seedSQL string,
) isolationDatabase {
	t.Helper()

	for _, statement := range []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", databaseName),
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", username),
		fmt.Sprintf("CREATE DATABASE `%s`", databaseName),
		fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'", username, password),
		fmt.Sprintf("GRANT SELECT, SHOW VIEW ON `%s`.* TO '%s'@'%%'", databaseName, username),
		"FLUSH PRIVILEGES",
	} {
		if _, err := adminDB.Exec(statement); err != nil {
			t.Fatalf("Exec(%q) returned error: %v", statement, err)
		}
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", username),
			fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", databaseName),
		} {
			if _, err := adminDB.Exec(statement); err != nil {
				t.Errorf("cleanup Exec(%q) returned error: %v", statement, err)
			}
		}
	})

	dsn := fmt.Sprintf(
		"root:mission@tcp(127.0.0.1:3306)/%s?multiStatements=true",
		databaseName,
	)
	db := openMySQLOrSkipIntegration(t, dsn)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	}()

	loadSQLFixtureAtPath(t, db, fixturePath)
	if _, err := db.Exec(seedSQL); err != nil {
		t.Fatalf("seed data returned error: %v", err)
	}

	return isolationDatabase{
		Name: databaseName,
		DSN: fmt.Sprintf(
			"%s:%s@tcp(127.0.0.1:3306)/%s",
			username,
			password,
			databaseName,
		),
	}
}

func mustIsolationAgentService(
	t *testing.T,
	server *httptest.Server,
	tokenPlaintext, hostname string,
	runtime integrationRuntime,
	auditPath string,
) *edgecontroller.AgentService {
	t.Helper()

	client := controlplane.NewClient(server.URL, tokenPlaintext, server.Client())
	service, err := edgecontroller.NewAgentService(
		client,
		edgecontroller.AgentServiceConfig{
			Hostname:           hostname,
			HeartbeatInterval:  100 * time.Millisecond,
			QueryExecutor:      runtime,
			QueryAuditor:       auditlog.NewFileLogger(auditPath),
			SchemaIntrospector: runtime,
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}
	return service
}

func loadSQLFixtureAtPath(t *testing.T, db *sql.DB, path string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile returned error: %v", err)
	}
	if _, err := db.Exec(string(body)); err != nil {
		t.Fatalf("Exec fixture returned error: %v", err)
	}
}

func readIsolationAuditEvent(t *testing.T, path string) isolationAuditEvent {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile returned error: %v", err)
	}
	lines := strings.FieldsFunc(string(body), func(r rune) bool { return r == '\n' || r == '\r' })
	if len(lines) != 1 {
		t.Fatalf("expected exactly one audit log line in %s, got %d", path, len(lines))
	}

	var event isolationAuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	return event
}

func cleanupTenantRows(t *testing.T, pool *pgxpool.Pool, tenantIDs ...uuid.UUID) {
	t.Helper()

	for _, tenantID := range tenantIDs {
		if _, err := pool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID); err != nil {
			t.Errorf("DELETE tenant %s returned error: %v", tenantID, err)
		}
	}
}
