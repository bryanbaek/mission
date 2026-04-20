package handler

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	queryv1 "github.com/bryanbaek/mission/gen/go/query/v1"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
)

type fakeQueryAskController struct {
	askFn func(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		question string,
	) (controller.AskQuestionResult, error)
}

func (f fakeQueryAskController) AskQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	question string,
) (controller.AskQuestionResult, error) {
	return f.askFn(ctx, tenantID, clerkUserID, question)
}

func queryHandlerContext() context.Context {
	return auth.WithUser(context.Background(), auth.User{ID: "user_123"})
}

func TestQueryHandlerRejectsUnauthenticated(t *testing.T) {
	t.Parallel()

	handler := NewQueryHandler(fakeQueryAskController{
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

func TestQueryHandlerValidatesTenantID(t *testing.T) {
	t.Parallel()

	handler := NewQueryHandler(fakeQueryAskController{
		askFn: func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error) {
			t.Fatal("controller should not be invoked for invalid tenant id")
			return controller.AskQuestionResult{}, nil
		},
	})

	_, err := handler.AskQuestion(
		queryHandlerContext(),
		connect.NewRequest(&queryv1.AskQuestionRequest{
			TenantId: "not-a-uuid",
			Question: "측정소",
		}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)
}

func TestQueryHandlerMapsControllerErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		ctrlErr  error
		wantCode connect.Code
	}{
		{"access denied", controller.ErrQueryAccessDenied, connect.CodePermissionDenied},
		{"empty question", controller.ErrQueryEmptyQuestion, connect.CodeInvalidArgument},
		{"no schema", controller.ErrQueryNoSchema, connect.CodeFailedPrecondition},
		{"agent offline", controller.ErrQueryAgentOffline, connect.CodeFailedPrecondition},
		{"all attempts failed", controller.ErrQueryAllAttemptsFailed, connect.CodeFailedPrecondition},
		{"unexpected", errors.New("boom"), connect.CodeInternal},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewQueryHandler(fakeQueryAskController{
				askFn: func(context.Context, uuid.UUID, string, string) (controller.AskQuestionResult, error) {
					return controller.AskQuestionResult{}, tc.ctrlErr
				},
			})

			_, err := handler.AskQuestion(
				queryHandlerContext(),
				connect.NewRequest(&queryv1.AskQuestionRequest{
					TenantId: uuid.NewString(),
					Question: "질문",
				}),
			)
			requireConnectCode(t, err, tc.wantCode)
		})
	}
}

func TestQueryHandlerMapsResultToProto(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := NewQueryHandler(fakeQueryAskController{
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
	if len(resp.Msg.Attempts) != 1 || resp.Msg.Attempts[0].Stage != "execution" {
		t.Fatalf("attempts = %+v", resp.Msg.Attempts)
	}
	if len(resp.Msg.Warnings) != 1 || resp.Msg.Warnings[0] != "초안 레이어 사용" {
		t.Fatalf("warnings = %+v", resp.Msg.Warnings)
	}
}
