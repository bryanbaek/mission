package handler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	starterv1 "github.com/bryanbaek/mission/gen/go/starter/v1"
	"github.com/bryanbaek/mission/gen/go/starter/v1/starterv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type starterQuestionsController interface {
	List(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		locale model.Locale,
	) (controller.StarterQuestionsListResult, error)
	Regenerate(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		locale model.Locale,
	) (controller.StarterQuestionsListResult, error)
}

type StarterQuestionsHandler struct {
	starterv1connect.UnimplementedStarterQuestionsServiceHandler
	ctrl starterQuestionsController
}

func NewStarterQuestionsHandler(
	ctrl starterQuestionsController,
) *StarterQuestionsHandler {
	return &StarterQuestionsHandler{ctrl: ctrl}
}

func (h *StarterQuestionsHandler) List(
	ctx context.Context,
	req *connect.Request[starterv1.ListRequest],
) (*connect.Response[starterv1.ListResponse], error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	locale := model.NormalizeLocale(req.Msg.Locale)
	result, err := h.ctrl.List(ctx, tenantID, user.ID, locale)
	if err != nil {
		return nil, starterQuestionsError(err)
	}

	return connect.NewResponse(&starterv1.ListResponse{
		Questions:   starterQuestionsToProto(result.Questions),
		GeneratedAt: timestamppb.New(result.GeneratedAt),
		SetId:       result.SetID.String(),
	}), nil
}

func (h *StarterQuestionsHandler) Regenerate(
	ctx context.Context,
	req *connect.Request[starterv1.RegenerateRequest],
) (*connect.Response[starterv1.RegenerateResponse], error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	locale := model.NormalizeLocale(req.Msg.Locale)
	result, err := h.ctrl.Regenerate(ctx, tenantID, user.ID, locale)
	if err != nil {
		return nil, starterQuestionsError(err)
	}

	return connect.NewResponse(&starterv1.RegenerateResponse{
		Questions:   starterQuestionsToProto(result.Questions),
		GeneratedAt: timestamppb.New(result.GeneratedAt),
		SetId:       result.SetID.String(),
	}), nil
}

func starterQuestionsError(err error) error {
	switch {
	case errors.Is(err, controller.ErrStarterQuestionsAccessDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, controller.ErrStarterQuestionsNoLayer):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case llm.IsUnavailableError(err):
		return connect.NewError(
			connect.CodeUnavailable,
			publicLLMUnavailableError(),
		)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func starterQuestionsToProto(
	questions []model.StarterQuestion,
) []*starterv1.StarterQuestion {
	out := make([]*starterv1.StarterQuestion, 0, len(questions))
	for _, question := range questions {
		out = append(out, &starterv1.StarterQuestion{
			Id:           question.ID.String(),
			Text:         question.Text,
			Category:     string(question.Category),
			PrimaryTable: question.PrimaryTable,
			Ordinal:      question.Ordinal,
		})
	}
	return out
}
