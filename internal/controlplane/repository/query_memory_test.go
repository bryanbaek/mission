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
					*(dest[10].(*[]byte)) = []byte("[]")
					*(dest[11].(*[]byte)) = []byte("[]")
					*(dest[12].(*int64)) = 0
					*(dest[13].(*int64)) = 0
					*(dest[14].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[15].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[16].(*time.Time)) = createdAt
					*(dest[17].(**time.Time)) = nil
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
					*(dest[10].(*[]byte)) = []byte(`[{"sql":"SELECT 1","error":"","stage":"execution"}]`)
					*(dest[11].(*[]byte)) = []byte(`["warning"]`)
					*(dest[12].(*int64)) = 1
					*(dest[13].(*int64)) = 22
					*(dest[14].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[15].(*dbsql.NullString)) = dbsql.NullString{}
					*(dest[16].(*time.Time)) = completedAt.Add(-time.Minute)
					*(dest[17].(**time.Time)) = &completedAt
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
	if len(got.Attempts) != 1 || got.Attempts[0].SQL != "SELECT 1" {
		t.Fatalf("Attempts = %+v", got.Attempts)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "warning" {
		t.Fatalf("Warnings = %+v", got.Warnings)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(completedAt) {
		t.Fatalf("CompletedAt = %v, want %v", got.CompletedAt, completedAt)
	}
}

func TestTenantQueryRunRepositoryListByTenantAndUser(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := "user_123"
	createdAtA := time.Unix(1_700_000_800, 0).UTC()
	createdAtB := createdAtA.Add(-time.Minute)
	completedAtA := createdAtA.Add(10 * time.Second)

	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = uuid.New()
				*(dest[1].(*uuid.UUID)) = tenantID
				*(dest[2].(*uuid.UUID)) = uuid.New()
				*(dest[3].(**uuid.UUID)) = nil
				*(dest[4].(*model.QueryPromptContextSource)) = model.QueryPromptContextSourceApproved
				*(dest[5].(*string)) = userID
				*(dest[6].(*string)) = "첫 번째 질문"
				*(dest[7].(*model.QueryRunStatus)) = model.QueryRunStatusSucceeded
				*(dest[8].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT 1", Valid: true}
				*(dest[9].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT 1 LIMIT 1000", Valid: true}
				*(dest[10].(*[]byte)) = []byte(`[{"sql":"SELECT 1","error":"","stage":"execution"}]`)
				*(dest[11].(*[]byte)) = []byte(`["warning-a"]`)
				*(dest[12].(*int64)) = 1
				*(dest[13].(*int64)) = 25
				*(dest[14].(*dbsql.NullString)) = dbsql.NullString{}
				*(dest[15].(*dbsql.NullString)) = dbsql.NullString{}
				*(dest[16].(*time.Time)) = createdAtA
				*(dest[17].(**time.Time)) = &completedAtA
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = uuid.New()
				*(dest[1].(*uuid.UUID)) = tenantID
				*(dest[2].(*uuid.UUID)) = uuid.New()
				*(dest[3].(**uuid.UUID)) = nil
				*(dest[4].(*model.QueryPromptContextSource)) = model.QueryPromptContextSourceRawSchema
				*(dest[5].(*string)) = userID
				*(dest[6].(*string)) = "두 번째 질문"
				*(dest[7].(*model.QueryRunStatus)) = model.QueryRunStatusFailed
				*(dest[8].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT missing", Valid: true}
				*(dest[9].(*dbsql.NullString)) = dbsql.NullString{}
				*(dest[10].(*[]byte)) = []byte(`[{"sql":"SELECT missing","error":"Unknown column","stage":"execution"}]`)
				*(dest[11].(*[]byte)) = []byte(`["warning-b"]`)
				*(dest[12].(*int64)) = 0
				*(dest[13].(*int64)) = 10
				*(dest[14].(*dbsql.NullString)) = dbsql.NullString{String: "execution", Valid: true}
				*(dest[15].(*dbsql.NullString)) = dbsql.NullString{String: "Unknown column", Valid: true}
				*(dest[16].(*time.Time)) = createdAtB
				*(dest[17].(**time.Time)) = nil
				return nil
			},
		},
	}
	repo := &TenantQueryRunRepository{
		db: &fakeTenantDB{
			queryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
				if !strings.Contains(sql, "FROM tenant_query_runs") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 3 {
					t.Fatalf("args len = %d, want 3", len(args))
				}
				if args[0] != tenantID || args[1] != userID || args[2] != 20 {
					t.Fatalf("unexpected args: %#v", args)
				}
				return rows, nil
			},
		},
	}

	got, err := repo.ListByTenantAndUser(context.Background(), tenantID, userID, 20)
	if err != nil {
		t.Fatalf("ListByTenantAndUser returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Question != "첫 번째 질문" || got[1].Question != "두 번째 질문" {
		t.Fatalf("questions = %+v", got)
	}
	if got[0].Status != model.QueryRunStatusSucceeded || got[1].Status != model.QueryRunStatusFailed {
		t.Fatalf("statuses = %+v", got)
	}
	if got[0].Attempts[0].SQL != "SELECT 1" || got[1].Attempts[0].Error != "Unknown column" {
		t.Fatalf("attempts = %+v", got)
	}
	if got[0].Warnings[0] != "warning-a" || got[1].Warnings[0] != "warning-b" {
		t.Fatalf("warnings = %+v", got)
	}
	if got[1].ErrorStage != "execution" || got[1].ErrorMessage != "Unknown column" {
		t.Fatalf("error fields = %+v", got[1])
	}
}

func TestTenantQueryRunRepositoryListReviewQueue(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	queryRunID := uuid.New()
	schemaVersionID := uuid.New()
	semanticLayerID := uuid.New()
	completedAt := time.Unix(1_700_001_010, 0).UTC()
	feedbackCreatedAt := time.Unix(1_700_001_020, 0).UTC()
	feedbackUpdatedAt := feedbackCreatedAt.Add(30 * time.Second)
	reviewSignalAt := feedbackUpdatedAt

	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = queryRunID
				*(dest[1].(*uuid.UUID)) = tenantID
				*(dest[2].(*uuid.UUID)) = schemaVersionID
				*(dest[3].(**uuid.UUID)) = &semanticLayerID
				*(dest[4].(*model.QueryPromptContextSource)) = model.QueryPromptContextSourceApproved
				*(dest[5].(*string)) = "user_123"
				*(dest[6].(*string)) = "평균 pH는?"
				*(dest[7].(*model.QueryRunStatus)) = model.QueryRunStatusFailed
				*(dest[8].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT bad", Valid: true}
				*(dest[9].(*dbsql.NullString)) = dbsql.NullString{}
				*(dest[10].(*[]byte)) = []byte(`[{"sql":"SELECT bad","error":"Unknown column","stage":"execution"}]`)
				*(dest[11].(*[]byte)) = []byte(`["warning-a"]`)
				*(dest[12].(*int64)) = 0
				*(dest[13].(*int64)) = 27
				*(dest[14].(*dbsql.NullString)) = dbsql.NullString{String: "execution", Valid: true}
				*(dest[15].(*dbsql.NullString)) = dbsql.NullString{String: "Unknown column", Valid: true}
				*(dest[16].(*time.Time)) = completedAt.Add(-time.Minute)
				*(dest[17].(**time.Time)) = &completedAt
				*(dest[18].(*dbsql.NullString)) = dbsql.NullString{String: "user_456", Valid: true}
				*(dest[19].(*dbsql.NullString)) = dbsql.NullString{String: string(model.QueryFeedbackRatingDown), Valid: true}
				*(dest[20].(*dbsql.NullString)) = dbsql.NullString{String: "조인이 잘못됐습니다.", Valid: true}
				*(dest[21].(*dbsql.NullString)) = dbsql.NullString{String: "SELECT AVG(ph) FROM readings", Valid: true}
				*(dest[22].(*dbsql.NullTime)) = dbsql.NullTime{Time: feedbackCreatedAt, Valid: true}
				*(dest[23].(*dbsql.NullTime)) = dbsql.NullTime{Time: feedbackUpdatedAt, Valid: true}
				*(dest[24].(*bool)) = false
				*(dest[25].(**time.Time)) = nil
				*(dest[26].(*time.Time)) = reviewSignalAt
				return nil
			},
		},
	}
	repo := &TenantQueryRunRepository{
		db: &fakeTenantDB{
			queryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
				if !strings.Contains(sql, "WITH latest_feedback AS") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if !strings.Contains(sql, "r.reviewed_at IS NULL") {
					t.Fatalf("open queue SQL missing review filter: %q", sql)
				}
				if !strings.Contains(sql, "active_example.source_query_run_id IS NULL") {
					t.Fatalf("open queue SQL missing example filter: %q", sql)
				}
				if len(args) != 2 || args[0] != tenantID || args[1] != 50 {
					t.Fatalf("unexpected args: %#v", args)
				}
				return rows, nil
			},
		},
	}

	got, err := repo.ListReviewQueue(
		context.Background(),
		tenantID,
		model.ReviewQueueFilterOpen,
		50,
	)
	if err != nil {
		t.Fatalf("ListReviewQueue returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	item := got[0]
	if item.Run.ID != queryRunID || !item.HasFeedback || item.LatestFeedback == nil {
		t.Fatalf("review item = %+v", item)
	}
	if item.LatestFeedback.Rating != model.QueryFeedbackRatingDown {
		t.Fatalf("latest feedback = %+v", item.LatestFeedback)
	}
	if item.HasActiveCanonicalExample {
		t.Fatalf("HasActiveCanonicalExample = true, want false")
	}
	if item.ReviewedAt != nil {
		t.Fatalf("ReviewedAt = %v, want nil", item.ReviewedAt)
	}
	if !item.ReviewSignalAt.Equal(reviewSignalAt) {
		t.Fatalf("ReviewSignalAt = %v, want %v", item.ReviewSignalAt, reviewSignalAt)
	}
}

func TestTenantQueryRunRepositoryMarkReviewed(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	queryRunID := uuid.New()
	reviewedAt := time.Unix(1_700_001_100, 0).UTC()
	repo := &TenantQueryRunRepository{
		db: &fakeTenantDB{
			execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if !strings.Contains(sql, "UPDATE tenant_query_runs") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 4 {
					t.Fatalf("args len = %d, want 4", len(args))
				}
				if args[0] != tenantID || args[1] != queryRunID {
					t.Fatalf("unexpected ids: %#v", args)
				}
				if args[3] != "owner_123" {
					t.Fatalf("reviewed_by_user_id = %#v, want owner_123", args[3])
				}
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		},
	}

	if err := repo.MarkReviewed(
		context.Background(),
		tenantID,
		queryRunID,
		reviewedAt,
		"owner_123",
	); err != nil {
		t.Fatalf("MarkReviewed returned error: %v", err)
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
