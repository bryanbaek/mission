package handler

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	queryv1 "github.com/bryanbaek/mission/gen/go/query/v1"
	"github.com/bryanbaek/mission/gen/go/query/v1/queryv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
)

type queryAskController interface {
	AskQuestion(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		question string,
	) (controller.AskQuestionResult, error)
}

// QueryHandler is the Connect-RPC adapter around QueryController.AskQuestion.
type QueryHandler struct {
	queryv1connect.UnimplementedQueryServiceHandler
	ctrl queryAskController
}

func NewQueryHandler(ctrl queryAskController) *QueryHandler {
	return &QueryHandler{ctrl: ctrl}
}

func (h *QueryHandler) AskQuestion(
	ctx context.Context,
	req *connect.Request[queryv1.AskQuestionRequest],
) (*connect.Response[queryv1.AskQuestionResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}

	result, err := h.ctrl.AskQuestion(ctx, tenantID, user.ID, req.Msg.Question)
	if err != nil {
		return nil, queryAskError(err, result)
	}

	return connect.NewResponse(askResultToProto(result)), nil
}

func queryAskError(err error, result controller.AskQuestionResult) error {
	switch {
	case errors.Is(err, controller.ErrQueryAccessDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, controller.ErrQueryEmptyQuestion):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, controller.ErrQueryNoSchema):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, controller.ErrQueryAgentOffline):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, controller.ErrQueryAllAttemptsFailed):
		connectErr := connect.NewError(
			connect.CodeFailedPrecondition,
			err,
		)
		// Surface the attempt trace so the UI can show the user both SQL
		// tries and the underlying failure reason.
		if detail, detailErr := connect.NewErrorDetail(
			askResultToProto(result),
		); detailErr == nil {
			connectErr.AddDetail(detail)
		}
		return connectErr
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func askResultToProto(
	result controller.AskQuestionResult,
) *queryv1.AskQuestionResponse {
	rows := make([]*queryv1.Row, 0, len(result.Rows))
	for _, row := range result.Rows {
		values := make(map[string]string, len(row))
		for key, value := range row {
			if value == nil {
				continue
			}
			values[key] = stringifyCell(value)
		}
		rows = append(rows, &queryv1.Row{Values: values})
	}
	attempts := make([]*queryv1.AttemptDebug, 0, len(result.Attempts))
	for _, attempt := range result.Attempts {
		attempts = append(attempts, &queryv1.AttemptDebug{
			Sql:   attempt.SQL,
			Error: attempt.Error,
			Stage: attempt.Stage,
		})
	}
	return &queryv1.AskQuestionResponse{
		SqlOriginal:   result.SQLOriginal,
		SqlExecuted:   result.SQLExecuted,
		LimitInjected: result.LimitInjected,
		Columns:       append([]string(nil), result.Columns...),
		Rows:          rows,
		RowCount:      result.RowCount,
		ElapsedMs:     result.ElapsedMS,
		SummaryKo:     result.SummaryKo,
		Warnings:      append([]string(nil), result.Warnings...),
		Attempts:      attempts,
	}
}

func stringifyCell(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
