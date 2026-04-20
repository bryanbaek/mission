package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	cpdb "github.com/bryanbaek/mission/internal/controlplane/db"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	mysqlgateway "github.com/bryanbaek/mission/internal/edgeagent/gateway/mysql"
	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
)

func TestSemanticLayerDraftRoundTripIntegration(t *testing.T) {
	ctx := context.Background()

	pool := openSemanticPostgresOrSkip(t)
	defer pool.Close()
	if err := cpdb.Migrate(semanticPostgresURL()); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	adminDB := openSemanticMySQLOrSkip(t, semanticMySQLAdminDSN())
	t.Cleanup(func() {
		if err := adminDB.Close(); err != nil {
			t.Errorf("adminDB.Close returned error: %v", err)
		}
	})
	loadSemanticSchemaFixture(t, adminDB)

	mysqlGateway, err := mysqlgateway.Open(ctx, semanticMySQLReadOnlyDSN())
	if err != nil {
		t.Fatalf("mysqlgateway.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := mysqlGateway.Close(); err != nil {
			t.Errorf("mysqlGateway.Close returned error: %v", err)
		}
	})

	introspected, _, _, _, err := mysqlGateway.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("IntrospectSchema returned error: %v", err)
	}

	tenantID := uuid.New()
	if _, err := pool.Exec(
		ctx,
		`INSERT INTO tenants (id, slug, name) VALUES ($1, $2, $3)`,
		tenantID,
		"semantic-it-"+uuid.NewString()[:8],
		"Semantic Integration Test",
	); err != nil {
		t.Fatalf("insert tenant returned error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID)
	})

	schemaRepo := repository.NewTenantSchemaRepository(pool)
	layerRepo := repository.NewTenantSemanticLayerRepository(pool)

	modelBlob := normalizeSchemaBlob(introspectToModelSchema(introspected))
	schemaJSON, err := json.Marshal(modelBlob)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	schemaVersion, err := schemaRepo.Create(
		ctx,
		tenantID,
		time.Unix(1_700_000_900, 0).UTC(),
		hashSchemaBlob(schemaJSON),
		schemaJSON,
	)
	if err != nil {
		t.Fatalf("Create schema version returned error: %v", err)
	}

	generated := generatedSemanticContentForSchema(modelBlob)
	generatedJSON, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	completer := &fakeSemanticCompleter{
		response: llm.CompletionResponse{
			Content:  string(generatedJSON),
			Provider: "fake",
			Model:    "claude-sonnet-4-6",
			Usage: llm.Usage{
				Provider:             "fake",
				Model:                "claude-sonnet-4-6",
				CacheReadInputTokens: 17,
			},
		},
	}

	ctrl := NewSemanticLayerController(
		fakeSemanticMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID, ClerkUserID: "user_123"}, nil
			},
		},
		schemaRepo,
		layerRepo,
		completer,
		SemanticLayerControllerConfig{
			Model:     "claude-sonnet-4-6",
			MaxTokens: 4096,
		},
	)

	result, err := ctrl.Draft(ctx, tenantID, "user_123", schemaVersion.ID)
	if err != nil {
		t.Fatalf("Draft returned error: %v", err)
	}
	if result.Usage.CacheReadInputTokens != 17 {
		t.Fatalf("CacheReadInputTokens = %d, want 17", result.Usage.CacheReadInputTokens)
	}

	stored, err := layerRepo.LatestDraftBySchemaVersion(ctx, tenantID, schemaVersion.ID)
	if err != nil {
		t.Fatalf("LatestDraftBySchemaVersion returned error: %v", err)
	}

	var roundTripped model.SemanticLayerContent
	if err := json.Unmarshal(stored.Content, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal stored content returned error: %v", err)
	}
	if len(roundTripped.Tables) != len(modelBlob.Tables) {
		t.Fatalf("table count = %d, want %d", len(roundTripped.Tables), len(modelBlob.Tables))
	}

	nonEmptyDescriptions := 0
	totalColumns := 0
	for _, table := range roundTripped.Tables {
		if strings.TrimSpace(table.Description) != "" {
			nonEmptyDescriptions++
		}
		for _, column := range table.Columns {
			totalColumns++
			if strings.TrimSpace(column.Description) != "" {
				nonEmptyDescriptions++
			}
		}
	}
	if nonEmptyDescriptions == 0 || totalColumns == 0 {
		t.Fatalf("expected non-empty Korean descriptions, got %d descriptions across %d columns", nonEmptyDescriptions, totalColumns)
	}
	if roundTripped.Tables[0].Description == "" {
		t.Fatal("expected first table description to be populated")
	}
	if got := result.Layer.Content.Tables[0].Columns[0].Description; got == "" {
		t.Fatal("expected first column description to be populated")
	}
}

