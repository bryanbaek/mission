package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	queryv1 "github.com/bryanbaek/mission/gen/go/query/v1"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeQueryController struct {
	askFn              func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error)
	listMyRunsFn       func(context.Context, uuid.UUID, string, int32) (controller.ListMyQueryRunsResult, error)
	submitFeedbackFn   func(context.Context, uuid.UUID, string, uuid.UUID, model.QueryFeedbackRating, string, string) (controller.SubmitQueryFeedbackResult, error)
	createCanonicalFn  func(context.Context, uuid.UUID, string, uuid.UUID, string, string, string) (model.TenantCanonicalQueryExample, error)
	listCanonicalFn    func(context.Context, uuid.UUID, string) (controller.ListCanonicalQueryExamplesResult, error)
	archiveCanonicalFn func(context.Context, uuid.UUID, string, uuid.UUID) error
}

func (f fakeQueryController) AskQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	question string,
) (controller.AskQuestionResult, error) {
	return f.askFn(ctx, tenantID, clerkUserID, question)
}

func (f fakeQueryController) ListMyQueryRuns(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	limit int32,
) (controller.ListMyQueryRunsResult, error) {
	return f.listMyRunsFn(ctx, tenantID, clerkUserID, limit)
}

func (f fakeQueryController) SubmitFeedback(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
	rating model.QueryFeedbackRating,
	comment, correctedSQL string,
) (controller.SubmitQueryFeedbackResult, error) {
	return f.submitFeedbackFn(ctx, tenantID, clerkUserID, queryRunID, rating, comment, correctedSQL)
}

func (f fakeQueryController) CreateCanonicalExample(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
	question, sql, notes string,
) (model.TenantCanonicalQueryExample, error) {
	return f.createCanonicalFn(ctx, tenantID, clerkUserID, queryRunID, question, sql, notes)
}

func (f fakeQueryController) ListCanonicalExamples(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (controller.ListCanonicalQueryExamplesResult, error) {
	return f.listCanonicalFn(ctx, tenantID, clerkUserID)
}

func (f fakeQueryController) ArchiveCanonicalExample(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	exampleID uuid.UUID,
) error {
	return f.archiveCanonicalFn(ctx, tenantID, clerkUserID, exampleID)
}

func queryHandlerContext() context.Context {
	return auth.WithUser(context.Background(), auth.User{ID: "user_123"})
}

func TestQueryHandlerRejectsUnauthenticated(t *testing.T) {
	t.Parallel()

	handler := NewQueryHandler(fakeQueryController{
		askFn: func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error) {
			t.Fatal("controller should not be invoked without auth")
			return controller.AskQuestionResult{}, nil
		},
	})

	_, err := handler.AskQuestion(
		context.Background(),
		connect.NewRequest(&queryv1.AskQuestionRequest{
			TenantId: uuid.NewString(),
			Question: "측정소",
		}),
	)
	requireConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestQueryHandlerMapsResultToProto(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	queryRunID := uuid.New()
	handler := NewQueryHandler(fakeQueryController{
		askFn: func(_ context.Context, gotTenantID uuid.UUID, clerkUserID, question string) (controller.AskQuestionResult, error) {
			if gotTenantID != tenantID {
				t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
			}
			if clerkUserID != "user_123" {
				t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
			}
			if question != "지난달 평균 pH는?" {
				t.Fatalf("question = %q", question)
			}
			return controller.AskQuestionResult{
				QueryRunID:    queryRunID,
				SQLOriginal:   "SELECT AVG(ph) FROM readings",
				SQLExecuted:   "SELECT AVG(ph) FROM readings LIMIT 1000",
				LimitInjected: true,
				Columns:       []string{"avg_ph"},
				Rows: []map[string]any{
					{"avg_ph": 7.12},
					{"avg_ph": nil},
				},
				RowCount:  2,
				ElapsedMS: 88,
				SummaryKo: "평균 pH는 7.12입니다.",
				Warnings:  []string{"초안 레이어 사용"},
				Attempts: []controller.AskQuestionAttempt{
					{SQL: "SELECT AVG(ph) FROM readings", Stage: "execution"},
				},
			}, nil
		},
	})

	resp, err := handler.AskQuestion(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.AskQuestionRequest{
			TenantId: tenantID.String(),
			Question: "지난달 평균 pH는?",
		}),
	)
	if err != nil {
		t.Fatalf("AskQuestion returned error: %v", err)
	}
	if resp.Msg.QueryRunId != queryRunID.String() {
		t.Fatalf("query_run_id = %q, want %s", resp.Msg.QueryRunId, queryRunID)
	}
	if resp.Msg.SqlOriginal == "" || resp.Msg.SqlExecuted == "" {
		t.Fatalf("sql fields empty: %+v", resp.Msg)
	}
	if !resp.Msg.LimitInjected {
		t.Fatal("limit_injected = false, want true")
	}
	if resp.Msg.SummaryKo != "평균 pH는 7.12입니다." {
		t.Fatalf("summary = %q", resp.Msg.SummaryKo)
	}
	if len(resp.Msg.Rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(resp.Msg.Rows))
	}
	if resp.Msg.Rows[0].GetValues()["avg_ph"] != "7.12" {
		t.Fatalf("row[0][avg_ph] = %q", resp.Msg.Rows[0].GetValues()["avg_ph"])
	}
	if _, hasNil := resp.Msg.Rows[1].GetValues()["avg_ph"]; hasNil {
		t.Fatalf("nil cell should be absent, got %v", resp.Msg.Rows[1].GetValues())
	}
}

