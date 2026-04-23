package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

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
	`,
		queryRunID,
		clerkUserID,
		string(rating),
		strings.TrimSpace(comment),
		strings.TrimSpace(correctedSQL),
		now.UTC(),
	).Scan(
		&rec.QueryRunID,
		&rec.ClerkUserID,
		&rec.Rating,
		&rec.Comment,
		&rec.CorrectedSQL,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return model.TenantQueryFeedback{}, fmt.Errorf(
			"upsert tenant query feedback: %w",
			err,
		)
	}
	return rec, nil
}
