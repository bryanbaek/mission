package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
	"github.com/bryanbaek/mission/internal/queryerror"
	"github.com/bryanbaek/mission/internal/sqlguard"
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
)

const (
	defaultRetrievedExamplesLimit = 3
	defaultCanonicalExampleLimit  = 20
	defaultListMyQueryRunsLimit   = 20
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
) (AskQuestionResult, error) {
	pipelineStart := time.Now()
	question = strings.TrimSpace(question)
	if question == "" {
		return AskQuestionResult{}, ErrQueryEmptyQuestion
	}

	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return AskQuestionResult{}, ErrQueryAccessDenied
		}
		return AskQuestionResult{}, err
	}

	latestSchema, err := c.schemas.LatestByTenant(ctx, tenantID)
	if errors.Is(err, repository.ErrNotFound) {
		return AskQuestionResult{}, ErrQueryNoSchema
	}
	if err != nil {
		return AskQuestionResult{}, err
	}

	promptCtx, warnings, err := c.resolvePromptContext(ctx, tenantID, latestSchema)
	if err != nil {
		return AskQuestionResult{}, err
	}

	run, err := c.runs.Create(
		ctx,
		tenantID,
		latestSchema.ID,
		promptCtx.semanticLayerID,
		promptCtx.source,
		clerkUserID,
		question,
	)
	if err != nil {
		return AskQuestionResult{}, err
	}

	result := AskQuestionResult{
		QueryRunID: run.ID,
		Warnings:   append([]string(nil), warnings...),
	}

	retrievedExamples, err := c.resolveRetrievedExamples(
		ctx,
		tenantID,
		latestSchema.ID,
		question,
	)
	if err != nil {
		result, completeErr := c.completeFailedQueryRun(
			ctx,
			result,
			nil,
			warnings,
			nil,
			"generation",
			fmt.Sprintf("search canonical examples: %v", err),
		)
		if completeErr != nil {
			return result, completeErr
		}
		return result, fmt.Errorf("search canonical examples: %w", err)
	}
	retrievedExampleIDs := canonicalExampleIDs(retrievedExamples)

	attempts := make([]AskQuestionAttempt, 0, 2)
	const maxAttempts = 2
	var (
		priorSQL   string
		priorError string
	)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		genStart := time.Now()
		generated, err := c.generateSQL(
			ctx,
			question,
			promptCtx,
			retrievedExamples,
			priorSQL,
			priorError,
		)
		if err == nil {
			reqlog.Logger(ctx).InfoContext(ctx, "query.generate_sql",
				"duration_ms", time.Since(genStart).Milliseconds(),
				"attempt", attempt+1,
				"tenant_id", tenantID,
			)
		}
		if err != nil {
			attempts = append(attempts, AskQuestionAttempt{
				Stage: "generation",
				Error: err.Error(),
			})
			result.Attempts = append([]AskQuestionAttempt(nil), attempts...)
			result, completeErr := c.completeFailedQueryRun(
				ctx,
				result,
				attempts,
				warnings,
				retrievedExampleIDs,
				"generation",
				err.Error(),
			)
			if completeErr != nil {
				return result, completeErr
			}
			return result, fmt.Errorf("generate sql: %w", err)
		}

		guardResult, guardErr := sqlguard.Validate(generated.SQL)
		if guardErr != nil {
			attempts = append(attempts, AskQuestionAttempt{
				SQL:   generated.SQL,
				Stage: "validation",
				Error: guardErr.Error(),
			})
			priorSQL = generated.SQL
			priorError = guardErr.Error()
			continue
		}
		if !guardResult.OK {
			reason := guardResult.Reason
			if len(guardResult.BlockedConstructs) > 0 {
				reason = fmt.Sprintf(
					"%s (blocked: %s)",
					reason,
					strings.Join(guardResult.BlockedConstructs, ", "),
				)
			}
			attempts = append(attempts, AskQuestionAttempt{
				SQL:   generated.SQL,
				Stage: "validation",
				Error: reason,
			})
			priorSQL = generated.SQL
			priorError = reason
			continue
		}

		execStart := time.Now()
		execResult, err := c.agent.ExecuteQuery(
			ctx,
			tenantID,
			guardResult.RewrittenSQL,
		)
		if err == nil && execResult.Error == "" {
			reqlog.Logger(ctx).InfoContext(ctx, "query.execute",
				"duration_ms", time.Since(execStart).Milliseconds(),
				"sql_elapsed_ms", execResult.ElapsedMS,
				"row_count", len(execResult.Rows),
				"tenant_id", tenantID,
			)
		}
		if err != nil {
			attempts = append(attempts, AskQuestionAttempt{
				SQL:   guardResult.RewrittenSQL,
				Stage: "execution",
				Error: err.Error(),
			})
			result.Attempts = append([]AskQuestionAttempt(nil), attempts...)
			switch {
			case errors.Is(err, ErrTenantNotConnected),
				errors.Is(err, ErrSessionNotActive),
				errors.Is(err, ErrCommandRejected):
				result, completeErr := c.completeFailedQueryRun(
					ctx,
					result,
					attempts,
					warnings,
					retrievedExampleIDs,
					"execution",
					err.Error(),
				)
				if completeErr != nil {
					return result, completeErr
				}
				return result, ErrQueryAgentOffline
			}
			result, completeErr := c.completeFailedQueryRun(
				ctx,
				result,
				attempts,
				warnings,
				retrievedExampleIDs,
				"execution",
				err.Error(),
			)
			if completeErr != nil {
				return result, completeErr
			}
			return result, fmt.Errorf("execute query: %w", err)
		}
		if execResult.Error != "" {
			execError := formatExecutionError(execResult)
			attempts = append(attempts, AskQuestionAttempt{
				SQL:   guardResult.RewrittenSQL,
				Stage: "execution",
				Error: execError,
			})
			if execResult.ErrorCode == queryerror.CodePermissionDenied {
				result.Attempts = append([]AskQuestionAttempt(nil), attempts...)
				result, completeErr := c.completeFailedQueryRun(
					ctx,
					result,
					attempts,
					warnings,
					retrievedExampleIDs,
					"execution",
					execError,
				)
				if completeErr != nil {
					return result, completeErr
				}
				return result, ErrQueryAllAttemptsFailed
			}
			priorSQL = generated.SQL
			priorError = execError
			continue
		}

		attempts = append(attempts, AskQuestionAttempt{
			SQL:   generated.SQL,
			Stage: "execution",
		})

		summary, summaryErr := c.summarize(ctx, question, guardResult.RewrittenSQL, execResult)
		if summaryErr != nil {
			warnings = append(
				warnings,
				fmt.Sprintf("요약 생성에 실패했습니다: %v", summaryErr),
			)
		}
		if guardResult.LimitInjected {
			warnings = append(
				warnings,
				fmt.Sprintf(
					"안전을 위해 LIMIT %d을(를) 자동 적용했습니다.",
					sqlguard.DefaultRowLimit,
				),
			)
		}

		result = AskQuestionResult{
			QueryRunID:    run.ID,
			SQLOriginal:   generated.SQL,
			SQLExecuted:   guardResult.RewrittenSQL,
			LimitInjected: guardResult.LimitInjected,
			Columns:       execResult.Columns,
			Rows:          execResult.Rows,
			RowCount:      int64(len(execResult.Rows)),
			ElapsedMS:     execResult.ElapsedMS,
			SummaryKo:     summary,
			Warnings:      append([]string(nil), warnings...),
			Attempts:      append([]AskQuestionAttempt(nil), attempts...),
		}

		if _, err := c.runs.CompleteSucceeded(
			ctx,
			run.ID,
			result.SQLOriginal,
			result.SQLExecuted,
			toModelAttempts(attempts),
			warnings,
			result.RowCount,
			result.ElapsedMS,
			retrievedExampleIDs,
			c.now().UTC(),
		); err != nil {
			return result, fmt.Errorf("complete query run: %w", err)
		}
		reqlog.Logger(ctx).InfoContext(ctx, "query.pipeline",
			"duration_ms", time.Since(pipelineStart).Milliseconds(),
			"attempts", len(attempts),
			"tenant_id", tenantID,
		)
		return result, nil
	}

	result.Attempts = append([]AskQuestionAttempt(nil), attempts...)
	result, completeErr := c.completeFailedQueryRun(
		ctx,
		result,
		attempts,
		warnings,
		retrievedExampleIDs,
		errorStageFromAttempts(attempts),
		lastAttemptError(attempts),
	)
	if completeErr != nil {
		return result, completeErr
	}
	return result, ErrQueryAllAttemptsFailed
}

