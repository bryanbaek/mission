package controller

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeQueryMembership struct {
	ensureFn func(context.Context, uuid.UUID, string) (model.TenantUser, error)
}

func (f fakeQueryMembership) EnsureMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (model.TenantUser, error) {
	return f.ensureFn(ctx, tenantID, clerkUserID)
}

type fakeQuerySchemaStore struct {
	latestFn func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error)
}

func (f fakeQuerySchemaStore) LatestByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantSchemaVersion, error) {
	return f.latestFn(ctx, tenantID)
}

type fakeQueryLayerStore struct {
	approvedFn func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error)
	draftFn    func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error)
}

func (f fakeQueryLayerStore) LatestApprovedBySchemaVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return f.approvedFn(ctx, tenantID, schemaVersionID)
}

func (f fakeQueryLayerStore) LatestDraftBySchemaVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return f.draftFn(ctx, tenantID, schemaVersionID)
}

type fakeQueryRunStore struct {
	createFn            func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error)
	getFn               func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error)
	listFn              func(context.Context, uuid.UUID, string, int) ([]model.TenantQueryRun, error)
	listReviewFn        func(context.Context, uuid.UUID, model.ReviewQueueFilter, int) ([]model.TenantQueryRunReviewItem, error)
	markReviewedFn      func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) error
	completeSucceededFn func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error)
	completeFailedFn    func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error)
}

func (f fakeQueryRunStore) Create(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
	semanticLayerID *uuid.UUID,
	source model.QueryPromptContextSource,
	clerkUserID string,
	question string,
) (model.TenantQueryRun, error) {
	return f.createFn(ctx, tenantID, schemaVersionID, semanticLayerID, source, clerkUserID, question)
}

func (f fakeQueryRunStore) GetByTenantAndID(
	ctx context.Context,
	tenantID, id uuid.UUID,
) (model.TenantQueryRun, error) {
	return f.getFn(ctx, tenantID, id)
}

func (f fakeQueryRunStore) ListByTenantAndUser(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	limit int,
) ([]model.TenantQueryRun, error) {
	if f.listFn == nil {
		return nil, errors.New("unexpected ListByTenantAndUser")
	}
	return f.listFn(ctx, tenantID, clerkUserID, limit)
}

func (f fakeQueryRunStore) ListReviewQueue(
	ctx context.Context,
	tenantID uuid.UUID,
	filter model.ReviewQueueFilter,
	limit int,
) ([]model.TenantQueryRunReviewItem, error) {
	if f.listReviewFn == nil {
		return nil, errors.New("unexpected ListReviewQueue")
	}
	return f.listReviewFn(ctx, tenantID, filter, limit)
}

func (f fakeQueryRunStore) MarkReviewed(
	ctx context.Context,
	tenantID, id uuid.UUID,
	reviewedAt time.Time,
	reviewedByUserID string,
) error {
	if f.markReviewedFn == nil {
		return errors.New("unexpected MarkReviewed")
	}
	return f.markReviewedFn(ctx, tenantID, id, reviewedAt, reviewedByUserID)
}

func (f fakeQueryRunStore) CompleteSucceeded(
	ctx context.Context,
	id uuid.UUID,
	sqlOriginal, sqlExecuted string,
	attempts []model.QueryRunAttempt,
	warnings []string,
	rowCount, elapsedMS int64,
	retrievedExampleIDs []uuid.UUID,
	completedAt time.Time,
) (model.TenantQueryRun, error) {
	return f.completeSucceededFn(ctx, id, sqlOriginal, sqlExecuted, attempts, warnings, rowCount, elapsedMS, retrievedExampleIDs, completedAt)
}

func (f fakeQueryRunStore) CompleteFailed(
	ctx context.Context,
	id uuid.UUID,
	attempts []model.QueryRunAttempt,
	warnings []string,
	retrievedExampleIDs []uuid.UUID,
	errorStage, errorMessage string,
	completedAt time.Time,
) (model.TenantQueryRun, error) {
	return f.completeFailedFn(ctx, id, attempts, warnings, retrievedExampleIDs, errorStage, errorMessage, completedAt)
}

type fakeQueryFeedbackStore struct {
	upsertFn func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error)
}

