package introspect

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestLoadIntegration(t *testing.T) {
	admin := openMySQLOrSkip(t, mysqlAdminDSN())
	defer admin.Close()

	loadFixture(t, admin)

	readonly := openMySQLOrSkip(t, mysqlReadOnlyDSN())
	defer readonly.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startedAt := time.Now()
	blob, err := Load(ctx, readonly, "mission_app")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	elapsed := time.Since(startedAt)
	if elapsed >= 60*time.Second {
		t.Fatalf("elapsed = %s, want < 60s", elapsed)
	}

	if len(blob.Tables) != 6 {
		t.Fatalf("table count = %d, want 6", len(blob.Tables))
	}
	if len(blob.Columns) < 20 {
		t.Fatalf("column count = %d, want at least 20", len(blob.Columns))
	}
	if len(blob.ForeignKeys) < 6 {
		t.Fatalf("foreign key count = %d, want at least 6", len(blob.ForeignKeys))
	}

	if blob.Tables[0].TableName != "addresses" {
		t.Fatalf("first table = %q, want addresses", blob.Tables[0].TableName)
	}

	var (
		foundTableComment  bool
		foundColumnComment bool
		foundJSONColumn    bool
	)
	for _, table := range blob.Tables {
		if table.TableName == "customers" && table.TableComment == "Customer master data" {
			foundTableComment = true
		}
	}
	for _, column := range blob.Columns {
		if column.TableName == "customers" && column.ColumnName == "name" && column.ColumnComment == "Customer display name" {
			foundColumnComment = true
		}
		if column.TableName == "products" && column.ColumnName == "metadata" && column.DataType == "json" {
			foundJSONColumn = true
		}
	}
	if !foundTableComment {
		t.Fatal("expected customer table comment")
	}
	if !foundColumnComment {
		t.Fatal("expected customer.name column comment")
	}
	if !foundJSONColumn {
		t.Fatal("expected products.metadata json column")
	}
}

func mysqlAdminDSN() string {
	if dsn := os.Getenv("MISSION_TEST_MYSQL_ADMIN_DSN"); dsn != "" {
		return dsn
	}
	return "root:mission@tcp(127.0.0.1:3306)/mission_app?multiStatements=true"
}

func mysqlReadOnlyDSN() string {
	if dsn := os.Getenv("MISSION_TEST_MYSQL_READONLY_DSN"); dsn != "" {
		return dsn
	}
	return "mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app"
}

func openMySQLOrSkip(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("skipping MySQL integration test: %v", err)
	}
	return db
}

func loadFixture(t *testing.T, db *sql.DB) {
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