func (c *QueryController) SubmitFeedback(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
	rating model.QueryFeedbackRating,
	comment, correctedSQL string,
) (SubmitQueryFeedbackResult, error) {
	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SubmitQueryFeedbackResult{}, ErrQueryAccessDenied
		}
		return SubmitQueryFeedbackResult{}, err
	}
	if rating != model.QueryFeedbackRatingUp &&
		rating != model.QueryFeedbackRatingDown {
		return SubmitQueryFeedbackResult{}, ErrInvalidQueryFeedback
	}

	run, err := c.runs.GetByTenantAndID(ctx, tenantID, queryRunID)
	if errors.Is(err, repository.ErrNotFound) {
		return SubmitQueryFeedbackResult{}, ErrQueryRunNotFound
	}
	if err != nil {
		return SubmitQueryFeedbackResult{}, err
	}
	if run.ClerkUserID != clerkUserID {
		return SubmitQueryFeedbackResult{}, ErrQueryFeedbackAccessDenied
	}

	feedback, err := c.feedback.Upsert(
		ctx,
		queryRunID,
		clerkUserID,
		rating,
		comment,
		correctedSQL,
		c.now().UTC(),
	)
	if err != nil {
		return SubmitQueryFeedbackResult{}, err
	}
	return SubmitQueryFeedbackResult{Feedback: feedback}, nil
}

