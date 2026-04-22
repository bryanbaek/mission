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

type queryRunReviewScanner interface {
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

const tenantQueryRunReviewSelectColumns = tenantQueryRunSelectColumns + `,
			latest_feedback.clerk_user_id,
			latest_feedback.rating::text,
			latest_feedback.comment,
			latest_feedback.corrected_sql,
			latest_feedback.created_at,
			latest_feedback.updated_at,
			(active_example.source_query_run_id IS NOT NULL) AS has_active_canonical_example,
			r.reviewed_at,
			COALESCE(latest_feedback.updated_at, r.created_at) AS review_signal_at
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

func (r *TenantQueryRunRepository) ListReviewQueue(
	ctx context.Context,
	tenantID uuid.UUID,
	filter model.ReviewQueueFilter,
	limit int,
) ([]model.TenantQueryRunReviewItem, error) {
	rows, err := r.db.Query(ctx, listReviewQueueSQL(filter), tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list review queue: %w", err)
	}
	defer rows.Close()

	out := make([]model.TenantQueryRunReviewItem, 0)
	for rows.Next() {
		rec, err := scanTenantQueryRunReviewItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review queue item: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review queue: %w", err)
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

func listReviewQueueSQL(filter model.ReviewQueueFilter) string {
	conditions := []string{
		"r.tenant_id = $1",
		`(
			r.status = 'failed'
			OR latest_feedback.rating = 'down'
			OR NULLIF(BTRIM(latest_feedback.corrected_sql), '') IS NOT NULL
		)`,
	}

	if filter == model.ReviewQueueFilterOpen || filter == "" {
		conditions = append(
			conditions,
			"active_example.source_query_run_id IS NULL",
			"r.reviewed_at IS NULL",
		)
	}

	return `
		WITH latest_feedback AS (
			SELECT DISTINCT ON (query_run_id)
				query_run_id,
				clerk_user_id,
				rating,
				comment,
				corrected_sql,
				created_at,
				updated_at
			FROM tenant_query_feedback
			ORDER BY query_run_id, updated_at DESC, created_at DESC, clerk_user_id ASC
		)
		SELECT
` + tenantQueryRunReviewSelectColumns + `
		FROM tenant_query_runs r
		LEFT JOIN latest_feedback
			ON latest_feedback.query_run_id = r.id
		LEFT JOIN LATERAL (
			SELECT source_query_run_id
			FROM tenant_canonical_query_examples
			WHERE tenant_id = r.tenant_id
			  AND source_query_run_id = r.id
			  AND archived_at IS NULL
			LIMIT 1
		) AS active_example ON TRUE
		WHERE ` + strings.Join(conditions, `
		  AND `) + `
		ORDER BY COALESCE(latest_feedback.updated_at, r.created_at) DESC, r.created_at DESC
		LIMIT $2
	`
}

func scanTenantQueryRunReviewItem(
	scanner queryRunReviewScanner,
) (model.TenantQueryRunReviewItem, error) {
	var rec model.TenantQueryRunReviewItem
	var (
		sqlOriginal             sql.NullString
		sqlExecuted             sql.NullString
		errorStage              sql.NullString
		errorMessage            sql.NullString
		attemptsJSON            []byte
		warningsJSON            []byte
		latestFeedbackUserID    sql.NullString
		latestFeedbackRating    sql.NullString
		latestFeedbackComment   sql.NullString
		latestFeedbackCorrected sql.NullString
		latestFeedbackCreatedAt sql.NullTime
		latestFeedbackUpdatedAt sql.NullTime
	)
	if err := scanner.Scan(
		&rec.Run.ID,
		&rec.Run.TenantID,
		&rec.Run.SchemaVersionID,
		&rec.Run.SemanticLayerID,
		&rec.Run.PromptContextSource,
		&rec.Run.ClerkUserID,
		&rec.Run.Question,
		&rec.Run.Status,
		&sqlOriginal,
		&sqlExecuted,
		&attemptsJSON,
		&warningsJSON,
		&rec.Run.RowCount,
		&rec.Run.ElapsedMS,
		&errorStage,
		&errorMessage,
		&rec.Run.CreatedAt,
		&rec.Run.CompletedAt,
		&latestFeedbackUserID,
		&latestFeedbackRating,
		&latestFeedbackComment,
		&latestFeedbackCorrected,
		&latestFeedbackCreatedAt,
		&latestFeedbackUpdatedAt,
		&rec.HasActiveCanonicalExample,
		&rec.ReviewedAt,
		&rec.ReviewSignalAt,
	); err != nil {
		return model.TenantQueryRunReviewItem{}, err
	}

	rec.Run.SQLOriginal = sqlOriginal.String
	rec.Run.SQLExecuted = sqlExecuted.String
	rec.Run.ErrorStage = errorStage.String
	rec.Run.ErrorMessage = errorMessage.String

	attempts, err := decodeQueryRunAttempts(attemptsJSON)
	if err != nil {
		return model.TenantQueryRunReviewItem{}, fmt.Errorf("decode query attempts: %w", err)
	}
	warnings, err := decodeQueryRunWarnings(warningsJSON)
	if err != nil {
		return model.TenantQueryRunReviewItem{}, fmt.Errorf("decode query warnings: %w", err)
	}
	rec.Run.Attempts = attempts
	rec.Run.Warnings = warnings

	if latestFeedbackRating.Valid {
		rec.HasFeedback = true
		rec.LatestFeedback = &model.TenantQueryFeedback{
			QueryRunID:   rec.Run.ID,
			ClerkUserID:  latestFeedbackUserID.String,
			Rating:       model.QueryFeedbackRating(latestFeedbackRating.String),
			Comment:      latestFeedbackComment.String,
			CorrectedSQL: latestFeedbackCorrected.String,
		}
		if latestFeedbackCreatedAt.Valid {
			rec.LatestFeedback.CreatedAt = latestFeedbackCreatedAt.Time
		}
		if latestFeedbackUpdatedAt.Valid {
			rec.LatestFeedback.UpdatedAt = latestFeedbackUpdatedAt.Time
		}
	}

	return rec, nil
}

type TenantQueryFeedbackRepository struct {
	db queryMemoryDB
}

func NewTenantQueryFeedbackRepository(
	pool *pgxpool.Pool,
) *TenantQueryFeedbackRepository {
	return &TenantQueryFeedbackRepository{db: pool}
}

func (r *TenantQueryFeedbackRepository) Upsert(
	ctx context.Context,
	queryRunID uuid.UUID,
	clerkUserID string,
	rating model.QueryFeedbackRating,
	comment, correctedSQL string,
	now time.Time,
) (model.TenantQueryFeedback, error) {
	var rec model.TenantQueryFeedback
	err := r.db.QueryRow(ctx, `
		INSERT INTO tenant_query_feedback (
			query_run_id,
			clerk_user_id,
			rating,
			comment,
			corrected_sql,
			created_at,
			updated_at
		) VALUES ($1, $2, $3::query_feedback_rating, $4, $5, $6, $6)
		ON CONFLICT (query_run_id, clerk_user_id) DO UPDATE
		SET
			rating = EXCLUDED.rating,
			comment = EXCLUDED.comment,
			corrected_sql = EXCLUDED.corrected_sql,
			updated_at = EXCLUDED.updated_at
		RETURNING
			query_run_id,
			clerk_user_id,
			rating::text,
			comment,
			corrected_sql,
			created_at,
			updated_at
	`, queryRunID, clerkUserID, string(rating), strings.TrimSpace(comment), strings.TrimSpace(correctedSQL), now.UTC()).Scan(
		&rec.QueryRunID,
		&rec.ClerkUserID,
		&rec.Rating,
		&rec.Comment,
		&rec.CorrectedSQL,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return model.TenantQueryFeedback{}, fmt.Errorf("upsert tenant query feedback: %w", err)
	}
	return rec, nil
}

type TenantCanonicalQueryExampleRepository struct {
	db queryMemoryDB
}

func NewTenantCanonicalQueryExampleRepository(
	pool *pgxpool.Pool,
) *TenantCanonicalQueryExampleRepository {
	return &TenantCanonicalQueryExampleRepository{db: pool}
}

func (r *TenantCanonicalQueryExampleRepository) Create(
	ctx context.Context,
	tenantID, schemaVersionID, sourceQueryRunID uuid.UUID,
	createdByUserID, question, sql, notes string,
) (model.TenantCanonicalQueryExample, error) {
	var rec model.TenantCanonicalQueryExample
	err := r.db.QueryRow(ctx, `
		INSERT INTO tenant_canonical_query_examples (
			tenant_id,
			schema_version_id,
			source_query_run_id,
			created_by_user_id,
			question,
			sql,
			notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			tenant_id,
			schema_version_id,
			source_query_run_id,
			created_by_user_id,
			question,
			sql,
			notes,
			archived_at,
			created_at
	`, tenantID, schemaVersionID, sourceQueryRunID, createdByUserID, strings.TrimSpace(question), strings.TrimSpace(sql), strings.TrimSpace(notes)).Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.SchemaVersionID,
		&rec.SourceQueryRunID,
		&rec.CreatedByUserID,
		&rec.Question,
		&rec.SQL,
		&rec.Notes,
		&rec.ArchivedAt,
		&rec.CreatedAt,
	)
	if err != nil {
		return model.TenantCanonicalQueryExample{}, fmt.Errorf("insert canonical query example: %w", err)
	}
	return rec, nil
}

func (r *TenantCanonicalQueryExampleRepository) ListActiveByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
	limit int,
) ([]model.TenantCanonicalQueryExample, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			tenant_id,
			schema_version_id,
			source_query_run_id,
			created_by_user_id,
			question,
			sql,
			notes,
			archived_at,
			created_at
		FROM tenant_canonical_query_examples
		WHERE tenant_id = $1
		  AND archived_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list canonical query examples: %w", err)
	}
	defer rows.Close()

	var out []model.TenantCanonicalQueryExample
	for rows.Next() {
		var rec model.TenantCanonicalQueryExample
		if err := rows.Scan(
			&rec.ID,
			&rec.TenantID,
			&rec.SchemaVersionID,
			&rec.SourceQueryRunID,
			&rec.CreatedByUserID,
			&rec.Question,
			&rec.SQL,
			&rec.Notes,
			&rec.ArchivedAt,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan canonical query example: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical query examples: %w", err)
	}
	return out, nil
}

func (r *TenantCanonicalQueryExampleRepository) SearchActiveByQuestion(
	ctx context.Context,
	tenantID uuid.UUID,
	question string,
	limit int,
	schemaVersionID *uuid.UUID,
) ([]model.TenantCanonicalQueryExample, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if schemaVersionID != nil {
		rows, err = r.db.Query(ctx, `
			SELECT
				id,
				tenant_id,
				schema_version_id,
				source_query_run_id,
				created_by_user_id,
				question,
				sql,
				notes,
				archived_at,
				created_at
			FROM tenant_canonical_query_examples
			WHERE tenant_id = $1
			  AND schema_version_id = $2
			  AND archived_at IS NULL
			ORDER BY
				GREATEST(
					similarity(question, $3),
					similarity(notes, $3)
				) DESC,
				created_at DESC
			LIMIT $4
		`, tenantID, *schemaVersionID, question, limit)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT
				id,
				tenant_id,
				schema_version_id,
				source_query_run_id,
				created_by_user_id,
				question,
				sql,
				notes,
				archived_at,
				created_at
			FROM tenant_canonical_query_examples
			WHERE tenant_id = $1
			  AND archived_at IS NULL
			ORDER BY
				GREATEST(
					similarity(question, $2),
					similarity(notes, $2)
				) DESC,
				created_at DESC
			LIMIT $3
		`, tenantID, question, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("search canonical query examples: %w", err)
	}
	defer rows.Close()

	var out []model.TenantCanonicalQueryExample
	for rows.Next() {
		var rec model.TenantCanonicalQueryExample
		if err := rows.Scan(
			&rec.ID,
			&rec.TenantID,
			&rec.SchemaVersionID,
			&rec.SourceQueryRunID,
			&rec.CreatedByUserID,
			&rec.Question,
			&rec.SQL,
			&rec.Notes,
			&rec.ArchivedAt,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan canonical query example search result: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical query example search result: %w", err)
	}
	return out, nil
}

func (r *TenantCanonicalQueryExampleRepository) Archive(
	ctx context.Context,
	tenantID, id uuid.UUID,
	archivedAt time.Time,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE tenant_canonical_query_examples
		SET archived_at = $3
		WHERE tenant_id = $1
		  AND id = $2
		  AND archived_at IS NULL
	`, tenantID, id, archivedAt.UTC())
	if err != nil {
		return fmt.Errorf("archive canonical query example: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
