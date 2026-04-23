package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type queryMemoryDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type queryRunCompletion struct {
	Status              model.QueryRunStatus
	SQLOriginal         string
	SQLExecuted         string
	Attempts            []model.QueryRunAttempt
	Warnings            []string
	RowCount            int64
	ElapsedMS           int64
	RetrievedExampleIDs []uuid.UUID
	ErrorStage          string
	ErrorMessage        string
	CompletedAt         time.Time
}

type queryRunScanner interface {
	Scan(dest ...any) error
}

const tenantQueryRunSelectColumns = `
			id,
			tenant_id,
			schema_version_id,
			semantic_layer_id,
			prompt_context_source::text,
			clerk_user_id,
			question,
			status::text,
			sql_original,
			sql_executed,
			attempts_json,
			warnings_json,
			row_count,
			elapsed_ms,
			error_stage,
			error_message,
			created_at,
	completed_at
`

type TenantQueryRunRepository struct {
	db queryMemoryDB
}

func NewTenantQueryRunRepository(pool *pgxpool.Pool) *TenantQueryRunRepository {
	return &TenantQueryRunRepository{db: pool}
}

func (r *TenantQueryRunRepository) Create(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
	semanticLayerID *uuid.UUID,
	source model.QueryPromptContextSource,
	clerkUserID, question string,
) (model.TenantQueryRun, error) {
	rec, err := scanTenantQueryRun(r.db.QueryRow(ctx, `
		INSERT INTO tenant_query_runs (
			tenant_id,
			schema_version_id,
			semantic_layer_id,
			prompt_context_source,
			clerk_user_id,
			question,
			status
		) VALUES ($1, $2, $3, $4::query_prompt_context_source, $5, $6, 'running')
		RETURNING
`+tenantQueryRunSelectColumns+`
	`, tenantID, schemaVersionID, semanticLayerID, string(source), clerkUserID, question))
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("insert tenant query run: %w", err)
	}
	return rec, nil
}

func (r *TenantQueryRunRepository) GetByTenantAndID(
	ctx context.Context,
	tenantID, id uuid.UUID,
) (model.TenantQueryRun, error) {
	return r.selectOne(ctx, `
		SELECT
`+tenantQueryRunSelectColumns+`
		FROM tenant_query_runs
		WHERE tenant_id = $1
		  AND id = $2
	`, tenantID, id)
}

func (r *TenantQueryRunRepository) ListByTenantAndUser(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	limit int,
) ([]model.TenantQueryRun, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
`+tenantQueryRunSelectColumns+`
		FROM tenant_query_runs
		WHERE tenant_id = $1
		  AND clerk_user_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, tenantID, clerkUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tenant query runs: %w", err)
	}
	defer rows.Close()

	out := make([]model.TenantQueryRun, 0)
	for rows.Next() {
		rec, err := scanTenantQueryRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tenant query run: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant query runs: %w", err)
	}
	return out, nil
}