func (c *QueryController) ListMyQueryRuns(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	limit int32,
) (ListMyQueryRunsResult, error) {
	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ListMyQueryRunsResult{}, ErrQueryAccessDenied
		}
		return ListMyQueryRunsResult{}, err
	}

	normalizedLimit := int(limit)
	if normalizedLimit <= 0 || normalizedLimit > defaultListMyQueryRunsLimit {
		normalizedLimit = defaultListMyQueryRunsLimit
	}

	runs, err := c.runs.ListByTenantAndUser(
		ctx,
		tenantID,
		clerkUserID,
		normalizedLimit,
	)
	if err != nil {
		return ListMyQueryRunsResult{}, err
	}
	return ListMyQueryRunsResult{
		Runs: append([]model.TenantQueryRun(nil), runs...),
	}, nil
}

func (c *QueryController) CreateCanonicalExample(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
	question, sql, notes string,
) (model.TenantCanonicalQueryExample, error) {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.TenantCanonicalQueryExample{}, ErrQueryAccessDenied
	}
	if err != nil {
		return model.TenantCanonicalQueryExample{}, err
	}
	if membership.Role != model.RoleOwner {
		return model.TenantCanonicalQueryExample{}, ErrCanonicalQueryExampleOwnerOnly
	}
	if strings.TrimSpace(question) == "" || strings.TrimSpace(sql) == "" {
		return model.TenantCanonicalQueryExample{}, ErrInvalidCanonicalQueryExample
	}

	run, err := c.runs.GetByTenantAndID(ctx, tenantID, queryRunID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.TenantCanonicalQueryExample{}, ErrQueryRunNotFound
	}
	if err != nil {
		return model.TenantCanonicalQueryExample{}, err
	}

	return c.examples.Create(
		ctx,
		tenantID,
		run.SchemaVersionID,
		run.ID,
		clerkUserID,
		question,
		sql,
		notes,
	)
}

func (c *QueryController) ListCanonicalExamples(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (ListCanonicalQueryExamplesResult, error) {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ListCanonicalQueryExamplesResult{}, ErrQueryAccessDenied
	}
	if err != nil {
		return ListCanonicalQueryExamplesResult{}, err
	}
	examples, err := c.examples.ListActiveByTenant(
		ctx,
		tenantID,
		c.maxCanonicalExamples,
	)
	if err != nil {
		return ListCanonicalQueryExamplesResult{}, err
	}
	return ListCanonicalQueryExamplesResult{
		ViewerCanManage: membership.Role == model.RoleOwner,
		Examples:        examples,
	}, nil
}