func (f fakeQueryFeedbackStore) Upsert(
	ctx context.Context,
	queryRunID uuid.UUID,
	clerkUserID string,
	rating model.QueryFeedbackRating,
	comment, correctedSQL string,
	now time.Time,
) (model.TenantQueryFeedback, error) {
	return f.upsertFn(ctx, queryRunID, clerkUserID, rating, comment, correctedSQL, now)
}

type searchCall struct {
	question        string
	schemaVersionID *uuid.UUID
}

type fakeQueryExampleStore struct {
	createFn  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, string) (model.TenantCanonicalQueryExample, error)
	listFn    func(context.Context, uuid.UUID, int) ([]model.TenantCanonicalQueryExample, error)
	searchFn  func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error)
	archiveFn func(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	searches  []searchCall
}

func (f *fakeQueryExampleStore) Create(
	ctx context.Context,
	tenantID, schemaVersionID, sourceQueryRunID uuid.UUID,
	createdByUserID, question, sql, notes string,
) (model.TenantCanonicalQueryExample, error) {
	return f.createFn(ctx, tenantID, schemaVersionID, sourceQueryRunID, createdByUserID, question, sql, notes)
}

func (f *fakeQueryExampleStore) ListActiveByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
	limit int,
) ([]model.TenantCanonicalQueryExample, error) {
	return f.listFn(ctx, tenantID, limit)
}

func (f *fakeQueryExampleStore) SearchActiveByQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	question string,
	limit int,
	schemaVersionID *uuid.UUID,
) ([]model.TenantCanonicalQueryExample, error) {
	f.searches = append(f.searches, searchCall{
		question:        question,
		schemaVersionID: schemaVersionID,
	})
	return f.searchFn(ctx, tenantID, question, limit, schemaVersionID)
}

func (f *fakeQueryExampleStore) Archive(
	ctx context.Context,
	tenantID, id uuid.UUID,
	archivedAt time.Time,
) error {
	return f.archiveFn(ctx, tenantID, id, archivedAt)
}

type fakeQueryAgent struct {
	executeFn func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error)
	calls     []string
}

func (f *fakeQueryAgent) ExecuteQuery(
	ctx context.Context,
	tenantID uuid.UUID,
	sql string,
) (AgentExecuteQueryResult, error) {
	f.calls = append(f.calls, sql)
	return f.executeFn(ctx, tenantID, sql)
}

type fakeQueryCompleter struct {
	responses []llm.CompletionResponse
	errs      []error
	calls     []llm.CompletionRequest
}

func (f *fakeQueryCompleter) Complete(
	_ context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	f.calls = append(f.calls, req)
	idx := len(f.calls) - 1
	if idx < len(f.errs) && f.errs[idx] != nil {
		return llm.CompletionResponse{}, f.errs[idx]
	}
	if idx >= len(f.responses) {
		return llm.CompletionResponse{}, errors.New("no fake response configured")
	}
	return f.responses[idx], nil
}

func queryTestSchema() ([]byte, model.SchemaBlob) {
	blob := model.SchemaBlob{
		DatabaseName: "mission_app",
		Tables: []model.SchemaTable{{
			TableSchema: "mission_app",
			TableName:   "readings",
			TableType:   "BASE TABLE",
		}},
		Columns: []model.SchemaColumn{
			{
				TableSchema:     "mission_app",
				TableName:       "readings",
				ColumnName:      "station_id",
				OrdinalPosition: 1,
				DataType:        "int",
				ColumnType:      "int(11)",
			},
			{
				TableSchema:     "mission_app",
				TableName:       "readings",
				ColumnName:      "ph",
				OrdinalPosition: 2,
				DataType:        "decimal",
				ColumnType:      "decimal(5,2)",
			},
		},
	}
	raw, _ := json.Marshal(blob)
	return raw, blob
}

