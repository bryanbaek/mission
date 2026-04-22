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

func TestReviewQueueMigrationUpContainsRequiredStructures(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("migrations", "0002_review_queue_fields.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	sql := string(body)
	required := []string{
		"ADD COLUMN reviewed_at TIMESTAMPTZ NULL",
		"ADD COLUMN reviewed_by_user_id TEXT NULL",
		"CREATE INDEX tenant_query_runs_review_queue_idx",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

func TestReviewQueueMigrationDownDropsRequiredStructures(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("migrations", "0002_review_queue_fields.down.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	sql := string(body)
	required := []string{
		"DROP INDEX IF EXISTS tenant_query_runs_review_queue_idx",
		"DROP COLUMN IF EXISTS reviewed_by_user_id",
		"DROP COLUMN IF EXISTS reviewed_at",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
