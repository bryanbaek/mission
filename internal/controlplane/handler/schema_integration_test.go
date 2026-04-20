package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	cpcontroller "github.com/bryanbaek/mission/internal/controlplane/controller"
	cpdb "github.com/bryanbaek/mission/internal/controlplane/db"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	controlplane "github.com/bryanbaek/mission/internal/edgeagent/gateway/controlplane"
	mysqlgateway "github.com/bryanbaek/mission/internal/edgeagent/gateway/mysql"
	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
)

func TestSchemaIntrospectionRoundTripIntegration(t *testing.T) {
	ctx := context.Background()

	pool := openPostgresOrSkip(t)
	defer pool.Close()
	if err := cpdb.Migrate(postgresURL()); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	adminDB := openMySQLOrSkipIntegration(t, mysqlAdminDSNIntegration())
	t.Cleanup(func() {
		if err := adminDB.Close(); err != nil {
			t.Errorf("adminDB.Close returned error: %v", err)
		}
	})
	loadSchemaFixture(t, adminDB)

	tenantRepo := repository.NewTenantRepository(pool)
	tokenRepo := repository.NewTenantTokenRepository(pool)
	schemaRepo := repository.NewTenantSchemaRepository(pool)
	tenantCtrl := cpcontroller.NewTenantController(tenantRepo, tokenRepo)
	agentSessions := cpcontroller.NewAgentSessionManager(cpcontroller.AgentSessionManagerConfig{})
	schemaCtrl := cpcontroller.NewSchemaController(
		schemaRepo,
		agentSessions,
		cpcontroller.SchemaControllerConfig{},
	)

	tenant, err := tenantCtrl.Create(
		ctx,
		"user_123",
		"schema-it-"+uuid.NewString()[:8],
		"Schema Integration Test",
	)
	if err != nil {
		t.Fatalf("Create tenant returned error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	_, plaintextToken, err := tenantCtrl.IssueAgentToken(ctx, tenant.ID, "edge-it")
	if err != nil {
		t.Fatalf("IssueAgentToken returned error: %v", err)
	}

	agentHandler := NewAgentHandler(agentSessions)
	schemaHandler := NewSchemaDebugHandler(tenantCtrl, schemaCtrl)
	agentPath, agentSvc := agentv1connect.NewAgentServiceHandler(agentHandler)

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(auth.RequireAgentToken(tokenRepo))
		r.Mount(agentPath, agentSvc)
	})
	router.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(&auth.FakeVerifier{
			Tokens: map[string]auth.User{"dev-token": {ID: "user_123"}},
		}))
		r.Post(
			"/api/debug/tenants/{tenantID}/schema/introspect",
			schemaHandler.Introspect,
		)
	})

	server := httptest.NewServer(router)
	defer server.Close()

	mysqlGateway, err := mysqlgateway.Open(ctx, mysqlReadOnlyDSNIntegration())
	if err != nil {
		t.Fatalf("mysqlgateway.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := mysqlGateway.Close(); err != nil {
			t.Errorf("mysqlGateway.Close returned error: %v", err)
		}
	})

	edgeClient := controlplane.NewClient(server.URL, plaintextToken, server.Client())
	service, err := edgecontroller.NewAgentService(
		edgeClient,
		edgecontroller.AgentServiceConfig{
			Hostname:           "schema-it-host",
			HeartbeatInterval:  100 * time.Millisecond,
			QueryExecutor:      integrationRuntime{gateway: mysqlGateway},
			SchemaIntrospector: integrationRuntime{gateway: mysqlGateway},
		},
	)
	if err != nil {
		t.Fatalf("NewAgentService returned error: %v", err)
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- service.Run(runCtx)
	}()

	waitForAgentConnection(t, agentSessions, tenant.ID)

	first := postSchemaIntrospection(t, server, tenant.ID)
	if !first.Changed {
		t.Fatal("first capture Changed = false, want true")
	}
	if first.TableCount != 6 {
		t.Fatalf("first TableCount = %d, want 6", first.TableCount)
	}

	second := postSchemaIntrospection(t, server, tenant.ID)
	if second.Changed {
		t.Fatal("second capture Changed = true, want false")
	}
	if second.VersionID != first.VersionID {
		t.Fatalf("second VersionID = %q, want %q", second.VersionID, first.VersionID)
	}

	if _, err := adminDB.Exec(`
		ALTER TABLE orders
		ADD COLUMN external_reference VARCHAR(64) NULL COMMENT 'Partner reference'
	`); err != nil {
		t.Fatalf("ALTER TABLE returned error: %v", err)
	}

	third := postSchemaIntrospection(t, server, tenant.ID)
	if !third.Changed {
		t.Fatal("third capture Changed = false, want true")
	}
	if third.VersionID == first.VersionID {
		t.Fatalf("third VersionID = %q, want new version", third.VersionID)
	}

	var rowCount int
	if err := pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM tenant_schemas WHERE tenant_id = $1",
		tenant.ID,
	).Scan(&rowCount); err != nil {
		t.Fatalf("QueryRow count returned error: %v", err)
	}
	if rowCount != 2 {
		t.Fatalf("rowCount = %d, want 2", rowCount)
	}

	cancelRun()
	select {
	case err := <-runErrCh:
		if err != nil {
			t.Fatalf("service.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for edge agent shutdown")
	}
}

