package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

var ErrNotFound = errors.New("not found")

type tenantDB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type TenantRepository struct {
	db tenantDB
}

func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{db: pool}
}

// CreateWithOwner creates a tenant and inserts the caller as its first owner
// in a single transaction.
func (r *TenantRepository) CreateWithOwner(ctx context.Context, slug, name, ownerClerkID string) (model.Tenant, error) {
	var t model.Tenant
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`INSERT INTO tenants (slug, name) VALUES ($1, $2)
			 RETURNING id, slug, name, created_at`,
			slug, name)
		if err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt); err != nil {
			return fmt.Errorf("insert tenant: %w", err)
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO tenant_users (tenant_id, clerk_user_id, role)
			 VALUES ($1, $2, $3)`,
			t.ID, ownerClerkID, string(model.RoleOwner))
		if err != nil {
			return fmt.Errorf("insert owner: %w", err)
		}
		return nil
	})
	return t, err
}

// ListForUser returns every tenant the given Clerk user is a member of.
func (r *TenantRepository) ListForUser(ctx context.Context, clerkUserID string) ([]model.Tenant, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id, t.slug, t.name, t.created_at
		 FROM tenants t
		 JOIN tenant_users tu ON tu.tenant_id = t.id
		 WHERE tu.clerk_user_id = $1
		 ORDER BY t.created_at`,
		clerkUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetMembership returns the user's role in the tenant, or ErrNotFound if they
// aren't a member. This is the authority for tenant-scoped access checks.
func (r *TenantRepository) GetMembership(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (model.TenantUser, error) {
	var tu model.TenantUser
	var role string
	err := r.db.QueryRow(ctx,
		`SELECT tenant_id, clerk_user_id, role, created_at
		 FROM tenant_users WHERE tenant_id = $1 AND clerk_user_id = $2`,
		tenantID, clerkUserID).
		Scan(&tu.TenantID, &tu.ClerkUserID, &role, &tu.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantUser{}, ErrNotFound
	}
	if err != nil {
		return model.TenantUser{}, err
	}
	tu.Role = model.Role(role)
	return tu, nil
}