func TestQueryHandlerAttachesTerminalFailureDetails(t *testing.T) {
	t.Parallel()

	queryRunID := uuid.New()
	handler := NewQueryHandler(fakeQueryController{
		askFn: func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error) {
			return controller.AskQuestionResult{
				QueryRunID: queryRunID,
				Warnings:   []string{"원본 스키마만 사용"},
				Attempts: []controller.AskQuestionAttempt{
					{SQL: "SELECT missing FROM readings", Stage: "execution", Error: "Unknown column"},
				},
			}, controller.ErrQueryAllAttemptsFailed
		},
	})

	_, err := handler.AskQuestion(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.AskQuestionRequest{
			TenantId: uuid.NewString(),
			Question: "질문",
		}),
	)
	requireConnectCode(t, err, connect.CodeFailedPrecondition)

	connectErr := connect.CodeOf(err)
	if connectErr != connect.CodeFailedPrecondition {
		t.Fatalf("connect code = %v", connectErr)
	}

	ce := new(connect.Error)
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want *connect.Error", err)
	}
	details := ce.Details()
	if len(details) != 1 {
		t.Fatalf("detail len = %d, want 1", len(details))
	}

	msg, detailErr := details[0].Value()
	if detailErr != nil {
		t.Fatalf("decode detail: %v", detailErr)
	}
	detail, ok := msg.(*queryv1.AskQuestionResponse)
	if !ok {
		t.Fatalf("detail type = %T, want *AskQuestionResponse", msg)
	}
	if detail.QueryRunId != queryRunID.String() {
		t.Fatalf("query_run_id detail = %q, want %s", detail.QueryRunId, queryRunID)
	}
}