type integrationRuntime struct {
	gateway *mysqlgateway.Gateway
}

func (r integrationRuntime) ExecuteQuery(
	ctx context.Context,
	sql string,
) (edgecontroller.QueryResult, error) {
	result, err := r.gateway.ExecuteQuery(ctx, sql)
	if err != nil {
		return edgecontroller.QueryResult{}, err
	}
	return edgecontroller.QueryResult{
		Columns:      result.Columns,
		Rows:         result.Rows,
		ElapsedMS:    result.ElapsedMS,
		DatabaseUser: result.DatabaseUser,
		DatabaseName: result.DatabaseName,
	}, nil
}

func (r integrationRuntime) IntrospectSchema(
	ctx context.Context,
) (introspect.SchemaBlob, int64, string, string, error) {
	return r.gateway.IntrospectSchema(ctx)
}

type schemaCaptureResponse struct {
	VersionID       string    `json:"version_id"`
	Changed         bool      `json:"changed"`
	CapturedAt      time.Time `json:"captured_at"`
	SchemaHash      string    `json:"schema_hash"`
	DatabaseName    string    `json:"database_name"`
	TableCount      int       `json:"table_count"`
	ColumnCount     int       `json:"column_count"`
	ForeignKeyCount int       `json:"foreign_key_count"`
}

func postSchemaIntrospection(
	t *testing.T,
	server *httptest.Server,
	tenantID uuid.UUID,
) schemaCaptureResponse {
	t.Helper()

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/api/debug/tenants/"+tenantID.String()+"/schema/introspect",
		nil,
	)
	if err != nil {
		t.Fatalf("http.NewRequest returned error: %v", err)
	}
	req.Header.Set("Authorization", "Bearer dev-token")

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("resp.Body.Close returned error: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("status = %d, want 200 (body=%v)", resp.StatusCode, body)
	}

	var body schemaCaptureResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("json.NewDecoder returned error: %v", err)
	}
	return body
}

func waitForAgentConnection(
	t *testing.T,
	sessions *cpcontroller.AgentSessionManager,
	tenantID uuid.UUID,
) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := sessions.LatestSessionForTenant(tenantID)
		if ok && snapshot.Status == "online" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for agent connection")
}

func openPostgresOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), postgresURL())
	if err != nil {
		t.Skipf("skipping Postgres integration test: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping Postgres integration test: %v", err)
	}
	return pool
}

func postgresURL() string {
	if url := os.Getenv("MISSION_TEST_POSTGRES_URL"); url != "" {
		return url
	}
	return "postgres://mission:mission@127.0.0.1:5432/mission?sslmode=disable"
}

func mysqlAdminDSNIntegration() string {
	if dsn := os.Getenv("MISSION_TEST_MYSQL_ADMIN_DSN"); dsn != "" {
		return dsn
	}
	return "root:mission@tcp(127.0.0.1:3306)/mission_app?multiStatements=true"
}

func mysqlReadOnlyDSNIntegration() string {
	if dsn := os.Getenv("MISSION_TEST_MYSQL_READONLY_DSN"); dsn != "" {
		return dsn
	}
	return "mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app"
}

func openMySQLOrSkipIntegration(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("db.Close returned error after PingContext failed: %v", closeErr)
		}
		t.Skipf("skipping MySQL integration test: %v", err)
	}
	return db
}

func loadSchemaFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "tests", "fixtures", "schema_introspection.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile returned error: %v", err)
	}
	if _, err := db.Exec(string(body)); err != nil {
		t.Fatalf("Exec fixture returned error: %v", err)
	}
}
