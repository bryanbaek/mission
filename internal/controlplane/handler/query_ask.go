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
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type queryServiceController interface {
	AskQuestion(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		question string,
		locale model.Locale,
	) (controller.AskQuestionResult, error)
	ListMyQueryRuns(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		limit int32,
	) (controller.ListMyQueryRunsResult, error)
	ListReviewQueue(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		filter model.ReviewQueueFilter,
		limit int32,
	) (controller.ListReviewQueueResult, error)
	MarkQueryRunReviewed(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		queryRunID uuid.UUID,
	) (controller.MarkQueryRunReviewedResult, error)
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
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	locale := model.NormalizeLocale(req.Msg.GetLocale())
	result, err := h.ctrl.AskQuestion(ctx, tenantID, user.ID, req.Msg.Question, locale)
	if err != nil {
		return nil, queryAskError(err, result)
	}

	return connect.NewResponse(askResultToProto(result)), nil
}

func (h *QueryHandler) ListMyQueryRuns(
	ctx context.Context,
	req *connect.Request[queryv1.ListMyQueryRunsRequest],
) (*connect.Response[queryv1.ListMyQueryRunsResponse], error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	result, err := h.ctrl.ListMyQueryRuns(ctx, tenantID, user.ID, req.Msg.Limit)
	if err != nil {
		return nil, queryReadError(err)
	}

	runs := make([]*queryv1.QueryRunHistoryItem, 0, len(result.Runs))
	for _, run := range result.Runs {
		runs = append(runs, queryRunHistoryToProto(run))
	}
	return connect.NewResponse(&queryv1.ListMyQueryRunsResponse{
		Runs: runs,
	}), nil
}

func (h *QueryHandler) ListReviewQueue(
	ctx context.Context,
	req *connect.Request[queryv1.ListReviewQueueRequest],
) (*connect.Response[queryv1.ListReviewQueueResponse], error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}

	result, err := h.ctrl.ListReviewQueue(
		ctx,
		tenantID,
		user.ID,
		reviewQueueFilterFromProto(req.Msg.Filter),
		req.Msg.Limit,
	)
	if err != nil {
		return nil, queryReadError(err)
	}

	items := make([]*queryv1.QueryRunReviewItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, queryRunReviewToProto(item))
	}
	return connect.NewResponse(&queryv1.ListReviewQueueResponse{
		Items: items,
	}), nil
}

func (h *QueryHandler) MarkQueryRunReviewed(
	ctx context.Context,
	req *connect.Request[queryv1.MarkQueryRunReviewedRequest],
) (*connect.Response[queryv1.MarkQueryRunReviewedResponse], error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	queryRunID, err := parseConnectUUID(req.Msg.QueryRunId, "query_run_id")
	if err != nil {
		return nil, err
	}

	result, err := h.ctrl.MarkQueryRunReviewed(
		ctx,
		tenantID,
		user.ID,
		queryRunID,
	)
	if err != nil {
		return nil, queryMutationError(err)
	}

	return connect.NewResponse(&queryv1.MarkQueryRunReviewedResponse{
		QueryRunId: result.QueryRunID.String(),
		ReviewedAt: timestamppb.New(result.ReviewedAt),
	}), nil
}

func (h *QueryHandler) SubmitQueryFeedback(
	ctx context.Context,
	req *connect.Request[queryv1.SubmitQueryFeedbackRequest],
) (*connect.Response[queryv1.SubmitQueryFeedbackResponse], error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	queryRunID, err := parseConnectUUID(req.Msg.QueryRunId, "query_run_id")
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
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	queryRunID, err := parseConnectUUID(req.Msg.QueryRunId, "query_run_id")
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
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
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
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := parseConnectUUID(req.Msg.TenantId, "tenant_id")
	if err != nil {
		return nil, err
	}
	exampleID, err := parseConnectUUID(req.Msg.ExampleId, "example_id")
	if err != nil {
		return nil, err
	}

	if err := h.ctrl.ArchiveCanonicalExample(ctx, tenantID, user.ID, exampleID); err != nil {
		return nil, queryMutationError(err)
	}
	return connect.NewResponse(&queryv1.ArchiveCanonicalQueryExampleResponse{}), nil
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
	case llm.IsUnavailableError(err):
		return queryErrorWithDetail(
			connect.CodeUnavailable,
			publicLLMUnavailableError(),
			result,
		)
	default:
		return queryErrorWithDetail(connect.CodeInternal, err, result)
	}
}

