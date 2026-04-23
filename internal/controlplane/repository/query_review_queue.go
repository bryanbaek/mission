package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type queryRunReviewScanner interface {
	Scan(dest ...any) error
}

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
