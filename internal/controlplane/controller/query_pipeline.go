package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
	"github.com/bryanbaek/mission/internal/queryerror"
	"github.com/bryanbaek/mission/internal/sqlguard"
)

func (c *QueryController) executeAskQuestionPipeline(
	ctx context.Context,
	pipelineStart time.Time,
	tenantID uuid.UUID,
	question string,
	prepared preparedAskQuestion,
	locale model.Locale,
) (AskQuestionResult, error) {
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
			prepared.promptCtx,
			prepared.retrievedExamples,
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
			return c.failAskQuestion(
				ctx,
				prepared,
				attempts,
				"generation",
				err.Error(),
				fmt.Errorf("generate sql: %w", err),
			)
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
			switch {
			case errors.Is(err, ErrTenantNotConnected),
				errors.Is(err, ErrSessionNotActive),
				errors.Is(err, ErrCommandRejected):
				return c.failAskQuestion(
					ctx,
					prepared,
					attempts,
					"execution",
					err.Error(),
					ErrQueryAgentOffline,
				)
			default:
				return c.failAskQuestion(
					ctx,
					prepared,
					attempts,
					"execution",
					err.Error(),
					fmt.Errorf("execute query: %w", err),
				)
			}
		}
		if execResult.Error != "" {
			execError := formatExecutionError(execResult)
			attempts = append(attempts, AskQuestionAttempt{
				SQL:   guardResult.RewrittenSQL,
				Stage: "execution",
				Error: execError,
			})
			if execResult.ErrorCode == queryerror.CodePermissionDenied {
				return c.failAskQuestion(
					ctx,
					prepared,
					attempts,
					"execution",
					execError,
					ErrQueryAllAttemptsFailed,
				)
			}
			priorSQL = generated.SQL
			priorError = execError
			continue
		}

		attempts = append(attempts, AskQuestionAttempt{
			SQL:   generated.SQL,
			Stage: "execution",
		})

		return c.completeSuccessfulAskQuestion(
			ctx,
			pipelineStart,
			tenantID,
			question,
			prepared,
			generated.SQL,
			guardResult,
			execResult,
			attempts,
			locale,
		)
	}

	return c.failAskQuestion(
		ctx,
		prepared,
		attempts,
		errorStageFromAttempts(attempts),
		lastAttemptError(attempts),
		ErrQueryAllAttemptsFailed,
	)
}

func (c *QueryController) completeSuccessfulAskQuestion(
	ctx context.Context,
	pipelineStart time.Time,
	tenantID uuid.UUID,
	question string,
	prepared preparedAskQuestion,
	sqlOriginal string,
	guardResult sqlguard.Result,
	execResult AgentExecuteQueryResult,
	attempts []AskQuestionAttempt,
	locale model.Locale,
) (AskQuestionResult, error) {
	warnings := append([]string(nil), prepared.warnings...)
	summaryStart := time.Now()
	summary, summaryErr := c.summarize(
		ctx,
		question,
		guardResult.RewrittenSQL,
		execResult,
	)
	summaryAttrs := []any{
		"duration_ms", time.Since(summaryStart).Milliseconds(),
		"tenant_id", tenantID,
		"row_count", len(execResult.Rows),
	}
	if summaryErr != nil {
		summaryAttrs = append(summaryAttrs, "error", summaryErr.Error())
		warnings = append(warnings, warnSummaryFailed(locale, summaryErr))
	}
	reqlog.Logger(ctx).InfoContext(ctx, "query.summarize", summaryAttrs...)

	if guardResult.LimitInjected {
		warnings = append(warnings, warnLimitInjected(locale))
	}

	result := AskQuestionResult{
		QueryRunID:    prepared.run.ID,
		SQLOriginal:   sqlOriginal,
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
		prepared.run.ID,
		result.SQLOriginal,
		result.SQLExecuted,
		toModelAttempts(attempts),
		warnings,
		result.RowCount,
		result.ElapsedMS,
		prepared.retrievedExampleIDs,
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

func (c *QueryController) failAskQuestion(
	ctx context.Context,
	prepared preparedAskQuestion,
	attempts []AskQuestionAttempt,
	errorStage, errorMessage string,
	returnErr error,
) (AskQuestionResult, error) {
	prepared.result.Attempts = append([]AskQuestionAttempt(nil), attempts...)
	result, completeErr := c.completeFailedQueryRun(
		ctx,
		prepared.result,
		attempts,
		prepared.warnings,
		prepared.retrievedExampleIDs,
		errorStage,
		errorMessage,
	)
	if completeErr != nil {
		return result, completeErr
	}
	return result, returnErr
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
