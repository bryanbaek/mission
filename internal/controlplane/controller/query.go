package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

var (
	// ErrQueryAccessDenied is returned when the authenticated user is not a
	// member of the tenant.
	ErrQueryAccessDenied = errors.New("not a member of this tenant")
	// ErrQueryNoSchema is returned when the tenant has no introspected schema
	// yet — the question cannot be answered without one.
	ErrQueryNoSchema = errors.New(
		"tenant has no captured schema yet; run introspection first",
	)
	// ErrQueryEmptyQuestion is returned when the question is blank.
	ErrQueryEmptyQuestion = errors.New("question is required")
	// ErrQueryAllAttemptsFailed is returned when both the first attempt and
	// the retry attempt produced SQL that failed validation or execution.
	ErrQueryAllAttemptsFailed = errors.New(
		"all SQL generation attempts failed",
	)
	// ErrQueryAgentOffline is returned when no edge agent is connected for
	// the tenant.
	ErrQueryAgentOffline = errors.New(
		"edge agent is not connected for this tenant",
	)
	ErrQueryRunNotFound               = errors.New("query run not found")
	ErrQueryFeedbackAccessDenied      = errors.New("only the original query creator can submit feedback")
	ErrInvalidQueryFeedback           = errors.New("feedback rating is required")
	ErrCanonicalQueryExampleNotFound  = errors.New("canonical query example not found")
	ErrCanonicalQueryExampleOwnerOnly = errors.New("owner role required")
	ErrInvalidCanonicalQueryExample   = errors.New("question and sql are required")
	ErrQueryReviewOwnerOnly           = errors.New("owner role required")
)

const (
	defaultRetrievedExamplesLimit = 3
	defaultCanonicalExampleLimit  = 20
	defaultListMyQueryRunsLimit   = 20
	defaultReviewQueueLimit       = 50
)

type queryMembershipCheckerCtl interface {
	EnsureMembership(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (model.TenantUser, error)
}

type querySchemaStoreCtl interface {
	LatestByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
	) (model.TenantSchemaVersion, error)
}

type querySemanticLayerStoreCtl interface {
	LatestApprovedBySchemaVersion(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
	) (model.TenantSemanticLayer, error)
	LatestDraftBySchemaVersion(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
	) (model.TenantSemanticLayer, error)
}

type queryRunStoreCtl interface {
	Create(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
		semanticLayerID *uuid.UUID,
		source model.QueryPromptContextSource,
		clerkUserID string,
		question string,
	) (model.TenantQueryRun, error)
	GetByTenantAndID(
		ctx context.Context,
		tenantID, id uuid.UUID,
	) (model.TenantQueryRun, error)
	ListByTenantAndUser(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		limit int,
	) ([]model.TenantQueryRun, error)
	ListReviewQueue(
		ctx context.Context,
		tenantID uuid.UUID,
		filter model.ReviewQueueFilter,
		limit int,
	) ([]model.TenantQueryRunReviewItem, error)
	MarkReviewed(
		ctx context.Context,
		tenantID, id uuid.UUID,
		reviewedAt time.Time,
		reviewedByUserID string,
	) error
	CompleteSucceeded(
		ctx context.Context,
		id uuid.UUID,
		sqlOriginal, sqlExecuted string,
		attempts []model.QueryRunAttempt,
		warnings []string,
		rowCount, elapsedMS int64,
		retrievedExampleIDs []uuid.UUID,
		completedAt time.Time,
	) (model.TenantQueryRun, error)
	CompleteFailed(
		ctx context.Context,
		id uuid.UUID,
		attempts []model.QueryRunAttempt,
		warnings []string,
		retrievedExampleIDs []uuid.UUID,
		errorStage, errorMessage string,
		completedAt time.Time,
	) (model.TenantQueryRun, error)
}

type queryFeedbackStoreCtl interface {
	Upsert(
		ctx context.Context,
		queryRunID uuid.UUID,
		clerkUserID string,
		rating model.QueryFeedbackRating,
		comment, correctedSQL string,
		now time.Time,
	) (model.TenantQueryFeedback, error)
}

type queryCanonicalExampleStoreCtl interface {
	Create(
		ctx context.Context,
		tenantID, schemaVersionID, sourceQueryRunID uuid.UUID,
		createdByUserID, question, sql, notes string,
	) (model.TenantCanonicalQueryExample, error)
	ListActiveByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
		limit int,
	) ([]model.TenantCanonicalQueryExample, error)
	SearchActiveByQuestion(
		ctx context.Context,
		tenantID uuid.UUID,
		question string,
		limit int,
		schemaVersionID *uuid.UUID,
	) ([]model.TenantCanonicalQueryExample, error)
	Archive(
		ctx context.Context,
		tenantID, id uuid.UUID,
		archivedAt time.Time,
	) error
}

type queryAgentExecutor interface {
	ExecuteQuery(
		ctx context.Context,
		tenantID uuid.UUID,
		sql string,
	) (AgentExecuteQueryResult, error)
}

type queryCompleter interface {
	Complete(
		ctx context.Context,
		req llm.CompletionRequest,
	) (llm.CompletionResponse, error)
}

// QueryControllerConfig configures a QueryController. Zero values fall back
// to sensible defaults.
type QueryControllerConfig struct {
	Now                   func() time.Time
	Model                 string
	ProviderModels        map[string]string
	MaxTokens             int
	SummaryModel          string
	SummaryProviderModels map[string]string
	SummaryMaxTokens      int
	MaxSummaryRows        int
	MaxRetrievedExamples  int
	MaxCanonicalExamples  int
}