func (c *QueryController) ArchiveCanonicalExample(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	exampleID uuid.UUID,
) error {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrQueryAccessDenied
	}
	if err != nil {
		return err
	}
	if membership.Role != model.RoleOwner {
		return ErrCanonicalQueryExampleOwnerOnly
	}
	if err := c.examples.Archive(ctx, tenantID, exampleID, c.now().UTC()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCanonicalQueryExampleNotFound
		}
		return err
	}
	return nil
}

func (c *QueryController) completeFailedQueryRun(
	ctx context.Context,
	result AskQuestionResult,
	attempts []AskQuestionAttempt,
	warnings []string,
	retrievedExampleIDs []uuid.UUID,
	errorStage, errorMessage string,
) (AskQuestionResult, error) {
	if result.QueryRunID == uuid.Nil {
		return result, nil
	}
	if _, err := c.runs.CompleteFailed(
		ctx,
		result.QueryRunID,
		toModelAttempts(attempts),
		warnings,
		retrievedExampleIDs,
		errorStage,
		errorMessage,
		c.now().UTC(),
	); err != nil {
		return result, fmt.Errorf("complete query run: %w", err)
	}
	return result, nil
}

func toModelAttempts(attempts []AskQuestionAttempt) []model.QueryRunAttempt {
	out := make([]model.QueryRunAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, model.QueryRunAttempt{
			SQL:   attempt.SQL,
			Error: attempt.Error,
			Stage: attempt.Stage,
		})
	}
	return out
}

func errorStageFromAttempts(attempts []AskQuestionAttempt) string {
	for i := len(attempts) - 1; i >= 0; i-- {
		if strings.TrimSpace(attempts[i].Stage) != "" {
			return attempts[i].Stage
		}
	}
	return ""
}

func lastAttemptError(attempts []AskQuestionAttempt) string {
	for i := len(attempts) - 1; i >= 0; i-- {
		if strings.TrimSpace(attempts[i].Error) != "" {
			return attempts[i].Error
		}
	}
	return ""
}

func canonicalExampleIDs(
	examples []model.TenantCanonicalQueryExample,
) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(examples))
	for _, example := range examples {
		if example.ID == uuid.Nil {
			continue
		}
		out = append(out, example.ID)
	}
	return out
}

func (c *QueryController) resolveRetrievedExamples(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
	question string,
) ([]model.TenantCanonicalQueryExample, error) {
	sameSchemaExamples, err := c.examples.SearchActiveByQuestion(
		ctx,
		tenantID,
		question,
		c.maxRetrievedExamples,
		&schemaVersionID,
	)
	if err != nil {
		return nil, err
	}
	if len(sameSchemaExamples) > 0 {
		return sameSchemaExamples, nil
	}
	return c.examples.SearchActiveByQuestion(
		ctx,
		tenantID,
		question,
		c.maxRetrievedExamples,
		nil,
	)
}

// queryPromptContext is the context fed into the SQL-generation prompt.
// Either semanticLayer is populated (preferred) or schemaBlob is populated
// (fallback) — if both are present the semantic layer takes precedence but
// the schema blob is also included for fidelity on column types.
type queryPromptContext struct {
	schemaBlob      model.SchemaBlob
	schemaRaw       json.RawMessage
	semanticLayer   *model.SemanticLayerContent
	semanticLayerID *uuid.UUID
	source          model.QueryPromptContextSource
}