func generatedSemanticContentForSchema(schema model.SchemaBlob) model.SemanticLayerContent {
	columnsByTable := make(map[string][]model.SemanticColumn)
	for _, column := range schema.Columns {
		key := column.TableSchema + "." + column.TableName
		columnsByTable[key] = append(columnsByTable[key], model.SemanticColumn{
			TableSchema:     column.TableSchema,
			TableName:       column.TableName,
			ColumnName:      column.ColumnName,
			OrdinalPosition: column.OrdinalPosition,
			DataType:        column.DataType,
			ColumnType:      column.ColumnType,
			IsNullable:      column.IsNullable,
			ColumnComment:   column.ColumnComment,
			Description:     koreanizeIdentifier(column.ColumnName) + " 설명",
		})
	}

	out := model.SemanticLayerContent{
		Tables:           make([]model.SemanticTable, 0, len(schema.Tables)),
		Entities:         []model.SemanticEntity{{Name: "핵심 엔터티", Description: "대표 엔터티 묶음", SourceTables: []string{schema.Tables[0].TableSchema + "." + schema.Tables[0].TableName}}},
		CandidateMetrics: []model.CandidateMetric{{Name: "레코드 수", Description: "행 수 기반 기본 지표", SourceTables: []string{schema.Tables[0].TableSchema + "." + schema.Tables[0].TableName}}},
	}
	for _, table := range schema.Tables {
		key := table.TableSchema + "." + table.TableName
		out.Tables = append(out.Tables, model.SemanticTable{
			TableSchema:  table.TableSchema,
			TableName:    table.TableName,
			TableType:    table.TableType,
			TableComment: table.TableComment,
			Description:  koreanizeIdentifier(table.TableName) + " 테이블",
			Columns:      columnsByTable[key],
		})
	}
	return out
}

func koreanizeIdentifier(identifier string) string {
	switch identifier {
	case "customers":
		return "고객"
	case "orders":
		return "주문"
	case "line_items":
		return "주문 항목"
	case "products":
		return "제품"
	case "suppliers":
		return "공급사"
	case "shipments":
		return "출하"
	default:
		return identifier
	}
}

func introspectToModelSchema(schema introspect.SchemaBlob) model.SchemaBlob {
	out := model.SchemaBlob{
		DatabaseName: schema.DatabaseName,
		Tables:       make([]model.SchemaTable, 0, len(schema.Tables)),
		Columns:      make([]model.SchemaColumn, 0, len(schema.Columns)),
		PrimaryKeys:  make([]model.SchemaPrimaryKey, 0, len(schema.PrimaryKeys)),
		ForeignKeys:  make([]model.SchemaForeignKey, 0, len(schema.ForeignKeys)),
	}
	for _, table := range schema.Tables {
		out.Tables = append(out.Tables, model.SchemaTable{
			TableSchema:  table.TableSchema,
			TableName:    table.TableName,
			TableType:    table.TableType,
			TableComment: table.TableComment,
		})
	}
	for _, column := range schema.Columns {
		out.Columns = append(out.Columns, model.SchemaColumn{
			TableSchema:     column.TableSchema,
			TableName:       column.TableName,
			ColumnName:      column.ColumnName,
			OrdinalPosition: column.OrdinalPosition,
			DataType:        column.DataType,
			ColumnType:      column.ColumnType,
			IsNullable:      column.IsNullable,
			HasDefault:      column.HasDefault,
			DefaultValue:    column.DefaultValue,
			ColumnComment:   column.ColumnComment,
		})
	}
	for _, primaryKey := range schema.PrimaryKeys {
		out.PrimaryKeys = append(out.PrimaryKeys, model.SchemaPrimaryKey{
			TableSchema:     primaryKey.TableSchema,
			TableName:       primaryKey.TableName,
			ConstraintName:  primaryKey.ConstraintName,
			ColumnName:      primaryKey.ColumnName,
			OrdinalPosition: primaryKey.OrdinalPosition,
		})
	}
	for _, foreignKey := range schema.ForeignKeys {
		out.ForeignKeys = append(out.ForeignKeys, model.SchemaForeignKey{
			TableSchema:           foreignKey.TableSchema,
			TableName:             foreignKey.TableName,
			ConstraintName:        foreignKey.ConstraintName,
			ColumnName:            foreignKey.ColumnName,
			OrdinalPosition:       foreignKey.OrdinalPosition,
			ReferencedTableSchema: foreignKey.ReferencedTableSchema,
			ReferencedTableName:   foreignKey.ReferencedTableName,
			ReferencedColumnName:  foreignKey.ReferencedColumnName,
		})
	}
	return out
}

func openSemanticPostgresOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), semanticPostgresURL())
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

func semanticPostgresURL() string {
	if url := os.Getenv("MISSION_TEST_POSTGRES_URL"); url != "" {
		return url
	}
	return "postgres://mission:mission@127.0.0.1:5432/mission?sslmode=disable"
}

func semanticMySQLAdminDSN() string {
	if dsn := os.Getenv("MISSION_TEST_MYSQL_ADMIN_DSN"); dsn != "" {
		return dsn
	}
	return "root:mission@tcp(127.0.0.1:3306)/mission_app?multiStatements=true"
}

func semanticMySQLReadOnlyDSN() string {
	if dsn := os.Getenv("MISSION_TEST_MYSQL_READONLY_DSN"); dsn != "" {
		return dsn
	}
	return "mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app"
}

func openSemanticMySQLOrSkip(t *testing.T, dsn string) *sql.DB {
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

func loadSemanticSchemaFixture(t *testing.T, db *sql.DB) {
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