func queryTestSemanticLayer(
	t *testing.T,
	tenantID, schemaVersionID uuid.UUID,
	status model.SemanticLayerStatus,
) model.TenantSemanticLayer {
	t.Helper()
	content := model.SemanticLayerContent{
		Tables: []model.SemanticTable{{
			TableSchema: "mission_app",
			TableName:   "readings",
			Description: "측정소의 수질 측정값",
			Columns: []model.SemanticColumn{
				{
					TableSchema: "mission_app",
					TableName:   "readings",
					ColumnName:  "station_id",
					Description: "측정소 ID",
				},
				{
					TableSchema: "mission_app",
					TableName:   "readings",
					ColumnName:  "ph",
					Description: "pH 값",
				},
			},
		}},
	}
	body, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	return model.TenantSemanticLayer{
		ID:              uuid.New(),
		TenantID:        tenantID,
		SchemaVersionID: schemaVersionID,
		Status:          status,
		Content:         body,
		CreatedAt:       time.Unix(1_700_000_000, 0).UTC(),
	}
}

func completionJSON(t *testing.T, sql string) llm.CompletionResponse {
	t.Helper()
	body, err := json.Marshal(generatedSQL{
		Reasoning: "테스트",
		SQL:       sql,
		Notes:     "",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return llm.CompletionResponse{
		Content:  string(body),
		Provider: "fake",
		Model:    "fake-model",
	}
}

func newTestQueryController(
	membership queryMembershipCheckerCtl,
	schemas querySchemaStoreCtl,
	layers querySemanticLayerStoreCtl,
	runs queryRunStoreCtl,
	feedback queryFeedbackStoreCtl,
	examples queryCanonicalExampleStoreCtl,
	agent queryAgentExecutor,
	completer queryCompleter,
) *QueryController {
	return NewQueryController(
		membership,
		schemas,
		layers,
		runs,
		feedback,
		examples,
		agent,
		completer,
		QueryControllerConfig{
			Model:                "fake-model",
			MaxTokens:            1024,
			SummaryModel:         "fake-model",
			SummaryMaxTokens:     512,
			MaxSummaryRows:       10,
			MaxRetrievedExamples: 3,
			MaxCanonicalExamples: 20,
		},
	)
}

func TestQueryControllerAskQuestionPersistsSuccessfulRun(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	queryRunID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)
	exampleID := uuid.New()

	agent := &fakeQueryAgent{
		executeFn: func(_ context.Context, _ uuid.UUID, sql string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{
				Columns:   []string{"station_id", "avg_ph"},
				Rows:      []map[string]any{{"station_id": int64(1), "avg_ph": 7.2}},
				ElapsedMS: 42,
			}, nil
		},
	}
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "SELECT station_id, AVG(ph) AS avg_ph FROM mission_app.readings GROUP BY station_id"),
			{Content: "측정소 1의 pH 평균은 7.2입니다."},
		},
	}
	examples := &fakeQueryExampleStore{
		searchFn: func(_ context.Context, _ uuid.UUID, _ string, _ int, schemaVersionID *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
			if schemaVersionID == nil {
				t.Fatal("expected same-schema search")
			}
			return []model.TenantCanonicalQueryExample{{
				ID:               exampleID,
				SourceQueryRunID: uuid.New(),
				SchemaVersionID:  *schemaVersionID,
				Question:         "측정소별 평균 pH는?",
				SQL:              "SELECT station_id, AVG(ph) AS avg_ph FROM mission_app.readings GROUP BY station_id",
				Notes:            "기본 집계 예시",
				CreatedAt:        time.Unix(1_700_000_100, 0).UTC(),
			}}, nil
		},
	}
	var completedRetrievedIDs []uuid.UUID

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, TenantID: tenantID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("should not call draft")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{ID: queryRunID, TenantID: tenantID, SchemaVersionID: schemaVersionID}, nil
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(_ context.Context, id uuid.UUID, _ string, _ string, _ []model.QueryRunAttempt, _ []string, _ int64, _ int64, retrievedExampleIDs []uuid.UUID, _ time.Time) (model.TenantQueryRun, error) {
				if id != queryRunID {
					t.Fatalf("run id = %s, want %s", id, queryRunID)
				}
				completedRetrievedIDs = append([]uuid.UUID(nil), retrievedExampleIDs...)
				return model.TenantQueryRun{ID: id}, nil
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		examples,
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "측정소별 pH 평균은?")
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	if got.QueryRunID != queryRunID {
		t.Fatalf("QueryRunID = %s, want %s", got.QueryRunID, queryRunID)
	}
	if len(completedRetrievedIDs) != 1 || completedRetrievedIDs[0] != exampleID {
		t.Fatalf("retrieved ids = %+v", completedRetrievedIDs)
	}
	if len(completer.calls) == 0 {
		t.Fatal("expected completer to be called")
	}
	if len(completer.calls) != 2 {
		t.Fatalf("completer calls = %d, want 2", len(completer.calls))
	}
	if completer.calls[0].Operation != "query.generate_sql" {
		t.Fatalf("generation operation = %q, want query.generate_sql", completer.calls[0].Operation)
	}
	if completer.calls[1].Operation != "query.summarize" {
		t.Fatalf("summary operation = %q, want query.summarize", completer.calls[1].Operation)
	}
	cached := completer.calls[0].Messages[0].CachedContent
	if strings.Count(cached, "## 승인된 예시 쿼리") != 1 {
		t.Fatalf("cached prompt should include approved examples block once, got:\n%s", cached)
	}
	if !strings.Contains(cached, "기본 집계 예시") {
		t.Fatalf("cached prompt missing example notes: %s", cached)
	}
}

