package mysqlgateway

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func integrationDSN(t *testing.T, readWrite bool) string {
	t.Helper()

	key := "MISSION_TEST_MYSQL_READONLY_DSN"
	fallback := "mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app"
	if readWrite {
		key = "MISSION_TEST_MYSQL_READWRITE_DSN"
		fallback = "mission_rw:mission_rw@tcp(127.0.0.1:3306)/mission_app"
	}

	dsn := os.Getenv(key)
	if dsn == "" {
		dsn = fallback
	}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	cfg.Timeout = 2 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping MySQL integration test: %v", err)
	}

	return dsn
}
