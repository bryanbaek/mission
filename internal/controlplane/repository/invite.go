package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type inviteDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type InviteRepository struct {
	db inviteDB
}

func NewInviteRepository(pool *pgxpool.Pool) *InviteRepository {
	return &InviteRepository{db: pool}
}

func (r *InviteRepository) ListByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.TenantInvite, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, email, created_by_user_id, created_at
		FROM tenant_invites
		WHERE tenant_id = $1
		ORDER BY created_at DESC, email ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant invites: %w", err)
	}
	defer rows.Close()

	out := make([]model.TenantInvite, 0)
	for rows.Next() {
		var rec model.TenantInvite
		if err := rows.Scan(
			&rec.ID,
			&rec.TenantID,
			&rec.Email,
			&rec.CreatedByUserID,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant invite: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant invites: %w", err)
	}
	return out, nil
}

func (r *InviteRepository) CreateMany(
	ctx context.Context,
	tenantID uuid.UUID,
	emails []string,
	createdByUserID string,
) ([]model.TenantInvite, error) {
	out := make([]model.TenantInvite, 0, len(emails))
	for _, email := range emails {
		normalized := strings.TrimSpace(strings.ToLower(email))
		if normalized == "" {
			continue
		}

		var rec model.TenantInvite
		err := r.db.QueryRow(ctx, `
			INSERT INTO tenant_invites (
				tenant_id,
				email,
				created_by_user_id
			) VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, lower(email)) DO UPDATE
			SET created_by_user_id = tenant_invites.created_by_user_id
			RETURNING id, tenant_id, email, created_by_user_id, created_at
		`, tenantID, normalized, createdByUserID).Scan(
			&rec.ID,
			&rec.TenantID,
			&rec.Email,
			&rec.CreatedByUserID,
			&rec.CreatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("insert tenant invite: %w", err)
		}
		out = append(out, rec)
	}
	return out, nil
}
