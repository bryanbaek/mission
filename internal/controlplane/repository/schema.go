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

type tenantSchemaDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type TenantSchemaRepository struct {
	db tenantSchemaDB
}

func NewTenantSchemaRepository(pool *pgxpool.Pool) *TenantSchemaRepository {
	return &TenantSchemaRepository{db: pool}
}

func (r *TenantSchemaRepository) LatestByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantSchemaVersion, error) {
	var rec model.TenantSchemaVersion
	err := r.db.QueryRow(ctx, `
		SELECT
			id,
			tenant_id,
			captured_at,
			schema_hash,
			blob,
			created_at
		FROM tenant_schemas
		WHERE tenant_id = $1
		ORDER BY captured_at DESC, created_at DESC
		LIMIT 1
	`, tenantID).Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.CapturedAt,
		&rec.SchemaHash,
		&rec.Blob,
		&rec.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantSchemaVersion{}, ErrNotFound
	}
	if err != nil {
		return model.TenantSchemaVersion{}, fmt.Errorf("select latest tenant schema: %w", err)
	}
	return rec, nil
}

func (r *TenantSchemaRepository) Create(
	ctx context.Context,
	tenantID uuid.UUID,
	capturedAt time.Time,
	schemaHash string,
	blob []byte,
) (model.TenantSchemaVersion, error) {
	var rec model.TenantSchemaVersion
	err := r.db.QueryRow(ctx, `
		INSERT INTO tenant_schemas (
			tenant_id,
			captured_at,
			schema_hash,
			blob
		) VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, tenant_id, captured_at, schema_hash, blob, created_at
	`, tenantID, capturedAt.UTC(), schemaHash, blob).Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.CapturedAt,
		&rec.SchemaHash,
		&rec.Blob,
		&rec.CreatedAt,
	)
	if err != nil {
		return model.TenantSchemaVersion{}, fmt.Errorf("insert tenant schema: %w", err)
	}
	return rec, nil
}
