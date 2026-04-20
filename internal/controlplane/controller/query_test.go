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
	"github.com/bryanbaek/mission/internal/queryerror"
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

// fakeQueryCompleter returns responses in order. Tests set responses one
// per LLM call expected during the run.
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
	agent queryAgentExecutor,
	completer queryCompleter,
) *QueryController {
	return NewQueryController(
		membership,
		schemas,
		layers,
		agent,
		completer,
		QueryControllerConfig{
			Model:            "fake-model",
			MaxTokens:        1024,
			SummaryModel:     "fake-model",
			SummaryMaxTokens: 512,
			MaxSummaryRows:   10,
		},
	)
}

func TestQueryControllerAskQuestionHappyPath(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)

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
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "측정소별 pH 평균은?")
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	if got.SQLOriginal == "" || got.SQLExecuted == "" {
		t.Fatalf("expected SQL to be populated, got %+v", got)
	}
	if got.SummaryKo != "측정소 1의 pH 평균은 7.2입니다." {
		t.Fatalf("summary = %q", got.SummaryKo)
	}
	if len(got.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(got.Attempts))
	}
	if got.Attempts[0].Stage != "execution" || got.Attempts[0].Error != "" {
		t.Fatalf("attempt = %+v, want successful execution stage", got.Attempts[0])
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %v, want empty when approved layer is used", got.Warnings)
	}
	if len(completer.calls) != 2 {
		t.Fatalf("expected 2 LLM calls (gen+summary), got %d", len(completer.calls))
	}
	genReq := completer.calls[0]
	if genReq.OutputFormat == nil || genReq.OutputFormat.Name != "text_to_sql" {
		t.Fatalf("first call missing structured output schema: %+v", genReq.OutputFormat)
	}
	if genReq.CacheControl == nil || genReq.CacheControl.TTL != "1h" {
		t.Fatalf("cache control = %+v, want TTL 1h", genReq.CacheControl)
	}
	if !strings.Contains(genReq.Messages[0].Content, "측정소별") {
		t.Fatalf("user prompt missing question: %q", genReq.Messages[0].Content)
	}
	summaryReq := completer.calls[1]
	if summaryReq.OutputFormat != nil {
		t.Fatalf("summary call should not set OutputFormat")
	}
}

func TestQueryControllerRetriesOnValidationFailure(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)

	agent := &fakeQueryAgent{
		executeFn: func(_ context.Context, _ uuid.UUID, _ string) (AgentExecuteQueryResult, error) {
			return AgentExecuteQueryResult{
				Columns:   []string{"station_id"},
				Rows:      []map[string]any{{"station_id": int64(1)}},
				ElapsedMS: 5,
			}, nil
		},
	}
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			// Attempt 1: DELETE — sqlguard will reject.
			completionJSON(t, "DELETE FROM mission_app.readings"),
			// Attempt 2: valid SELECT.
			completionJSON(t, "SELECT station_id FROM mission_app.readings"),
			// Summary.
			{Content: "측정소 1이 있습니다."},
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "측정소 목록을 보여줘")
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("attempts = %+v, want 2 entries", got.Attempts)
	}
	if got.Attempts[0].Stage != "validation" {
		t.Fatalf("first attempt stage = %q, want validation", got.Attempts[0].Stage)
	}
	if got.Attempts[1].Stage != "execution" {
		t.Fatalf("second attempt stage = %q, want execution", got.Attempts[1].Stage)
	}
	if got.SummaryKo == "" {
		t.Fatalf("summary should be populated on success")
	}
	// retry prompt should mention prior SQL + error
	retryReq := completer.calls[1]
	if !strings.Contains(retryReq.Messages[0].Content, "DELETE") {
		t.Fatalf("retry prompt should mention prior SQL, got %q", retryReq.Messages[0].Content)
	}
}

func TestQueryControllerRetriesOnExecutionFailure(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)

	agent := &fakeQueryAgent{}
	agent.executeFn = func(_ context.Context, _ uuid.UUID, sql string) (AgentExecuteQueryResult, error) {
		if len(agent.calls) == 1 {
			// First execution fails with a MySQL error.
			return AgentExecuteQueryResult{
				Error:       "Unknown column 'nope' in 'field list'",
				ErrorCode:   queryerror.CodeUnspecified,
				ErrorReason: "Unknown column 'nope'",
			}, nil
		}
		return AgentExecuteQueryResult{
			Columns: []string{"station_id"},
			Rows:    []map[string]any{{"station_id": int64(7)}},
		}, nil
	}

	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "SELECT nope FROM mission_app.readings"),
			completionJSON(t, "SELECT station_id FROM mission_app.readings"),
			{Content: "측정소 7이 있습니다."},
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "측정소 목록")
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(got.Attempts))
	}
	if got.Attempts[0].Stage != "execution" || got.Attempts[0].Error == "" {
		t.Fatalf("first attempt = %+v", got.Attempts[0])
	}
}