func TestQueryControllerAskQuestionPersistsFailedRun(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	queryRunID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "DELETE FROM mission_app.readings"),
			completionJSON(t, "UPDATE mission_app.readings SET ph = 1"),
		},
	}
	var failedAttempts []model.QueryRunAttempt

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, TenantID: tenantID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{ID: queryRunID, TenantID: tenantID, SchemaVersionID: schemaVersionID}, nil
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(_ context.Context, id uuid.UUID, attempts []model.QueryRunAttempt, _ []string, _ []uuid.UUID, errorStage, _ string, _ time.Time) (model.TenantQueryRun, error) {
				if id != queryRunID {
					t.Fatalf("run id = %s, want %s", id, queryRunID)
				}
				if errorStage != "validation" {
					t.Fatalf("error stage = %q, want validation", errorStage)
				}
				failedAttempts = append([]model.QueryRunAttempt(nil), attempts...)
				return model.TenantQueryRun{ID: id}, nil
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, nil
			},
		},
		&fakeQueryAgent{
			executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
				return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
			},
		},
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "지워줘")
	if !errors.Is(err, ErrQueryAllAttemptsFailed) {
		t.Fatalf("AskQuestion error = %v, want ErrQueryAllAttemptsFailed", err)
	}
	if got.QueryRunID != queryRunID {
		t.Fatalf("QueryRunID = %s, want %s", got.QueryRunID, queryRunID)
	}
	if len(failedAttempts) != 2 {
		t.Fatalf("failed attempts = %+v", failedAttempts)
	}
}

func TestQueryControllerAskQuestionFallsBackToTenantWideExamples(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)
	examples := &fakeQueryExampleStore{
		searchFn: func(_ context.Context, _ uuid.UUID, _ string, _ int, schemaVersionID *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
			if schemaVersionID != nil {
				return nil, nil
			}
			return []model.TenantCanonicalQueryExample{{
				ID:              uuid.New(),
				SchemaVersionID: uuid.New(),
				Question:        "전체 평균 pH는?",
				SQL:             "SELECT AVG(ph) FROM mission_app.readings",
				CreatedAt:       time.Unix(1_700_000_200, 0).UTC(),
			}}, nil
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, TenantID: tenantID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("should not call draft")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{ID: uuid.New(), TenantID: tenantID, SchemaVersionID: schemaVersionID}, nil
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, nil
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		examples,
		&fakeQueryAgent{
			executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
				return AgentExecuteQueryResult{
					Columns:   []string{"avg_ph"},
					Rows:      []map[string]any{{"avg_ph": 7.1}},
					ElapsedMS: 18,
				}, nil
			},
		},
		&fakeQueryCompleter{
			responses: []llm.CompletionResponse{
				completionJSON(t, "SELECT AVG(ph) AS avg_ph FROM mission_app.readings"),
				{Content: "평균 pH는 7.1입니다."},
			},
		},
	)

	if _, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "전체 평균 pH는?"); err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	if len(examples.searches) != 2 {
		t.Fatalf("search count = %d, want 2", len(examples.searches))
	}
	if examples.searches[0].schemaVersionID == nil {
		t.Fatal("first search should be same-schema")
	}
	if examples.searches[1].schemaVersionID != nil {
		t.Fatal("second search should be tenant-wide fallback")
	}
}