func (r *TenantQueryRunRepository) MarkReviewed(
	ctx context.Context,
	tenantID, id uuid.UUID,
	reviewedAt time.Time,
	reviewedByUserID string,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE tenant_query_runs
		SET
			reviewed_at = $3,
			reviewed_by_user_id = $4
		WHERE tenant_id = $1
		  AND id = $2
	`, tenantID, id, reviewedAt.UTC(), strings.TrimSpace(reviewedByUserID))
	if err != nil {
		return fmt.Errorf("mark query run reviewed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TenantQueryRunRepository) CompleteSucceeded(
	ctx context.Context,
	id uuid.UUID,
	sqlOriginal, sqlExecuted string,
	attempts []model.QueryRunAttempt,
	warnings []string,
	rowCount, elapsedMS int64,
	retrievedExampleIDs []uuid.UUID,
	completedAt time.Time,
) (model.TenantQueryRun, error) {
	return r.complete(ctx, id, queryRunCompletion{
		Status:              model.QueryRunStatusSucceeded,
		SQLOriginal:         sqlOriginal,
		SQLExecuted:         sqlExecuted,
		Attempts:            attempts,
		Warnings:            warnings,
		RowCount:            rowCount,
		ElapsedMS:           elapsedMS,
		RetrievedExampleIDs: retrievedExampleIDs,
		CompletedAt:         completedAt,
	})
}

func (r *TenantQueryRunRepository) CompleteFailed(
	ctx context.Context,
	id uuid.UUID,
	attempts []model.QueryRunAttempt,
	warnings []string,
	retrievedExampleIDs []uuid.UUID,
	errorStage, errorMessage string,
	completedAt time.Time,
) (model.TenantQueryRun, error) {
	return r.complete(ctx, id, queryRunCompletion{
		Status:              model.QueryRunStatusFailed,
		Attempts:            attempts,
		Warnings:            warnings,
		RetrievedExampleIDs: retrievedExampleIDs,
		ErrorStage:          strings.TrimSpace(errorStage),
		ErrorMessage:        strings.TrimSpace(errorMessage),
		CompletedAt:         completedAt,
	})
}

func (r *TenantQueryRunRepository) complete(
	ctx context.Context,
	id uuid.UUID,
	params queryRunCompletion,
) (model.TenantQueryRun, error) {
	attemptsJSON, err := json.Marshal(params.Attempts)
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("marshal query attempts: %w", err)
	}
	warningsJSON, err := json.Marshal(params.Warnings)
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("marshal query warnings: %w", err)
	}
	retrievedExampleIDsJSON, err := marshalUUIDs(params.RetrievedExampleIDs)
	if err != nil {
		return model.TenantQueryRun{}, err
	}
	rec, err := scanTenantQueryRun(r.db.QueryRow(ctx, `
		UPDATE tenant_query_runs
		SET
			status = $2::query_run_status,
			sql_original = $3,
			sql_executed = $4,
			attempts_json = $5::jsonb,
			warnings_json = $6::jsonb,
			row_count = $7,
			elapsed_ms = $8,
			retrieved_example_ids_json = $9::jsonb,
			error_stage = $10,
			error_message = $11,
			completed_at = $12
		WHERE id = $1
		RETURNING
`+tenantQueryRunSelectColumns+`
	`, id,
		string(params.Status),
		nullableString(params.SQLOriginal),
		nullableString(params.SQLExecuted),
		attemptsJSON,
		warningsJSON,
		params.RowCount,
		params.ElapsedMS,
		retrievedExampleIDsJSON,
		nullableString(params.ErrorStage),
		nullableString(params.ErrorMessage),
		params.CompletedAt.UTC(),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantQueryRun{}, ErrNotFound
	}
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("update tenant query run: %w", err)
	}
	return rec, nil
}

func (r *TenantQueryRunRepository) selectOne(
	ctx context.Context,
	query string,
	args ...any,
) (model.TenantQueryRun, error) {
	rec, err := scanTenantQueryRun(r.db.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantQueryRun{}, ErrNotFound
	}
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("select tenant query run: %w", err)
	}
	return rec, nil
}

func scanTenantQueryRun(scanner queryRunScanner) (model.TenantQueryRun, error) {
	var rec model.TenantQueryRun
	var (
		sqlOriginal  sql.NullString
		sqlExecuted  sql.NullString
		errorStage   sql.NullString
		errorMessage sql.NullString
		attemptsJSON []byte
		warningsJSON []byte
	)
	if err := scanner.Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.SchemaVersionID,
		&rec.SemanticLayerID,
		&rec.PromptContextSource,
		&rec.ClerkUserID,
		&rec.Question,
		&rec.Status,
		&sqlOriginal,
		&sqlExecuted,
		&attemptsJSON,
		&warningsJSON,
		&rec.RowCount,
		&rec.ElapsedMS,
		&errorStage,
		&errorMessage,
		&rec.CreatedAt,
		&rec.CompletedAt,
	); err != nil {
		return model.TenantQueryRun{}, err
	}
	rec.SQLOriginal = sqlOriginal.String
	rec.SQLExecuted = sqlExecuted.String
	rec.ErrorStage = errorStage.String
	rec.ErrorMessage = errorMessage.String

	attempts, err := decodeQueryRunAttempts(attemptsJSON)
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("decode query attempts: %w", err)
	}
	warnings, err := decodeQueryRunWarnings(warningsJSON)
	if err != nil {
		return model.TenantQueryRun{}, fmt.Errorf("decode query warnings: %w", err)
	}
	rec.Attempts = attempts
	rec.Warnings = warnings
	return rec, nil
}

func decodeQueryRunAttempts(payload []byte) ([]model.QueryRunAttempt, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, nil
	}
	var attempts []model.QueryRunAttempt
	if err := json.Unmarshal(payload, &attempts); err != nil {
		return nil, err
	}
	return attempts, nil
}

func decodeQueryRunWarnings(payload []byte) ([]string, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, nil
	}
	var warnings []string
	if err := json.Unmarshal(payload, &warnings); err != nil {
		return nil, err
	}
	return warnings, nil
}

func marshalUUIDs(ids []uuid.UUID) ([]byte, error) {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		values = append(values, id.String())
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal retrieved example ids: %w", err)
	}
	return payload, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
