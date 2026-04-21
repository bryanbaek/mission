package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryMemoryMigrationUpContainsRequiredStructures(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("migrations", "0001_schema.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	sql := string(body)
	required := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE TABLE tenant_query_runs",
		"CREATE TABLE tenant_query_feedback",
		"CREATE TABLE tenant_canonical_query_examples",
		"CREATE INDEX tenant_canonical_query_examples_question_trgm_idx",
		"CREATE INDEX tenant_canonical_query_examples_notes_trgm_idx",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

func TestQueryMemoryMigrationDownDropsRequiredStructures(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("migrations", "0001_schema.down.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	sql := string(body)
	required := []string{
		"DROP TABLE IF EXISTS tenant_canonical_query_examples",
		"DROP TABLE IF EXISTS tenant_query_feedback",
		"DROP TABLE IF EXISTS tenant_query_runs",
		"DROP EXTENSION IF EXISTS pg_trgm",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