func TestQueryControllerBothAttemptsFail(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)

	agent := &fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
		t.Fatal("agent should not be called when SQL never passes validation")
		return AgentExecuteQueryResult{}, nil
	}}
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "DELETE FROM mission_app.readings"),
			completionJSON(t, "TRUNCATE TABLE mission_app.readings"),
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "지워줘")
	if !errors.Is(err, ErrQueryAllAttemptsFailed) {
		t.Fatalf("err = %v, want ErrQueryAllAttemptsFailed", err)
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(got.Attempts))
	}
}

func TestQueryControllerFallsBackToDraftSemanticLayer(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	draft := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusDraft)

	agent := &fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
		return AgentExecuteQueryResult{
			Columns: []string{"ph"},
			Rows:    []map[string]any{{"ph": 7.0}},
		}, nil
	}}
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "SELECT ph FROM mission_app.readings"),
			{Content: "pH는 7.0입니다."},
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return draft, nil
			},
		},
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "pH 값은?")
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	foundDraftWarning := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "초안") {
			foundDraftWarning = true
		}
	}
	if !foundDraftWarning {
		t.Fatalf("expected draft-fallback warning, got %+v", got.Warnings)
	}
}

func TestQueryControllerFallsBackToRawSchema(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()

	agent := &fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
		return AgentExecuteQueryResult{
			Columns: []string{"station_id"},
			Rows:    []map[string]any{{"station_id": int64(1)}},
		}, nil
	}}
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "SELECT station_id FROM mission_app.readings"),
			{Content: "한 개 있습니다."},
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		agent,
		completer,
	)

	got, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "측정소 목록")
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	foundRawWarning := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "원본 스키마") {
			foundRawWarning = true
		}
	}
	if !foundRawWarning {
		t.Fatalf("expected raw-schema warning, got %+v", got.Warnings)
	}
}

func TestQueryControllerRejectsNonMember(t *testing.T) {
	t.Parallel()

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, repository.ErrNotFound
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			t.Fatal("schema store should not be consulted for non-members")
			return model.TenantSchemaVersion{}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		&fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
			t.Fatal("agent should not be consulted for non-members")
			return AgentExecuteQueryResult{}, nil
		}},
		&fakeQueryCompleter{},
	)

	_, err := ctrl.AskQuestion(context.Background(), uuid.New(), "intruder", "뭐든지")
	if !errors.Is(err, ErrQueryAccessDenied) {
		t.Fatalf("err = %v, want ErrQueryAccessDenied", err)
	}
}

func TestQueryControllerReportsAgentOffline(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaRaw, _ := queryTestSchema()
	approved := queryTestSemanticLayer(t, tenantID, schemaVersionID, model.SemanticLayerStatusApproved)

	agent := &fakeQueryAgent{executeFn: func(context.Context, uuid.UUID, string) (AgentExecuteQueryResult, error) {
		return AgentExecuteQueryResult{}, ErrTenantNotConnected
	}}
	completer := &fakeQueryCompleter{
		responses: []llm.CompletionResponse{
			completionJSON(t, "SELECT station_id FROM mission_app.readings"),
		},
	}

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
			return model.TenantSchemaVersion{ID: schemaVersionID, Blob: schemaRaw}, nil
		}},
		fakeQueryLayerStore{
			approvedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return approved, nil
			},
			draftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, repository.ErrNotFound
			},
		},
		agent,
		completer,
	)

	_, err := ctrl.AskQuestion(context.Background(), tenantID, "user_1", "측정소")
	if !errors.Is(err, ErrQueryAgentOffline) {
		t.Fatalf("err = %v, want ErrQueryAgentOffline", err)
	}
}

func TestQueryControllerRequiresQuestion(t *testing.T) {
	t.Parallel()

	ctrl := newTestQueryController(
		fakeQueryMembership{ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
			t.Fatal("membership should not be consulted for empty question")
			return model.TenantUser{}, nil
		}},
		fakeQuerySchemaStore{},
		fakeQueryLayerStore{},
		&fakeQueryAgent{},
		&fakeQueryCompleter{},
	)

	_, err := ctrl.AskQuestion(context.Background(), uuid.New(), "user_1", "   ")
	if !errors.Is(err, ErrQueryEmptyQuestion) {
		t.Fatalf("err = %v, want ErrQueryEmptyQuestion", err)
	}
}
