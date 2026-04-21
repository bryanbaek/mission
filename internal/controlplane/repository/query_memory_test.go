package repository

import (
	"context"
	dbsql "database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

func TestTenantQueryRunRepositoryCreate(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	semanticLayerID := uuid.New()
	createdAt := time.Unix(1_700_000_500, 0).UTC()
	repo := &TenantQueryRunRepository{
		db: &fakeTenantDB{
			queryRowFn: func(_ context.Context, query string, args ...any) pgx.Row {
				if !strings.Contains(query, "INSERT INTO tenant_query_runs") {
					t.Fatalf("unexpected SQL: %q", query)
				}
				if len(args) != 6 {
					t.Fatalf("args len = %d, want 6", len(args))
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = uuid.New()
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*uuid.UUID)) = schemaVersionID
					*(dest[3].(**uuid.UUID)) = &semanticLayerID
					*(dest[4].(*model.QueryPromptContextSource)) = model.QueryPromptContextSourceApproved
					*(dest[5].(*string)) = "user_123"
					*(dest[6].(*string)) = "평균 pH는?"
					*(dest[7].(*model.QueryRunStatus)) = model.QueryRunStatusRunning
					*(dest[8].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[9].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[10].(*int64)) = 0
					*(dest[11].(*int64)) = 0
					*(dest[12].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[13].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[14].(*time.Time)) = createdAt
					*(dest[15].(**time.Time)) = nil
					return nil
				}}
			},
		},
	}

	got, err := repo.Create(
		context.Background(),
		tenantID,
		schemaVersionID,
		&semanticLayerID,
		model.QueryPromptContextSourceApproved,
		"user_123",
		"평균 pH는?",
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.TenantID != tenantID || got.SchemaVersionID != schemaVersionID {
		t.Fatalf("Create returned %+v", got)
	}
	if got.SemanticLayerID == nil || *got.SemanticLayerID != semanticLayerID {
		t.Fatalf("SemanticLayerID = %v, want %s", got.SemanticLayerID, semanticLayerID)
	}
	if got.Status != model.QueryRunStatusRunning {
		t.Fatalf("Status = %q, want running", got.Status)
	}
}

func TestTenantQueryRunRepositoryCompleteSucceededMarshalsPayloads(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	exampleID := uuid.New()
	completedAt := time.Unix(1_700_000_600, 0).UTC()
	repo := &TenantQueryRunRepository{
		db: &fakeTenantDB{
			queryRowFn: func(_ context.Context, query string, args ...any) pgx.Row {
				if !strings.Contains(query, "UPDATE tenant_query_runs") {
					t.Fatalf("unexpected SQL: %q", query)
				}
				if args[0] != runID {
					t.Fatalf("run id arg = %v, want %s", args[0], runID)
				}
				attemptsJSON, ok := args[4].([]byte)
				if !ok {
					t.Fatalf("attempts arg type = %T, want []byte", args[4])
				}
				var attempts []model.QueryRunAttempt
				if err := json.Unmarshal(attemptsJSON, &attempts); err != nil {
					t.Fatalf("unmarshal attempts: %v", err)
				}
				if len(attempts) != 1 || attempts[0].SQL != "SELECT 1" {
					t.Fatalf("attempts = %+v", attempts)
				}
				retrievedJSON, ok := args[8].([]byte)
				if !ok {
					t.Fatalf("retrieved arg type = %T, want []byte", args[8])
				}
				var retrieved []string
				if err := json.Unmarshal(retrievedJSON, &retrieved); err != nil {
					t.Fatalf("unmarshal retrieved ids: %v", err)
				}
				if len(retrieved) != 1 || retrieved[0] != exampleID.String() {
					t.Fatalf("retrieved ids = %+v", retrieved)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = runID
					*(dest[1].(*uuid.UUID)) = uuid.New()
					*(dest[2].(*uuid.UUID)) = uuid.New()
					*(dest[3].(**uuid.UUID)) = nil
					*(dest[4].(*model.QueryPromptContextSource)) = model.QueryPromptContextSourceApproved
					*(dest[5].(*string)) = "user_123"
					*(dest[6].(*string)) = "질문"
					*(dest[7].(*model.QueryRunStatus)) = model.QueryRunStatusSucceeded
					*(dest[8].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT 1", Valid: true}
					*(dest[9].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT 1 LIMIT 1000", Valid: true}
					*(dest[10].(*int64)) = 1
					*(dest[11].(*int64)) = 22
					*(dest[12].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[13].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[14].(*time.Time)) = completedAt.Add(-time.Minute)
					*(dest[15].(**time.Time)) = &completedAt
					return nil
				}}
			},
		},
	}

	got, err := repo.CompleteSucceeded(
		context.Background(),
		runID,
		"SELECT 1",
		"SELECT 1 LIMIT 1000",
		[]model.QueryRunAttempt{{SQL: "SELECT 1", Stage: "execution"}},
		[]string{"warning"},
		1,
		22,
		[]uuid.UUID{exampleID},
		completedAt,
	)
	if err != nil {
		t.Fatalf("CompleteSucceeded returned error: %v", err)
	}
	if got.Status != model.QueryRunStatusSucceeded {
		t.Fatalf("Status = %q, want succeeded", got.Status)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(completedAt) {
		t.Fatalf("CompletedAt = %v, want %v", got.CompletedAt, completedAt)
	}
}

func TestTenantQueryFeedbackRepositoryUpsert(t *testing.T) {
	t.Parallel()

	runID := uuid.New()
	now := time.Unix(1_700_000_700, 0).UTC()
	repo := &TenantQueryFeedbackRepository{
		db: &fakeTenantDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "INSERT INTO tenant_query_feedback") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if args[0] != runID || args[1] != "user_123" || args[2] != string(model.QueryFeedbackRatingDown) {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = runID
					*(dest[1].(*string)) = "user_123"
					*(dest[2].(*model.QueryFeedbackRating)) = model.QueryFeedbackRatingDown
					*(dest[3].(*string)) = "설명이 부족합니다."
					*(dest[4].(*string)) = "SELECT AVG(ph) FROM readings"
					*(dest[5].(*time.Time)) = now
					*(dest[6].(*time.Time)) = now
					return nil
				}}
			},
		},
	}

	got, err := repo.Upsert(
		context.Background(),
		runID,
		"user_123",
		model.QueryFeedbackRatingDown,
		"설명이 부족합니다.",
		"SELECT AVG(ph) FROM readings",
		now,
	)
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if got.Rating != model.QueryFeedbackRatingDown || got.QueryRunID != runID {
		t.Fatalf("Upsert returned %+v", got)
	}
}

func TestTenantCanonicalQueryExampleRepositoryArchiveNotFound(t *testing.T) {
	t.Parallel()

	repo := &TenantCanonicalQueryExampleRepository{
		db: &fakeTenantDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, nil
			},
		},
	}

	err := repo.Archive(context.Background(), uuid.New(), uuid.New(), time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Archive error = %v, want ErrNotFound", err)
	}
}
