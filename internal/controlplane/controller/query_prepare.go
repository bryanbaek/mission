package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
)

type queryPromptContextResult struct {
	promptCtx queryPromptContext
	warnings  []string
	err       error
}

type queryRetrievedExamplesResult struct {
	examples []model.TenantCanonicalQueryExample
	err      error
}

type preparedAskQuestion struct {
	result              AskQuestionResult
	run                 model.TenantQueryRun
	promptCtx           queryPromptContext
	warnings            []string
	retrievedExamples   []model.TenantCanonicalQueryExample
	retrievedExampleIDs []uuid.UUID
}

func (c *QueryController) prepareAskQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	question string,
	latestSchema model.TenantSchemaVersion,
) (preparedAskQuestion, error) {
	promptCtxCh, retrievedExamplesCh := c.startAskQuestionPreparation(
		ctx,
		tenantID,
		latestSchema,
		question,
	)

	promptCtxResult := <-promptCtxCh
	if promptCtxResult.err != nil {
		return preparedAskQuestion{}, promptCtxResult.err
	}

	run, err := c.runs.Create(
		ctx,
		tenantID,
		latestSchema.ID,
		promptCtxResult.promptCtx.semanticLayerID,
		promptCtxResult.promptCtx.source,
		clerkUserID,
		question,
	)
	if err != nil {
		return preparedAskQuestion{}, err
	}

	prepared := preparedAskQuestion{
		result: AskQuestionResult{
			QueryRunID: run.ID,
			Warnings:   append([]string(nil), promptCtxResult.warnings...),
		},
		run:       run,
		promptCtx: promptCtxResult.promptCtx,
		warnings:  append([]string(nil), promptCtxResult.warnings...),
	}

	retrievedExamplesResult := <-retrievedExamplesCh
	if retrievedExamplesResult.err != nil {
		return prepared, c.failPreparation(
			ctx,
			prepared,
			fmt.Sprintf(
				"search canonical examples: %v",
				retrievedExamplesResult.err,
			),
			fmt.Errorf(
				"search canonical examples: %w",
				retrievedExamplesResult.err,
			),
		)
	}

	prepared.retrievedExamples = retrievedExamplesResult.examples
	prepared.retrievedExampleIDs = canonicalExampleIDs(
		retrievedExamplesResult.examples,
	)
	return prepared, nil
}

func (c *QueryController) startAskQuestionPreparation(
	ctx context.Context,
	tenantID uuid.UUID,
	latestSchema model.TenantSchemaVersion,
	question string,
) (
	<-chan queryPromptContextResult,
	<-chan queryRetrievedExamplesResult,
) {
	promptCtxCh := make(chan queryPromptContextResult, 1)
	retrievedExamplesCh := make(chan queryRetrievedExamplesResult, 1)

	go func() {
		promptCtx, warnings, err := c.resolvePromptContext(
			ctx,
			tenantID,
			latestSchema,
		)
		promptCtxCh <- queryPromptContextResult{
			promptCtx: promptCtx,
			warnings:  warnings,
			err:       err,
		}
	}()

	go func() {
		start := time.Now()
		examples, err := c.resolveRetrievedExamples(
			ctx,
			tenantID,
			latestSchema.ID,
			question,
		)
		attrs := []any{
			"duration_ms", time.Since(start).Milliseconds(),
			"tenant_id", tenantID,
			"schema_version_id", latestSchema.ID,
			"count", len(examples),
		}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		reqlog.Logger(ctx).InfoContext(ctx, "query.retrieve_examples", attrs...)
		retrievedExamplesCh <- queryRetrievedExamplesResult{
			examples: examples,
			err:      err,
		}
	}()

	return promptCtxCh, retrievedExamplesCh
}

func (c *QueryController) failPreparation(
	ctx context.Context,
	prepared preparedAskQuestion,
	errorMessage string,
	returnErr error,
) error {
	_, completeErr := c.completeFailedQueryRun(
		ctx,
		prepared.result,
		nil,
		prepared.warnings,
		nil,
		"generation",
		errorMessage,
	)
	if completeErr != nil {
		return completeErr
	}
	return returnErr
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
// (fallback). If both are present, semanticLayer takes precedence while the
// schema blob remains available for column-type fidelity.
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
