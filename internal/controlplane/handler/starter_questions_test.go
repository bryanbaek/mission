package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	starterv1 "github.com/bryanbaek/mission/gen/go/starter/v1"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeStarterQuestionsController struct {
	listFn       func(context.Context, uuid.UUID, string) (controller.StarterQuestionsListResult, error)
	regenerateFn func(context.Context, uuid.UUID, string) (controller.StarterQuestionsListResult, error)
}

func (f fakeStarterQuestionsController) List(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (controller.StarterQuestionsListResult, error) {
	return f.listFn(ctx, tenantID, clerkUserID)
}

func (f fakeStarterQuestionsController) Regenerate(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (controller.StarterQuestionsListResult, error) {
	return f.regenerateFn(ctx, tenantID, clerkUserID)
}

func starterQuestionsHandlerContext() context.Context {
	return auth.WithUser(context.Background(), auth.User{ID: "user_123"})
}

func TestStarterQuestionsHandlerValidationAndErrorMapping(t *testing.T) {
	t.Parallel()

	handler := NewStarterQuestionsHandler(fakeStarterQuestionsController{
		listFn: func(context.Context, uuid.UUID, string) (controller.StarterQuestionsListResult, error) {
			return controller.StarterQuestionsListResult{}, controller.ErrStarterQuestionsAccessDenied
		},
		regenerateFn: func(context.Context, uuid.UUID, string) (controller.StarterQuestionsListResult, error) {
			return controller.StarterQuestionsListResult{}, controller.ErrStarterQuestionsNoLayer
		},
	})

	_, err := handler.List(
		context.Background(),
		connect.NewRequest(&starterv1.ListStarterQuestionsRequest{
			TenantId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodeUnauthenticated)

	_, err = handler.List(
		starterQuestionsHandlerContext(),
		connect.NewRequest(&starterv1.ListStarterQuestionsRequest{
			TenantId: "bad",
		}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = handler.List(
		starterQuestionsHandlerContext(),
		connect.NewRequest(&starterv1.ListStarterQuestionsRequest{
			TenantId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodePermissionDenied)

	_, err = handler.Regenerate(
		starterQuestionsHandlerContext(),
		connect.NewRequest(&starterv1.RegenerateStarterQuestionsRequest{
			TenantId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodeFailedPrecondition)
}

func TestStarterQuestionsHandlerMapsResultToProto(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	setID := uuid.New()
	generatedAt := time.Unix(1_700_200_200, 0).UTC()
	handler := NewStarterQuestionsHandler(fakeStarterQuestionsController{
		listFn: func(_ context.Context, gotTenantID uuid.UUID, clerkUserID string) (controller.StarterQuestionsListResult, error) {
			if gotTenantID != tenantID {
				t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
			}
			if clerkUserID != "user_123" {
				t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
			}
			return controller.StarterQuestionsListResult{
				Questions: []model.StarterQuestion{
					{
						ID:           uuid.New(),
						SetID:        setID,
						TenantID:     tenantID,
						Ordinal:      1,
						Text:         "이번 달 신규 고객 수는 몇 명인가요?",
						Category:     model.StarterQuestionCategoryCount,
						PrimaryTable: "customers",
						IsActive:     true,
					},
				},
				SetID:       setID,
				GeneratedAt: generatedAt,
			}, nil
		},
		regenerateFn: func(context.Context, uuid.UUID, string) (controller.StarterQuestionsListResult, error) {
			return controller.StarterQuestionsListResult{}, errors.New("unexpected Regenerate call")
		},
	})

	resp, err := handler.List(
		starterQuestionsHandlerContext(),
		connect.NewRequest(&starterv1.ListStarterQuestionsRequest{
			TenantId: tenantID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if resp.Msg.SetId != setID.String() {
		t.Fatalf("set_id = %q, want %s", resp.Msg.SetId, setID)
	}
	if len(resp.Msg.Questions) != 1 {
		t.Fatalf("len(questions) = %d, want 1", len(resp.Msg.Questions))
	}
	if resp.Msg.Questions[0].PrimaryTable != "customers" {
		t.Fatalf("primary_table = %q", resp.Msg.Questions[0].PrimaryTable)
	}
	if resp.Msg.GeneratedAt == nil || resp.Msg.GeneratedAt.AsTime() != generatedAt {
		t.Fatalf("generated_at = %v, want %v", resp.Msg.GeneratedAt, generatedAt)
	}
}
