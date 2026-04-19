package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type tenantTokenDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TenantTokenRepository struct {
	db tenantTokenDB
}

func NewTenantTokenRepository(pool *pgxpool.Pool) *TenantTokenRepository {
	return &TenantTokenRepository{db: pool}
}

// Create stores a new token row. The hash is the SHA-256 of the plaintext
// token; the plaintext is never persisted.
func (r *TenantTokenRepository) Create(ctx context.Context, tenantID uuid.UUID, label string, tokenHash []byte) (model.TenantToken, error) {
	var tt model.TenantToken
	err := r.db.QueryRow(ctx,
		`INSERT INTO tenant_tokens (tenant_id, label, token_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, tenant_id, label, created_at, last_used_at, revoked_at`,
		tenantID, label, tokenHash).
		Scan(&tt.ID, &tt.TenantID, &tt.Label, &tt.CreatedAt, &tt.LastUsedAt, &tt.RevokedAt)
	if err != nil {
		return model.TenantToken{}, fmt.Errorf("insert token: %w", err)
	}
	return tt, nil
}

func (r *TenantTokenRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.TenantToken, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, tenant_id, label, created_at, last_used_at, revoked_at
		 FROM tenant_tokens WHERE tenant_id = $1
		 ORDER BY created_at DESC`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.TenantToken
	for rows.Next() {
		var tt model.TenantToken
		if err := rows.Scan(&tt.ID, &tt.TenantID, &tt.Label, &tt.CreatedAt, &tt.LastUsedAt, &tt.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, tt)
	}
	return out, rows.Err()
}

// Revoke marks the token revoked. Returns ErrNotFound if it doesn't exist or
// doesn't belong to tenantID — never reveals which.
func (r *TenantTokenRepository) Revoke(ctx context.Context, tenantID, tokenID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE tenant_tokens SET revoked_at = NOW()
		 WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL`,
		tokenID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LookupActiveByHash returns the active (non-revoked) token row matching hash,
// or ErrNotFound. Used by edge-agent authentication later.
func (r *TenantTokenRepository) LookupActiveByHash(ctx context.Context, hash []byte) (model.TenantToken, error) {
	var tt model.TenantToken
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, label, created_at, last_used_at, revoked_at
		 FROM tenant_tokens WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash).
		Scan(&tt.ID, &tt.TenantID, &tt.Label, &tt.CreatedAt, &tt.LastUsedAt, &tt.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantToken{}, ErrNotFound
	}
	return tt, err
}