func TestQueryControllerAskQuestionFailsRunWhenExampleLookupFails(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	queryRunID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(
		t,
		tenantID,
		schemaVersionID,
		model.SemanticLayerStatusApproved,
	)
	var failedStage string

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{
				ID:       schemaVersionID,
				TenantID: tenantID,
				Blob:     schemaRaw,
			}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("should not call draft")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{
					ID:              queryRunID,
					TenantID:        tenantID,
					SchemaVersionID: schemaVersionID,
				}, nil
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(_ context.Context, id uuid.UUID, attempts []model.QueryRunAttempt, _ []string, retrievedExampleIDs []uuid.UUID, errorStage, errorMessage string, _ time.Time) (model.TenantQueryRun, error) {
				if id != queryRunID {
					t.Fatalf("run id = %s, want %s", id, queryRunID)
				}
				if len(attempts) != 0 {
					t.Fatalf("attempts = %+v, want no attempts", attempts)
				}
				if len(retrievedExampleIDs) != 0 {
					t.Fatalf("retrievedExampleIDs = %+v, want empty", retrievedExampleIDs)
				}
				if !strings.Contains(errorMessage, "example lookup failed") {
					t.Fatalf("error message = %q", errorMessage)
				}
				failedStage = errorStage
				return model.TenantQueryRun{ID: id}, nil
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("example lookup failed")
			},
		},
		&fakeQueryAgent{
			executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
				return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
			},
		},
		&fakeQueryCompleter{},
	)

	got, err := ctrl.AskQuestion(
		context.Background(),
		tenantID,
		"user_1",
		"승인된 예시를 찾아줘",
	)
	if err == nil || !strings.Contains(err.Error(), "search canonical examples") {
		t.Fatalf("AskQuestion error = %v, want canonical-example lookup error", err)
	}
	if got.QueryRunID != queryRunID {
		t.Fatalf("QueryRunID = %s, want %s", got.QueryRunID, queryRunID)
	}
	if failedStage != "generation" {
		t.Fatalf("failed stage = %q, want generation", failedStage)
	}
}

func TestQueryControllerAskQuestionReturnsAgentOffline(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	queryRunID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(
		t,
		tenantID,
		schemaVersionID,
		model.SemanticLayerStatusApproved,
	)
	var failedStage string

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{
				ID:       schemaVersionID,
				TenantID: tenantID,
				Blob:     schemaRaw,
			}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("should not call draft")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{
					ID:              queryRunID,
					TenantID:        tenantID,
					SchemaVersionID: schemaVersionID,
				}, nil
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(_ context.Context, id uuid.UUID, attempts []model.QueryRunAttempt, _ []string, _ []uuid.UUID, errorStage, errorMessage string, _ time.Time) (model.TenantQueryRun, error) {
				if id != queryRunID {
					t.Fatalf("run id = %s, want %s", id, queryRunID)
				}
				if len(attempts) != 1 || attempts[0].Stage != "execution" {
					t.Fatalf("attempts = %+v, want execution attempt", attempts)
				}
				if errorMessage != ErrTenantNotConnected.Error() {
					t.Fatalf("error message = %q, want %q", errorMessage, ErrTenantNotConnected.Error())
				}
				failedStage = errorStage
				return model.TenantQueryRun{ID: id}, nil
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, nil
			},
		},
		&fakeQueryAgent{
			executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
				return AgentExecuteQueryResult{}, ErrTenantNotConnected
			},
		},
		&fakeQueryCompleter{
			responses: []llm.CompletionResponse{
				completionJSON(t, "SELECT AVG(ph) FROM mission_app.readings"),
			},
		},
	)

	got, err := ctrl.AskQuestion(
		context.Background(),
		tenantID,
		"user_1",
		"에이전트가 연결되지 않았나요?",
	)
	if !errors.Is(err, ErrQueryAgentOffline) {
		t.Fatalf("AskQuestion error = %v, want ErrQueryAgentOffline", err)
	}
	if got.QueryRunID != queryRunID {
		t.Fatalf("QueryRunID = %s, want %s", got.QueryRunID, queryRunID)
	}
	if failedStage != "execution" {
		t.Fatalf("failed stage = %q, want execution", failedStage)
	}
}