func (c *QueryController) resolvePromptContext(
	ctx context.Context,
	tenantID uuid.UUID,
	schemaVersion model.TenantSchemaVersion,
) (queryPromptContext, []string, error) {
	var blob model.SchemaBlob
	if err := json.Unmarshal(schemaVersion.Blob, &blob); err != nil {
		return queryPromptContext{}, nil, fmt.Errorf(
			"unmarshal schema blob: %w",
			err,
		)
	}

	warnings := make([]string, 0, 1)

	approved, err := c.layers.LatestApprovedBySchemaVersion(
		ctx,
		tenantID,
		schemaVersion.ID,
	)
	switch {
	case err == nil:
		content, decodeErr := decodeSemanticLayerContent(approved)
		if decodeErr != nil {
			return queryPromptContext{}, nil, decodeErr
		}
		approvedID := approved.ID
		return queryPromptContext{
			schemaBlob:      blob,
			schemaRaw:       schemaVersion.Blob,
			semanticLayer:   &content,
			semanticLayerID: &approvedID,
			source:          model.QueryPromptContextSourceApproved,
		}, warnings, nil
	case !errors.Is(err, repository.ErrNotFound):
		return queryPromptContext{}, nil, err
	}

	draft, err := c.layers.LatestDraftBySchemaVersion(
		ctx,
		tenantID,
		schemaVersion.ID,
	)
	switch {
	case err == nil:
		content, decodeErr := decodeSemanticLayerContent(draft)
		if decodeErr != nil {
			return queryPromptContext{}, nil, decodeErr
		}
		warnings = append(
			warnings,
			"승인된 시맨틱 레이어가 없어 초안(draft) 레이어를 사용했습니다.",
		)
		draftID := draft.ID
		return queryPromptContext{
			schemaBlob:      blob,
			schemaRaw:       schemaVersion.Blob,
			semanticLayer:   &content,
			semanticLayerID: &draftID,
			source:          model.QueryPromptContextSourceDraft,
		}, warnings, nil
	case !errors.Is(err, repository.ErrNotFound):
		return queryPromptContext{}, nil, err
	}

	warnings = append(
		warnings,
		"시맨틱 레이어가 없어 원본 스키마만으로 SQL을 생성했습니다. 정확도가 낮을 수 있습니다.",
	)
	return queryPromptContext{
		schemaBlob: blob,
		schemaRaw:  schemaVersion.Blob,
		source:     model.QueryPromptContextSourceRawSchema,
	}, warnings, nil
}

func decodeSemanticLayerContent(
	layer model.TenantSemanticLayer,
) (model.SemanticLayerContent, error) {
	var content model.SemanticLayerContent
	if err := json.Unmarshal(layer.Content, &content); err != nil {
		return model.SemanticLayerContent{}, fmt.Errorf(
			"unmarshal semantic layer content: %w",
			err,
		)
	}
	return content, nil
}

type generatedSQL struct {
	Reasoning string `json:"reasoning"`
	SQL       string `json:"sql"`
	Notes     string `json:"notes"`
}

func (c *QueryController) generateSQL(
	ctx context.Context,
	question string,
	promptCtx queryPromptContext,
	examples []model.TenantCanonicalQueryExample,
	priorSQL, priorError string,
) (generatedSQL, error) {
	cached, dynamic := buildQueryUserPrompt(
		question,
		promptCtx,
		examples,
		priorSQL,
		priorError,
	)

	resp, err := c.completer.Complete(ctx, llm.CompletionRequest{
		System: querySystemPrompt,
		Messages: []llm.Message{{
			Role:          "user",
			CachedContent: cached,
			Content:       dynamic,
		}},
		Model:          c.model,
		ProviderModels: llm.CloneProviderModels(c.providerModels),
		MaxTokens:      c.maxTokens,
		OutputFormat: &llm.OutputFormat{
			Name:   "text_to_sql",
			Schema: querySQLOutputSchema(),
		},
		CacheControl: &llm.CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		},
	})
	if err != nil {
		return generatedSQL{}, err
	}

	var out generatedSQL
	if err := json.Unmarshal([]byte(resp.Content), &out); err != nil {
		return generatedSQL{}, fmt.Errorf(
			"decode generated sql: %w (raw=%q)",
			err,
			resp.Content,
		)
	}
	out.SQL = strings.TrimSpace(out.SQL)
	if out.SQL == "" {
		return generatedSQL{}, errors.New("LLM returned empty SQL")
	}
	return out, nil
}