func TestQueryHandlerListMyQueryRunsMapsResultToProto(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	runID := uuid.New()
	createdAt := time.Unix(1_700_003_000, 0).UTC()
	completedAt := createdAt.Add(15 * time.Second)
	handler := NewQueryHandler(fakeQueryController{
		askFn: func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error) {
			return controller.AskQuestionResult{}, errors.New("unexpected AskQuestion call")
		},
		listMyRunsFn: func(_ context.Context, gotTenantID uuid.UUID, clerkUserID string, limit int32) (controller.ListMyQueryRunsResult, error) {
			if gotTenantID != tenantID {
				t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
			}
			if clerkUserID != "user_123" {
				t.Fatalf("clerkUserID = %q", clerkUserID)
			}
			if limit != 7 {
				t.Fatalf("limit = %d, want 7", limit)
			}
			return controller.ListMyQueryRunsResult{
				Runs: []model.TenantQueryRun{{
					ID:                  runID,
					TenantID:            tenantID,
					Question:            "최근 평균 pH는?",
					Status:              model.QueryRunStatusFailed,
					PromptContextSource: model.QueryPromptContextSourceRawSchema,
					SQLOriginal:         "SELECT missing FROM readings",
					SQLExecuted:         "",
					Attempts: []model.QueryRunAttempt{{
						SQL:   "SELECT missing FROM readings",
						Error: "Unknown column",
						Stage: "execution",
					}},
					Warnings:     []string{"warning"},
					RowCount:     0,
					ElapsedMS:    18,
					ErrorStage:   "execution",
					ErrorMessage: "Unknown column",
					CreatedAt:    createdAt,
					CompletedAt:  &completedAt,
				}},
			}, nil
		},
		submitFeedbackFn: func(context.Context, uuid.UUID, string, uuid.UUID, model.QueryFeedbackRating, string, string) (controller.SubmitQueryFeedbackResult, error) {
			return controller.SubmitQueryFeedbackResult{}, errors.New("unexpected SubmitFeedback call")
		},
		createCanonicalFn: func(context.Context, uuid.UUID, string, uuid.UUID, string, string, string) (model.TenantCanonicalQueryExample, error) {
			return model.TenantCanonicalQueryExample{}, errors.New("unexpected CreateCanonicalExample call")
		},
		listCanonicalFn: func(context.Context, uuid.UUID, string) (controller.ListCanonicalQueryExamplesResult, error) {
			return controller.ListCanonicalQueryExamplesResult{}, errors.New("unexpected ListCanonicalExamples call")
		},
		archiveCanonicalFn: func(context.Context, uuid.UUID, string, uuid.UUID) error {
			return errors.New("unexpected ArchiveCanonicalExample call")
		},
	})

	resp, err := handler.ListMyQueryRuns(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.ListMyQueryRunsRequest{
			TenantId: tenantID.String(),
			Limit:    7,
		}),
	)
	if err != nil {
		t.Fatalf("ListMyQueryRuns returned error: %v", err)
	}
	if len(resp.Msg.Runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(resp.Msg.Runs))
	}
	run := resp.Msg.Runs[0]
	if run.Id != runID.String() {
		t.Fatalf("id = %q, want %s", run.Id, runID)
	}
	if run.Status != queryv1.QueryRunStatus_QUERY_RUN_STATUS_FAILED {
		t.Fatalf("status = %v", run.Status)
	}
	if run.PromptContextSource != queryv1.QueryPromptContextSource_QUERY_PROMPT_CONTEXT_SOURCE_RAW_SCHEMA {
		t.Fatalf("prompt_context_source = %v", run.PromptContextSource)
	}
	if len(run.Attempts) != 1 || run.Attempts[0].Error != "Unknown column" {
		t.Fatalf("attempts = %+v", run.Attempts)
	}
	if run.CreatedAt == nil || run.CompletedAt == nil {
		t.Fatalf("timestamps missing: %+v", run)
	}
}

func TestQueryHandlerListMyQueryRunsMapsAccessDenied(t *testing.T) {
	t.Parallel()

	handler := NewQueryHandler(fakeQueryController{
		askFn: func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error) {
			return controller.AskQuestionResult{}, errors.New("unexpected AskQuestion call")
		},
		listMyRunsFn: func(context.Context, uuid.UUID, string, int32) (controller.ListMyQueryRunsResult, error) {
			return controller.ListMyQueryRunsResult{}, controller.ErrQueryAccessDenied
		},
		submitFeedbackFn: func(context.Context, uuid.UUID, string, uuid.UUID, model.QueryFeedbackRating, string, string) (controller.SubmitQueryFeedbackResult, error) {
			return controller.SubmitQueryFeedbackResult{}, errors.New("unexpected SubmitFeedback call")
		},
		createCanonicalFn: func(context.Context, uuid.UUID, string, uuid.UUID, string, string, string) (model.TenantCanonicalQueryExample, error) {
			return model.TenantCanonicalQueryExample{}, errors.New("unexpected CreateCanonicalExample call")
		},
		listCanonicalFn: func(context.Context, uuid.UUID, string) (controller.ListCanonicalQueryExamplesResult, error) {
			return controller.ListCanonicalQueryExamplesResult{}, errors.New("unexpected ListCanonicalExamples call")
		},
		archiveCanonicalFn: func(context.Context, uuid.UUID, string, uuid.UUID) error {
			return errors.New("unexpected ArchiveCanonicalExample call")
		},
	})

	_, err := handler.ListMyQueryRuns(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.ListMyQueryRunsRequest{
			TenantId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodePermissionDenied)
}