func TestBuildQueryUserPromptIncludesApprovedExamplesOnce(t *testing.T) {
	t.Parallel()

	rawSchema, _ := queryTestSchema()
	cached, _ := buildQueryUserPrompt(
		"평균 pH는?",
		queryPromptContext{
			schemaRaw: rawSchema,
			source:    model.QueryPromptContextSourceApproved,
		},
		[]model.TenantCanonicalQueryExample{{
			ID:       uuid.New(),
			Question: "측정소별 평균 pH는?",
			SQL:      "SELECT station_id, AVG(ph) FROM readings GROUP BY station_id",
			Notes:    "대표 예시",
		}},
		"",
		"",
	)

	if strings.Count(cached, "## 승인된 예시 쿼리") != 1 {
		t.Fatalf("cached prompt should include approved examples block once, got:\n%s", cached)
	}
	if !strings.Contains(cached, "대표 예시") {
		t.Fatalf("cached prompt missing example notes: %s", cached)
	}
}

func TestQueryControllerListMyQueryRunsRequiresMembership(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, repository.ErrNotFound
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
	)

	_, err := ctrl.ListMyQueryRuns(context.Background(), tenantID, "user_123", 0)
	if !errors.Is(err, ErrQueryAccessDenied) {
		t.Fatalf("ListMyQueryRuns error = %v, want ErrQueryAccessDenied", err)
	}
}

func TestQueryControllerListMyQueryRunsReturnsPersonalRunsWithDefaultLimit(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	runID := uuid.New()
	var (
		gotTenantID   uuid.UUID
		gotClerkUser  string
		gotLimit      int
		returnedRunAt = time.Unix(1_700_002_000, 0).UTC()
	)
	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			listFn: func(_ context.Context, tenantID uuid.UUID, clerkUserID string, limit int) ([]model.TenantQueryRun, error) {
				gotTenantID = tenantID
				gotClerkUser = clerkUserID
				gotLimit = limit
				return []model.TenantQueryRun{{
					ID:                  runID,
					TenantID:            tenantID,
					ClerkUserID:         clerkUserID,
					Question:            "최근 30일 평균 pH는?",
					Status:              model.QueryRunStatusSucceeded,
					PromptContextSource: model.QueryPromptContextSourceApproved,
					SQLOriginal:         "SELECT AVG(ph) FROM readings",
					SQLExecuted:         "SELECT AVG(ph) FROM readings LIMIT 1000",
					Attempts: []model.QueryRunAttempt{{
						SQL:   "SELECT AVG(ph) FROM readings",
						Stage: "execution",
					}},
					Warnings:  []string{"warning"},
					RowCount:  1,
					ElapsedMS: 42,
					CreatedAt: returnedRunAt,
				}}, nil
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
	)

	result, err := ctrl.ListMyQueryRuns(context.Background(), tenantID, "user_123", 0)
	if err != nil {
		t.Fatalf("ListMyQueryRuns returned error: %v", err)
	}
	if gotTenantID != tenantID || gotClerkUser != "user_123" {
		t.Fatalf("list args = tenant:%s user:%q", gotTenantID, gotClerkUser)
	}
	if gotLimit != defaultListMyQueryRunsLimit {
		t.Fatalf("limit = %d, want %d", gotLimit, defaultListMyQueryRunsLimit)
	}
	if len(result.Runs) != 1 || result.Runs[0].ID != runID {
		t.Fatalf("runs = %+v", result.Runs)
	}
	if result.Runs[0].Question != "최근 30일 평균 pH는?" {
		t.Fatalf("question = %q", result.Runs[0].Question)
	}
}

func TestQueryControllerSubmitFeedbackRequiresOriginalUser(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	runID := uuid.New()
	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{
					ID:          runID,
					TenantID:    tenantID,
					ClerkUserID: "other_user",
				}, nil
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
	)

	_, err := ctrl.SubmitFeedback(
		context.Background(),
		tenantID,
		"user_123",
		runID,
		model.QueryFeedbackRatingUp,
		"",
		"",
	)
	if !errors.Is(err, ErrQueryFeedbackAccessDenied) {
		t.Fatalf("SubmitFeedback error = %v, want ErrQueryFeedbackAccessDenied", err)
	}
}

