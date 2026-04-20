package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type starterQuestionsDB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type starterQuestionsQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type StarterQuestionsRepository struct {
	db starterQuestionsDB
}

func NewStarterQuestionsRepository(
	pool *pgxpool.Pool,
) *StarterQuestionsRepository {
	return &StarterQuestionsRepository{db: pool}
}

func (r *StarterQuestionsRepository) InsertSet(
	ctx context.Context,
	tenantID, semanticLayerID, setID uuid.UUID,
	questions []model.StarterQuestion,
) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		return r.insertSet(ctx, tx, tenantID, semanticLayerID, setID, questions)
	})
}

func (r *StarterQuestionsRepository) DeactivatePriorSets(
	ctx context.Context,
	tenantID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tenant_starter_questions
		SET is_active = FALSE
		WHERE tenant_id = $1
		  AND is_active = TRUE
	`, tenantID)
	if err != nil {
		return fmt.Errorf("deactivate starter question sets: %w", err)
	}
	return nil
}

func (r *StarterQuestionsRepository) ReplaceActiveSet(
	ctx context.Context,
	tenantID, semanticLayerID, setID uuid.UUID,
	questions []model.StarterQuestion,
) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := r.deactivatePriorSets(ctx, tx, tenantID); err != nil {
			return err
		}
		return r.insertSet(ctx, tx, tenantID, semanticLayerID, setID, questions)
	})
}

func (r *StarterQuestionsRepository) LatestActive(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
	rows, err := r.db.Query(ctx, `
		WITH latest_set AS (
			SELECT set_id
			FROM tenant_starter_questions
			WHERE tenant_id = $1
			  AND is_active = TRUE
			ORDER BY created_at DESC, ordinal ASC
			LIMIT 1
		)
		SELECT
			id,
			set_id,
			tenant_id,
			semantic_layer_id,
			ordinal,
			text,
			category,
			primary_table,
			created_at,
			is_active
		FROM tenant_starter_questions
		WHERE tenant_id = $1
		  AND is_active = TRUE
		  AND set_id = (SELECT set_id FROM latest_set)
		ORDER BY ordinal ASC
	`, tenantID)
	if err != nil {
		return nil, uuid.Nil, time.Time{}, fmt.Errorf("select starter questions: %w", err)
	}
	defer rows.Close()

	questions := make([]model.StarterQuestion, 0, 10)
	var (
		setID       uuid.UUID
		generatedAt time.Time
	)
	for rows.Next() {
		var question model.StarterQuestion
		if err := rows.Scan(
			&question.ID,
			&question.SetID,
			&question.TenantID,
			&question.SemanticLayerID,
			&question.Ordinal,
			&question.Text,
			&question.Category,
			&question.PrimaryTable,
			&question.CreatedAt,
			&question.IsActive,
		); err != nil {
			return nil, uuid.Nil, time.Time{}, fmt.Errorf("scan starter question row: %w", err)
		}
		if generatedAt.IsZero() {
			setID = question.SetID
			generatedAt = question.CreatedAt
		}
		questions = append(questions, question)
	}
	if err := rows.Err(); err != nil {
		return nil, uuid.Nil, time.Time{}, fmt.Errorf("iterate starter question rows: %w", err)
	}
	if len(questions) == 0 {
		return nil, uuid.Nil, time.Time{}, ErrNotFound
	}
	return questions, setID, generatedAt, nil
}

func (r *StarterQuestionsRepository) deactivatePriorSets(
	ctx context.Context,
	db starterQuestionsQueryer,
	tenantID uuid.UUID,
) error {
	_, err := db.Exec(ctx, `
		UPDATE tenant_starter_questions
		SET is_active = FALSE
		WHERE tenant_id = $1
		  AND is_active = TRUE
	`, tenantID)
	if err != nil {
		return fmt.Errorf("deactivate starter question sets: %w", err)
	}
	return nil
}

func (r *StarterQuestionsRepository) insertSet(
	ctx context.Context,
	db starterQuestionsQueryer,
	tenantID, semanticLayerID, setID uuid.UUID,
	questions []model.StarterQuestion,
) error {
	for _, question := range questions {
		if _, err := db.Exec(ctx, `
			INSERT INTO tenant_starter_questions (
				id,
				tenant_id,
				semantic_layer_id,
				set_id,
				ordinal,
				text,
				category,
				primary_table,
				is_active
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			question.ID,
			tenantID,
			semanticLayerID,
			setID,
			question.Ordinal,
			question.Text,
			string(question.Category),
			question.PrimaryTable,
			question.IsActive,
		); err != nil {
			return fmt.Errorf("insert starter question: %w", err)
		}
	}
	return nil
}

func scanStarterQuestion(rows pgx.Rows) (model.StarterQuestion, error) {
	var rec model.StarterQuestion
	err := rows.Scan(
		&rec.ID,
		&rec.SetID,
		&rec.TenantID,
		&rec.SemanticLayerID,
		&rec.Ordinal,
		&rec.Text,
		&rec.Category,
		&rec.PrimaryTable,
		&rec.CreatedAt,
		&rec.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.StarterQuestion{}, ErrNotFound
	}
	if err != nil {
		return model.StarterQuestion{}, fmt.Errorf("scan starter question: %w", err)
	}
	return rec, nil
}