func queryReadError(err error) error {
	switch {
	case errors.Is(err, controller.ErrQueryAccessDenied),
		errors.Is(err, controller.ErrQueryReviewOwnerOnly):
		return connect.NewError(connect.CodePermissionDenied, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func queryMutationError(err error) error {
	switch {
	case errors.Is(err, controller.ErrQueryAccessDenied),
		errors.Is(err, controller.ErrQueryFeedbackAccessDenied),
		errors.Is(err, controller.ErrCanonicalQueryExampleOwnerOnly),
		errors.Is(err, controller.ErrQueryReviewOwnerOnly):
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

func attemptsToProto(
	attempts []controller.AskQuestionAttempt,
) []*queryv1.AttemptDebug {
	out := make([]*queryv1.AttemptDebug, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, &queryv1.AttemptDebug{
			Sql:   attempt.SQL,
			Error: attempt.Error,
			Stage: attempt.Stage,
		})
	}
	return out
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
		Attempts:      attemptsToProto(result.Attempts),
		QueryRunId:    queryRunID,
	}
}

func queryRunHistoryToProto(
	run model.TenantQueryRun,
) *queryv1.QueryRunHistoryItem {
	out := &queryv1.QueryRunHistoryItem{
		Id:                  run.ID.String(),
		Question:            run.Question,
		Status:              queryRunStatusToProto(run.Status),
		PromptContextSource: queryPromptContextSourceToProto(run.PromptContextSource),
		SqlOriginal:         run.SQLOriginal,
		SqlExecuted:         run.SQLExecuted,
		RowCount:            run.RowCount,
		ElapsedMs:           run.ElapsedMS,
		ErrorStage:          run.ErrorStage,
		ErrorMessage:        run.ErrorMessage,
		Warnings:            append([]string(nil), run.Warnings...),
		Attempts:            attemptsToProto(toControllerAttempts(run.Attempts)),
		CreatedAt:           timestamppb.New(run.CreatedAt),
	}
	if run.CompletedAt != nil {
		out.CompletedAt = timestamppb.New(*run.CompletedAt)
	}
	return out
}

func queryRunReviewToProto(
	item model.TenantQueryRunReviewItem,
) *queryv1.QueryRunReviewItem {
	out := &queryv1.QueryRunReviewItem{
		Run:                       queryRunHistoryToProto(item.Run),
		HasFeedback:               item.HasFeedback,
		HasActiveCanonicalExample: item.HasActiveCanonicalExample,
	}
	if item.LatestFeedback != nil {
		out.LatestFeedback = feedbackToProto(*item.LatestFeedback)
	}
	if item.ReviewedAt != nil {
		out.ReviewedAt = timestamppb.New(*item.ReviewedAt)
	}
	return out
}

func toControllerAttempts(
	attempts []model.QueryRunAttempt,
) []controller.AskQuestionAttempt {
	out := make([]controller.AskQuestionAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, controller.AskQuestionAttempt{
			SQL:   attempt.SQL,
			Error: attempt.Error,
			Stage: attempt.Stage,
		})
	}
	return out
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

func reviewQueueFilterFromProto(
	filter queryv1.ReviewQueueFilter,
) model.ReviewQueueFilter {
	switch filter {
	case queryv1.ReviewQueueFilter_REVIEW_QUEUE_FILTER_ALL_RECENT:
		return model.ReviewQueueFilterAllRecent
	default:
		return model.ReviewQueueFilterOpen
	}
}

func queryRunStatusToProto(
	status model.QueryRunStatus,
) queryv1.QueryRunStatus {
	switch status {
	case model.QueryRunStatusRunning:
		return queryv1.QueryRunStatus_QUERY_RUN_STATUS_RUNNING
	case model.QueryRunStatusSucceeded:
		return queryv1.QueryRunStatus_QUERY_RUN_STATUS_SUCCEEDED
	case model.QueryRunStatusFailed:
		return queryv1.QueryRunStatus_QUERY_RUN_STATUS_FAILED
	default:
		return queryv1.QueryRunStatus_QUERY_RUN_STATUS_UNSPECIFIED
	}
}

func queryPromptContextSourceToProto(
	source model.QueryPromptContextSource,
) queryv1.QueryPromptContextSource {
	switch source {
	case model.QueryPromptContextSourceApproved:
		return queryv1.QueryPromptContextSource_QUERY_PROMPT_CONTEXT_SOURCE_APPROVED
	case model.QueryPromptContextSourceDraft:
		return queryv1.QueryPromptContextSource_QUERY_PROMPT_CONTEXT_SOURCE_DRAFT
	case model.QueryPromptContextSourceRawSchema:
		return queryv1.QueryPromptContextSource_QUERY_PROMPT_CONTEXT_SOURCE_RAW_SCHEMA
	default:
		return queryv1.QueryPromptContextSource_QUERY_PROMPT_CONTEXT_SOURCE_UNSPECIFIED
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