func TestQueryControllerCreateCanonicalExampleRequiresOwner(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
			createFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, string) (model.TenantCanonicalQueryExample, error) {
				return model.TenantCanonicalQueryExample{}, errors.New("unexpected Create")
			},
			listFn: func(context.Context, uuid.UUID, int) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected ListActiveByTenant")
			},
			archiveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
				return errors.New("unexpected Archive")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
	)

	_, err := ctrl.CreateCanonicalExample(
		context.Background(),
		tenantID,
		"user_123",
		uuid.New(),
		"평균 pH는?",
		"SELECT AVG(ph) FROM readings",
		"",
	)
	if !errors.Is(err, ErrCanonicalQueryExampleOwnerOnly) {
		t.Fatalf("CreateCanonicalExample error = %v, want ErrCanonicalQueryExampleOwnerOnly", err)
	}
}

func TestQueryControllerListReviewQueueRequiresOwner(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
			createFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, string) (model.TenantCanonicalQueryExample, error) {
				return model.TenantCanonicalQueryExample{}, errors.New("unexpected Create")
			},
			listFn: func(context.Context, uuid.UUID, int) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected ListActiveByTenant")
			},
			archiveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
				return errors.New("unexpected Archive")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
	)

	_, err := ctrl.ListReviewQueue(
		context.Background(),
		tenantID,
		"user_123",
		model.ReviewQueueFilterOpen,
		0,
	)
	if !errors.Is(err, ErrQueryReviewOwnerOnly) {
		t.Fatalf("ListReviewQueue error = %v, want ErrQueryReviewOwnerOnly", err)
	}
}

func TestQueryControllerListReviewQueueReturnsOwnerItemsWithDefaultLimit(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	queryRunID := uuid.New()
	var gotFilter model.ReviewQueueFilter
	var gotLimit int
	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleOwner}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected GetByTenantAndID")
			},
			listReviewFn: func(_ context.Context, gotTenantID uuid.UUID, filter model.ReviewQueueFilter, limit int) ([]model.TenantQueryRunReviewItem, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				gotFilter = filter
				gotLimit = limit
				return []model.TenantQueryRunReviewItem{{
					Run: model.TenantQueryRun{
						ID:       queryRunID,
						TenantID: tenantID,
						Question: "평균 pH는?",
						Status:   model.QueryRunStatusFailed,
						CreatedAt: time.Unix(
							1_700_001_200,
							0,
						).UTC(),
					},
				}}, nil
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
			createFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string, string) (model.TenantCanonicalQueryExample, error) {
				return model.TenantCanonicalQueryExample{}, errors.New("unexpected Create")
			},
			listFn: func(context.Context, uuid.UUID, int) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected ListActiveByTenant")
			},
			archiveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
				return errors.New("unexpected Archive")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
	)

	result, err := ctrl.ListReviewQueue(
		context.Background(),
		tenantID,
		"user_123",
		"",
		0,
	)
	if err != nil {
		t.Fatalf("ListReviewQueue returned error: %v", err)
	}
	if gotFilter != model.ReviewQueueFilterOpen {
		t.Fatalf("filter = %q, want open", gotFilter)
	}
	if gotLimit != defaultReviewQueueLimit {
		t.Fatalf("limit = %d, want %d", gotLimit, defaultReviewQueueLimit)
	}
	if len(result.Items) != 1 || result.Items[0].Run.ID != queryRunID {
		t.Fatalf("items = %+v", result.Items)
	}
}