func (c *QueryController) summarize(
	ctx context.Context,
	question, executedSQL string,
	execResult AgentExecuteQueryResult,
) (string, error) {
	userPrompt := buildSummaryUserPrompt(
		question,
		executedSQL,
		execResult,
		c.maxSummaryRows,
	)

	resp, err := c.completer.Complete(ctx, llm.CompletionRequest{
		System: querySummarySystemPrompt,
		Messages: []llm.Message{{
			Role:    "user",
			Content: userPrompt,
		}},
		Model:          c.summaryModel,
		ProviderModels: llm.CloneProviderModels(c.summaryProviderModels),
		MaxTokens:      c.summaryMaxTokens,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func formatExecutionError(result AgentExecuteQueryResult) string {
	switch {
	case result.ErrorReason != "":
		if len(result.BlockedConstructs) > 0 {
			return fmt.Sprintf(
				"%s (%s)",
				result.ErrorReason,
				strings.Join(result.BlockedConstructs, ", "),
			)
		}
		return result.ErrorReason
	case result.Error != "":
		return result.Error
	default:
		return "unknown execution error"
	}
}

// buildQueryUserPrompt splits the LLM user prompt into two parts for
// Anthropic prompt caching: cachedContent (semi-static schema context) and
// dynamicContent (per-query question and retry info).
func buildQueryUserPrompt(
	question string,
	promptCtx queryPromptContext,
	examples []model.TenantCanonicalQueryExample,
	priorSQL, priorError string,
) (cachedContent, dynamicContent string) {
	var cached strings.Builder

	if len(examples) > 0 {
		cached.WriteString("## 승인된 예시 쿼리\n")
		for index, example := range examples {
			fmt.Fprintf(&cached, "### 예시 %d\n", index+1)
			cached.WriteString("질문: ")
			cached.WriteString(example.Question)
			cached.WriteString("\n")
			if strings.TrimSpace(example.Notes) != "" {
				cached.WriteString("노트: ")
				cached.WriteString(example.Notes)
				cached.WriteString("\n")
			}
			cached.WriteString("SQL:\n```sql\n")
			cached.WriteString(example.SQL)
			cached.WriteString("\n```\n\n")
		}
	}

	if promptCtx.semanticLayer != nil {
		cached.WriteString("## 시맨틱 레이어 (")
		cached.WriteString(string(promptCtx.source))
		cached.WriteString(")\n")
		payload, err := json.MarshalIndent(promptCtx.semanticLayer, "", "  ")
		if err == nil {
			cached.Write(payload)
		}
		cached.WriteString("\n\n")
	}

	cached.WriteString("## 원본 MySQL 스키마\n")
	cached.Write(promptCtx.schemaRaw)
	cached.WriteString("\n\n")

	cached.WriteString(strings.TrimSpace(`
## 지시 사항
- 위 스키마, 시맨틱 레이어, 승인된 예시 쿼리만 근거로 SQL을 작성합니다.
- 읽기 전용 SELECT (또는 WITH / SHOW)만 사용합니다.
- 존재하지 않는 테이블이나 컬럼은 절대 참조하지 않습니다.
- 승인된 예시 쿼리의 패턴은 재사용할 수 있지만, 현재 질문에 맞게 필요한 부분만 조정합니다.
- LIMIT이 필요하면 명시적으로 선언하지만, 누락해도 시스템이 자동으로 LIMIT 1000을 붙입니다.
- reasoning 필드에는 어떤 테이블/컬럼을 사용했고 왜 그렇게 조인했는지 한국어로 간단히 기록합니다.
- sql 필드에는 실행 가능한 단일 MySQL 문만 담습니다. 주석이나 세미콜론은 포함하지 않습니다.
- notes 필드에는 추정한 부분이나 사용자가 알아야 할 주의 사항을 한국어로 적습니다.
`))

	var dynamic strings.Builder

	dynamic.WriteString("## 사용자 질문\n")
	dynamic.WriteString(question)
	dynamic.WriteString("\n\n")

	if priorSQL != "" {
		dynamic.WriteString("## 이전 시도 실패\n")
		dynamic.WriteString("아래 SQL을 생성했지만 검증 또는 실행에 실패했습니다. ")
		dynamic.WriteString("원인을 고려하여 수정된 SQL을 다시 만들어 주세요.\n\n")
		dynamic.WriteString("이전 SQL:\n```sql\n")
		dynamic.WriteString(priorSQL)
		dynamic.WriteString("\n```\n\n실패 사유:\n")
		dynamic.WriteString(priorError)
		dynamic.WriteString("\n\n")
	}

	return cached.String(), dynamic.String()
}

func buildSummaryUserPrompt(
	question, executedSQL string,
	execResult AgentExecuteQueryResult,
	maxRows int,
) string {
	var builder strings.Builder

	builder.WriteString("## 원본 질문\n")
	builder.WriteString(question)
	builder.WriteString("\n\n")

	builder.WriteString("## 실행한 SQL\n```sql\n")
	builder.WriteString(executedSQL)
	builder.WriteString("\n```\n\n")

	builder.WriteString("## 결과 메타데이터\n")
	fmt.Fprintf(&builder, "- 컬럼: %s\n", strings.Join(execResult.Columns, ", "))
	fmt.Fprintf(&builder, "- 행 수: %d\n", len(execResult.Rows))
	fmt.Fprintf(&builder, "- 실행 시간(ms): %d\n", execResult.ElapsedMS)
	builder.WriteString("\n")

	truncated := execResult.Rows
	if len(truncated) > maxRows {
		truncated = truncated[:maxRows]
	}
	builder.WriteString("## 결과 데이터 (JSON)\n")
	payload, err := json.MarshalIndent(truncated, "", "  ")
	if err != nil {
		payload = []byte("[]")
	}
	builder.Write(payload)
	builder.WriteString("\n")
	if len(execResult.Rows) > maxRows {
		fmt.Fprintf(
			&builder,
			"\n(전체 %d행 중 상위 %d행만 표시)\n",
			len(execResult.Rows),
			maxRows,
		)
	}

	builder.WriteString("\n## 지시 사항\n")
	builder.WriteString(strings.TrimSpace(`
- 결과를 바탕으로 질문에 대한 답을 한국어로 자연스럽게 요약합니다.
- 첫 문장은 핵심 답이어야 합니다.
- 구체적인 숫자/값은 결과에서 인용합니다.
- 데이터에 없는 내용은 추측하지 않습니다. 불확실하면 "데이터 기준"이라고 명시합니다.
- 3~5문장 이내로 작성합니다. 목록이나 마크다운은 사용하지 않습니다.
`))

	return builder.String()
}

const querySystemPrompt = `
당신은 한국어로 일하는 신중한 MySQL 분석가입니다.
사용자의 자연어 질문을 주어진 스키마와 시맨틱 레이어만 근거로 읽기 전용 MySQL SELECT 문으로 변환합니다.

반드시 지킬 규칙:
- 존재하지 않는 테이블/컬럼을 만들어내지 않습니다. 확신이 없으면 notes에 적어 두세요.
- DELETE, UPDATE, INSERT, REPLACE, CREATE, DROP, ALTER, TRUNCATE, GRANT, REVOKE, LOCK, SET, CALL, LOAD 등은 절대 생성하지 않습니다.
- SELECT ... INTO OUTFILE, INTO DUMPFILE, 공유 외 FOR UPDATE 잠금은 허용되지 않습니다.
- 세미콜론으로 여러 문장을 이어 붙이지 않습니다. 하나의 문장만 반환합니다.
- 주석(-- , /* */, #)은 생성하지 않습니다.
- 응답은 반드시 주어진 JSON 스키마를 따르며 reasoning, sql, notes 세 필드를 모두 포함합니다.
`

const querySummarySystemPrompt = `
당신은 한국어로 일하는 데이터 분석 요약가입니다.
쿼리 결과를 보고 사용자에게 명확하고 짧은 한국어 답변을 제공합니다.
데이터에 없는 사실은 지어내지 않습니다. 추측이 섞이면 "데이터 기준"이라고 명시합니다.
`

func querySQLOutputSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reasoning": stringSchema,
			"sql":       stringSchema,
			"notes":     stringSchema,
		},
		"required":             []string{"reasoning", "sql", "notes"},
		"additionalProperties": false,
	}
}
