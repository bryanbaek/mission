package handler

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	queryv1 "github.com/bryanbaek/mission/gen/go/query/v1"
	"github.com/bryanbaek/mission/gen/go/query/v1/queryv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type queryServiceController interface {
	AskQuestion(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		question string,
	) (controller.AskQuestionResult, error)
	SubmitFeedback(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		queryRunID uuid.UUID,
		rating model.QueryFeedbackRating,
		comment, correctedSQL string,
	) (controller.SubmitQueryFeedbackResult, error)
	CreateCanonicalExample(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		queryRunID uuid.UUID,
		question, sql, notes string,
	) (model.TenantCanonicalQueryExample, error)
	ListCanonicalExamples(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (controller.ListCanonicalQueryExamplesResult, error)
	ArchiveCanonicalExample(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		exampleID uuid.UUID,
	) error
}

// QueryHandler is the Connect-RPC adapter around QueryController.
type QueryHandler struct {
	queryv1connect.UnimplementedQueryServiceHandler
	ctrl queryServiceController
}

func NewQueryHandler(ctrl queryServiceController) *QueryHandler {
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

	tenantID, err := parseUUIDArg(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	result, err := h.ctrl.AskQuestion(ctx, tenantID, user.ID, req.Msg.Question)
	if err != nil {
		return nil, queryAskError(err, result)
	}

	return connect.NewResponse(askResultToProto(result)), nil
}

func (h *QueryHandler) SubmitQueryFeedback(
	ctx context.Context,
	req *connect.Request[queryv1.SubmitQueryFeedbackRequest],
) (*connect.Response[queryv1.SubmitQueryFeedbackResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := parseUUIDArg(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	queryRunID, err := parseUUIDArg(req.Msg.QueryRunId, "query_run_id")
	if err != nil {
		return nil, err
	}

	result, err := h.ctrl.SubmitFeedback(
		ctx,
		tenantID,
		user.ID,
		queryRunID,
		ratingFromProto(req.Msg.Rating),
		req.Msg.Comment,
		req.Msg.CorrectedSql,
	)
	if err != nil {
		return nil, queryMutationError(err)
	}

	return connect.NewResponse(&queryv1.SubmitQueryFeedbackResponse{
		Feedback: feedbackToProto(result.Feedback),
	}), nil
}

func (h *QueryHandler) CreateCanonicalQueryExample(
	ctx context.Context,
	req *connect.Request[queryv1.CreateCanonicalQueryExampleRequest],
) (*connect.Response[queryv1.CreateCanonicalQueryExampleResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := parseUUIDArg(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	queryRunID, err := parseUUIDArg(req.Msg.QueryRunId, "query_run_id")
	if err != nil {
		return nil, err
	}

	example, err := h.ctrl.CreateCanonicalExample(
		ctx,
		tenantID,
		user.ID,
		queryRunID,
		req.Msg.Question,
		req.Msg.Sql,
		req.Msg.Notes,
	)
	if err != nil {
		return nil, queryMutationError(err)
	}

	return connect.NewResponse(&queryv1.CreateCanonicalQueryExampleResponse{
		Example: canonicalExampleToProto(example),
	}), nil
}

func (h *QueryHandler) ListCanonicalQueryExamples(
	ctx context.Context,
	req *connect.Request[queryv1.ListCanonicalQueryExamplesRequest],
) (*connect.Response[queryv1.ListCanonicalQueryExamplesResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := parseUUIDArg(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	result, err := h.ctrl.ListCanonicalExamples(ctx, tenantID, user.ID)
	if err != nil {
		return nil, queryMutationError(err)
	}

	examples := make([]*queryv1.CanonicalQueryExample, 0, len(result.Examples))
	for _, example := range result.Examples {
		examples = append(examples, canonicalExampleToProto(example))
	}
	return connect.NewResponse(&queryv1.ListCanonicalQueryExamplesResponse{
		Examples:        examples,
		ViewerCanManage: result.ViewerCanManage,
	}), nil
}

func (h *QueryHandler) ArchiveCanonicalQueryExample(
	ctx context.Context,
	req *connect.Request[queryv1.ArchiveCanonicalQueryExampleRequest],
) (*connect.Response[queryv1.ArchiveCanonicalQueryExampleResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := parseUUIDArg(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	exampleID, err := parseUUIDArg(req.Msg.ExampleId, "example_id")
	if err != nil {
		return nil, err
	}

	if err := h.ctrl.ArchiveCanonicalExample(ctx, tenantID, user.ID, exampleID); err != nil {
		return nil, queryMutationError(err)
	}
	return connect.NewResponse(&queryv1.ArchiveCanonicalQueryExampleResponse{}), nil
}

func parseUUIDArg(value, field string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid %s", field),
		)
	}
	return parsed, nil
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
		return queryErrorWithDetail(connect.CodeFailedPrecondition, err, result)
	case errors.Is(err, controller.ErrQueryAllAttemptsFailed):
		return queryErrorWithDetail(connect.CodeFailedPrecondition, err, result)
	default:
		return queryErrorWithDetail(connect.CodeInternal, err, result)
	}
}

func queryMutationError(err error) error {
	switch {
	case errors.Is(err, controller.ErrQueryAccessDenied),
		errors.Is(err, controller.ErrQueryFeedbackAccessDenied),
		errors.Is(err, controller.ErrCanonicalQueryExampleOwnerOnly):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, controller.ErrInvalidQueryFeedback),
		errors.Is(err, controller.ErrInvalidCanonicalQueryExample):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, controller.ErrQueryRunNotFound),
		errors.Is(err, controller.ErrCanonicalQueryExampleNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func queryErrorWithDetail(
	code connect.Code,
	err error,
	result controller.AskQuestionResult,
) error {
	connectErr := connect.NewError(code, err)
	if result.QueryRunID == uuid.Nil &&
		len(result.Attempts) == 0 &&
		len(result.Warnings) == 0 {
		return connectErr
	}
	if detail, detailErr := connect.NewErrorDetail(askResultToProto(result)); detailErr == nil {
		connectErr.AddDetail(detail)
	}
	return connectErr
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
	queryRunID := ""
	if result.QueryRunID != uuid.Nil {
		queryRunID = result.QueryRunID.String()
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
		QueryRunId:    queryRunID,
	}
}

func feedbackToProto(
	feedback model.TenantQueryFeedback,
) *queryv1.QueryFeedback {
	return &queryv1.QueryFeedback{
		QueryRunId:   feedback.QueryRunID.String(),
		Rating:       ratingToProto(feedback.Rating),
		Comment:      feedback.Comment,
		CorrectedSql: feedback.CorrectedSQL,
		CreatedAt:    timestamppb.New(feedback.CreatedAt),
		UpdatedAt:    timestamppb.New(feedback.UpdatedAt),
	}
}

func canonicalExampleToProto(
	example model.TenantCanonicalQueryExample,
) *queryv1.CanonicalQueryExample {
	return &queryv1.CanonicalQueryExample{
		Id:               example.ID.String(),
		SourceQueryRunId: example.SourceQueryRunID.String(),
		SchemaVersionId:  example.SchemaVersionID.String(),
		Question:         example.Question,
		Sql:              example.SQL,
		Notes:            example.Notes,
		CreatedAt:        timestamppb.New(example.CreatedAt),
	}
}

func ratingFromProto(
	rating queryv1.QueryFeedbackRating,
) model.QueryFeedbackRating {
	switch rating {
	case queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_UP:
		return model.QueryFeedbackRatingUp
	case queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_DOWN:
		return model.QueryFeedbackRatingDown
	default:
		return ""
	}
}

func ratingToProto(
	rating model.QueryFeedbackRating,
) queryv1.QueryFeedbackRating {
	switch rating {
	case model.QueryFeedbackRatingUp:
		return queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_UP
	case model.QueryFeedbackRatingDown:
		return queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_DOWN
	default:
		return queryv1.QueryFeedbackRating_QUERY_FEEDBACK_RATING_UNSPECIFIED
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