func TestQueryControllerCreateCanonicalExampleMarksRunReviewed(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	queryRunID := uuid.New()
	schemaVersionID := uuid.New()
	now := time.Unix(1_700_001_300, 0).UTC()
	var markedReviewed bool

	ctrl := NewQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{TenantID: tenantID, Role: model.RoleOwner}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant")
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion")
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion")
			},
		},
		fakeQueryRunStore{
			createFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, model.QueryPromptContextSource, string, string) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected Create")
			},
			getFn: func(_ context.Context, gotTenantID uuid.UUID, gotQueryRunID uuid.UUID) (model.TenantQueryRun, error) {
				if gotTenantID != tenantID || gotQueryRunID != queryRunID {
					t.Fatalf("unexpected ids: %s / %s", gotTenantID, gotQueryRunID)
				}
				return model.TenantQueryRun{
					ID:              queryRunID,
					TenantID:        tenantID,
					SchemaVersionID: schemaVersionID,
				}, nil
			},
			markReviewedFn: func(_ context.Context, gotTenantID, gotQueryRunID uuid.UUID, reviewedAt time.Time, reviewedByUserID string) error {
				if gotTenantID != tenantID || gotQueryRunID != queryRunID {
					t.Fatalf("unexpected ids: %s / %s", gotTenantID, gotQueryRunID)
				}
				if reviewedByUserID != "owner_123" {
					t.Fatalf("reviewedByUserID = %q", reviewedByUserID)
				}
				if !reviewedAt.Equal(now) {
					t.Fatalf("reviewedAt = %v, want %v", reviewedAt, now)
				}
				markedReviewed = true
				return nil
			},
			listReviewFn: func(context.Context, uuid.UUID, model.ReviewQueueFilter, int) ([]model.TenantQueryRunReviewItem, error) {
				return nil, errors.New("unexpected ListReviewQueue")
			},
			completeSucceededFn: func(context.Context, uuid.UUID, string, string, []model.QueryRunAttempt, []string, int64, int64, []uuid.UUID, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteSucceeded")
			},
			completeFailedFn: func(context.Context, uuid.UUID, []model.QueryRunAttempt, []string, []uuid.UUID, string, string, time.Time) (model.TenantQueryRun, error) {
				return model.TenantQueryRun{}, errors.New("unexpected CompleteFailed")
			},
		},
		fakeQueryFeedbackStore{
			upsertFn: func(context.Context, uuid.UUID, string, model.QueryFeedbackRating, string, string, time.Time) (model.TenantQueryFeedback, error) {
				return model.TenantQueryFeedback{}, errors.New("unexpected Upsert")
			},
		},
		&fakeQueryExampleStore{
			searchFn: func(context.Context, uuid.UUID, string, int, *uuid.UUID) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected SearchActiveByQuestion")
			},
			createFn: func(_ context.Context, gotTenantID, gotSchemaVersionID, gotSourceQueryRunID uuid.UUID, createdByUserID, question, sql, notes string) (model.TenantCanonicalQueryExample, error) {
				if gotTenantID != tenantID || gotSchemaVersionID != schemaVersionID || gotSourceQueryRunID != queryRunID {
					t.Fatalf("unexpected create ids: %s / %s / %s", gotTenantID, gotSchemaVersionID, gotSourceQueryRunID)
				}
				if createdByUserID != "owner_123" || question != "평균 pH는?" || sql != "SELECT AVG(ph) FROM readings" || notes != "기본 집계" {
					t.Fatalf("unexpected create payload: %q / %q / %q / %q", createdByUserID, question, sql, notes)
				}
				return model.TenantCanonicalQueryExample{ID: uuid.New(), SourceQueryRunID: gotSourceQueryRunID}, nil
			},
			listFn: func(context.Context, uuid.UUID, int) ([]model.TenantCanonicalQueryExample, error) {
				return nil, errors.New("unexpected ListActiveByTenant")
			},
			archiveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
				return errors.New("unexpected Archive")
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{}, errors.New("unexpected ExecuteQuery")
		}},
		&fakeQueryCompleter{},
		QueryControllerConfig{
			Now:                  func() time.Time { return now },
			Model:                "fake-model",
			MaxTokens:            1024,
			SummaryModel:         "fake-model",
			SummaryMaxTokens:     512,
			MaxSummaryRows:       10,
			MaxRetrievedExamples: 3,
			MaxCanonicalExamples: 20,
		},
	)

	if _, err := ctrl.CreateCanonicalExample(
		context.Background(),
		tenantID,
		"owner_123",
		queryRunID,
		"평균 pH는?",
		"SELECT AVG(ph) FROM readings",
		"기본 집계",
	); err != nil {
		t.Fatalf("CreateCanonicalExample returned error: %v", err)
	}
	if !markedReviewed {
		t.Fatal("expected MarkReviewed to be called")
	}
}
