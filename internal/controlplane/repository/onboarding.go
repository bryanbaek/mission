package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type onboardingDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type OnboardingRepository struct {
	db onboardingDB
}

func NewOnboardingRepository(pool *pgxpool.Pool) *OnboardingRepository {
	return &OnboardingRepository{db: pool}
}

func (r *OnboardingRepository) GetByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantOnboardingState, error) {
	var rec model.TenantOnboardingState
	err := r.db.QueryRow(ctx, `
		SELECT
			tenant_id,
			current_step,
			payload,
			completed_at,
			created_at,
			updated_at
		FROM tenant_onboarding_state
		WHERE tenant_id = $1
	`, tenantID).Scan(
		&rec.TenantID,
		&rec.CurrentStep,
		&rec.Payload,
		&rec.CompletedAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantOnboardingState{}, ErrNotFound
	}
	if err != nil {
		return model.TenantOnboardingState{}, fmt.Errorf("select onboarding state: %w", err)
	}
	return rec, nil
}

func (r *OnboardingRepository) Upsert(
	ctx context.Context,
	tenantID uuid.UUID,
	currentStep int32,
	payload []byte,
	completedAt *time.Time,
) (model.TenantOnboardingState, error) {
	var rec model.TenantOnboardingState
	err := r.db.QueryRow(ctx, `
		INSERT INTO tenant_onboarding_state (
			tenant_id,
			current_step,
			payload,
			completed_at,
			updated_at
		) VALUES ($1, $2, $3::jsonb, $4, NOW())
		ON CONFLICT (tenant_id) DO UPDATE
		SET
			current_step = EXCLUDED.current_step,
			payload = EXCLUDED.payload,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		RETURNING tenant_id, current_step, payload, completed_at, created_at, updated_at
	`, tenantID, currentStep, payload, completedAt).Scan(
		&rec.TenantID,
		&rec.CurrentStep,
		&rec.Payload,
		&rec.CompletedAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return model.TenantOnboardingState{}, fmt.Errorf("upsert onboarding state: %w", err)
	}
	return rec, nil
}

func (r *OnboardingRepository) ListWorkspacesForUser(
	ctx context.Context,
	clerkUserID string,
) ([]model.OnboardingWorkspace, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			t.id,
			t.slug,
			t.name,
			tu.role,
			COALESCE(os.completed_at IS NOT NULL, FALSE) AS onboarding_complete,
			COALESCE(os.current_step, 1) AS current_step,
			COALESCE(os.updated_at, t.created_at) AS updated_at
		FROM tenants t
		JOIN tenant_users tu ON tu.tenant_id = t.id
		LEFT JOIN tenant_onboarding_state os ON os.tenant_id = t.id
		WHERE tu.clerk_user_id = $1
		ORDER BY COALESCE(os.updated_at, t.created_at) DESC, t.created_at DESC
	`, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("list onboarding workspaces: %w", err)
	}
	defer rows.Close()

	out := make([]model.OnboardingWorkspace, 0)
	for rows.Next() {
		var rec model.OnboardingWorkspace
		var role string
		if err := rows.Scan(
			&rec.TenantID,
			&rec.Slug,
			&rec.Name,
			&role,
			&rec.OnboardingComplete,
			&rec.CurrentStep,
			&rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan onboarding workspace: %w", err)
		}
		rec.Role = model.WorkspaceRole(role)
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate onboarding workspaces: %w", err)
	}
	return out, nil
}