// AskQuestionAttempt records one pass through the SQL-generation pipeline.
// Stage is one of "generation", "validation", "execution".
type AskQuestionAttempt struct {
	SQL   string
	Error string
	Stage string
}

// AskQuestionResult is the controller's output for AskQuestion.
type AskQuestionResult struct {
	QueryRunID    uuid.UUID
	SQLOriginal   string
	SQLExecuted   string
	LimitInjected bool
	Columns       []string
	Rows          []map[string]any
	RowCount      int64
	ElapsedMS     int64
	SummaryKo     string
	Warnings      []string
	Attempts      []AskQuestionAttempt
}

type SubmitQueryFeedbackResult struct {
	Feedback model.TenantQueryFeedback
}

type ListCanonicalQueryExamplesResult struct {
	ViewerCanManage bool
	Examples        []model.TenantCanonicalQueryExample
}

type ListMyQueryRunsResult struct {
	Runs []model.TenantQueryRun
}

type ListReviewQueueResult struct {
	Items []model.TenantQueryRunReviewItem
}

type MarkQueryRunReviewedResult struct {
	QueryRunID uuid.UUID
	ReviewedAt time.Time
}

// QueryController orchestrates the NL-to-SQL pipeline: semantic-layer lookup,
// canonical-example retrieval, LLM SQL generation, sqlguard validation,
// edge-agent execution, and Korean summarization.
type QueryController struct {
	tenants               queryMembershipCheckerCtl
	schemas               querySchemaStoreCtl
	layers                querySemanticLayerStoreCtl
	runs                  queryRunStoreCtl
	feedback              queryFeedbackStoreCtl
	examples              queryCanonicalExampleStoreCtl
	agent                 queryAgentExecutor
	completer             queryCompleter
	now                   func() time.Time
	model                 string
	providerModels        map[string]string
	maxTokens             int
	summaryModel          string
	summaryProviderModels map[string]string
	summaryMaxTokens      int
	maxSummaryRows        int
	maxRetrievedExamples  int
	maxCanonicalExamples  int
}

func NewQueryController(
	tenants queryMembershipCheckerCtl,
	schemas querySchemaStoreCtl,
	layers querySemanticLayerStoreCtl,
	runs queryRunStoreCtl,
	feedback queryFeedbackStoreCtl,
	examples queryCanonicalExampleStoreCtl,
	agent queryAgentExecutor,
	completer queryCompleter,
	cfg QueryControllerConfig,
) *QueryController {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = "claude-sonnet-4-6"
	}
	providerModels := llm.CloneProviderModels(cfg.ProviderModels)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4_096
	}
	summaryModel := strings.TrimSpace(cfg.SummaryModel)
	if summaryModel == "" {
		summaryModel = modelName
	}
	summaryProviderModels := llm.CloneProviderModels(cfg.SummaryProviderModels)
	if len(summaryProviderModels) == 0 {
		summaryProviderModels = llm.CloneProviderModels(providerModels)
	}
	summaryMaxTokens := cfg.SummaryMaxTokens
	if summaryMaxTokens <= 0 {
		summaryMaxTokens = 800
	}
	maxSummaryRows := cfg.MaxSummaryRows
	if maxSummaryRows <= 0 {
		maxSummaryRows = 100
	}
	maxRetrievedExamples := cfg.MaxRetrievedExamples
	if maxRetrievedExamples <= 0 {
		maxRetrievedExamples = defaultRetrievedExamplesLimit
	}
	maxCanonicalExamples := cfg.MaxCanonicalExamples
	if maxCanonicalExamples <= 0 {
		maxCanonicalExamples = defaultCanonicalExampleLimit
	}
	return &QueryController{
		tenants:               tenants,
		schemas:               schemas,
		layers:                layers,
		runs:                  runs,
		feedback:              feedback,
		examples:              examples,
		agent:                 agent,
		completer:             completer,
		now:                   now,
		model:                 modelName,
		providerModels:        providerModels,
		maxTokens:             maxTokens,
		summaryModel:          summaryModel,
		summaryProviderModels: summaryProviderModels,
		summaryMaxTokens:      summaryMaxTokens,
		maxSummaryRows:        maxSummaryRows,
		maxRetrievedExamples:  maxRetrievedExamples,
		maxCanonicalExamples:  maxCanonicalExamples,
	}
}

// AskQuestion runs the full NL-to-SQL pipeline for one question.
func (c *QueryController) AskQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	question string,
	locale model.Locale,
) (AskQuestionResult, error) {
	pipelineStart := time.Now()
	question, latestSchema, err := c.validateAskQuestionRequest(
		ctx,
		tenantID,
		clerkUserID,
		question,
	)
	if err != nil {
		return AskQuestionResult{}, err
	}

	prepared, err := c.prepareAskQuestion(
		ctx,
		tenantID,
		clerkUserID,
		question,
		latestSchema,
		locale,
	)
	if err != nil {
		return prepared.result, err
	}

	return c.executeAskQuestionPipeline(
		ctx,
		pipelineStart,
		tenantID,
		question,
		prepared,
		locale,
	)
}

func (c *QueryController) validateAskQuestionRequest(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	question string,
) (string, model.TenantSchemaVersion, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", model.TenantSchemaVersion{}, ErrQueryEmptyQuestion
	}

	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", model.TenantSchemaVersion{}, ErrQueryAccessDenied
		}
		return "", model.TenantSchemaVersion{}, err
	}

	latestSchema, err := c.schemas.LatestByTenant(ctx, tenantID)
	if errors.Is(err, repository.ErrNotFound) {
		return "", model.TenantSchemaVersion{}, ErrQueryNoSchema
	}
	if err != nil {
		return "", model.TenantSchemaVersion{}, err
	}

	return question, latestSchema, nil
}