func TestQueryHandlerSubmitFeedback(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	queryRunID := uuid.New()
	now := time.Unix(1_700_001_000, 0).UTC()
	handler := NewQueryHandler(fakeQueryController{
		submitFeedbackFn: func(
			_ context.Context,
			gotTenantID uuid.UUID,
			clerkUserID string,
			gotQueryRunID uuid.UUID,
			rating model.QueryFeedbackRating,
			comment, correctedSQL string,
		) (controller.SubmitQueryFeedbackResult, error) {
			if gotTenantID != tenantID || gotQueryRunID != queryRunID {
				t.Fatalf("unexpected ids: tenant=%s run=%s", gotTenantID, gotQueryRunID)
			}
			if clerkUserID != "user_123" {
				t.Fatalf("clerkUserID = %q", clerkUserID)
			}
			if rating != model.QueryFeedbackRatingDown {
				t.Fatalf("rating = %q", rating)
			}
			if comment != "조인이 잘못됐습니다." || correctedSQL != "SELECT AVG(ph) FROM readings" {
				t.Fatalf("unexpected feedback payload: %q / %q", comment, correctedSQL)
			}
			return controller.SubmitQueryFeedbackResult{
				Feedback: model.TenantQueryFeedback{
					QueryRunID:   queryRunID,
					Rating:       model.QueryFeedbackRatingDown,
					Comment:      comment,
					CorrectedSQL: correctedSQL,
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			}, nil
		},
	})

	resp, err := handler.SubmitQueryFeedback(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.SubmitQueryFeedbackRequest{
			TenantId:     tenantID.String(),
			QueryRunId:   queryRunID.String(),
			Rating:       queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_DOWN,
			Comment:      "조인이 잘못됐습니다.",
			CorrectedSql: "SELECT AVG(ph) FROM readings",
		}),
	)
	if err != nil {
		t.Fatalf("SubmitQueryFeedback returned error: %v", err)
	}
	if resp.Msg.Feedback.GetQueryRunId() != queryRunID.String() {
		t.Fatalf("query_run_id = %q", resp.Msg.Feedback.GetQueryRunId())
	}
	if resp.Msg.Feedback.Rating != queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_DOWN {
		t.Fatalf("rating = %v", resp.Msg.Feedback.Rating)
	}
}

func TestQueryHandlerOwnerOnlyCanonicalExampleActions(t *testing.T) {
	t.Parallel()

	handler := NewQueryHandler(fakeQueryController{
		createCanonicalFn: func(context.Context, uuid.UUID, string, uuid.UUID, string, string, string) (model.TenantCanonicalQueryExample, error) {
			return model.TenantCanonicalQueryExample{}, controller.ErrCanonicalQueryExampleOwnerOnly
		},
	})

	_, err := handler.CreateCanonicalQueryExample(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.CreateCanonicalQueryExampleRequest{
			TenantId:   uuid.NewString(),
			QueryRunId: uuid.NewString(),
			Question:   "평균 pH는?",
			Sql:        "SELECT AVG(ph) FROM readings",
		}),
	)
	requireConnectCode(t, err, connect.CodePermissionDenied)
}

func TestQueryHandlerListsCanonicalExamples(t *testing.T) {
	t.Parallel()

	exampleID := uuid.New()
	queryRunID := uuid.New()
	schemaVersionID := uuid.New()
	createdAt := time.Unix(1_700_001_100, 0).UTC()
	handler := NewQueryHandler(fakeQueryController{
		listCanonicalFn: func(context.Context, uuid.UUID, string) (controller.ListCanonicalQueryExamplesResult, error) {
			return controller.ListCanonicalQueryExamplesResult{
				ViewerCanManage: true,
				Examples: []model.TenantCanonicalQueryExample{{
					ID:               exampleID,
					SourceQueryRunID: queryRunID,
					SchemaVersionID:  schemaVersionID,
					Question:         "평균 pH는?",
					SQL:              "SELECT AVG(ph) FROM readings",
					Notes:            "기본 평균 질의",
					CreatedAt:        createdAt,
				}},
			}, nil
		},
	})

	resp, err := handler.ListCanonicalQueryExamples(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.ListCanonicalQueryExamplesRequest{
			TenantId: uuid.NewString(),
		}),
	)
	if err != nil {
		t.Fatalf("ListCanonicalQueryExamples returned error: %v", err)
	}
	if !resp.Msg.ViewerCanManage {
		t.Fatal("viewer_can_manage = false, want true")
	}
	if len(resp.Msg.Examples) != 1 || resp.Msg.Examples[0].GetId() != exampleID.String() {
		t.Fatalf("examples = %+v", resp.Msg.Examples)
	}
}
