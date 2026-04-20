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

type semanticLayerDB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type semanticLayerQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TenantSemanticLayerRepository struct {
	db semanticLayerDB
}

func NewTenantSemanticLayerRepository(
	pool *pgxpool.Pool,
) *TenantSemanticLayerRepository {
	return &TenantSemanticLayerRepository{db: pool}
}

func (r *TenantSemanticLayerRepository) LatestDraftBySchemaVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return r.selectOne(ctx, r.db, `
		SELECT
			id,
			tenant_id,
			schema_version_id,
			status::text,
			content,
			created_at,
			approved_at,
			approved_by_user_id
		FROM tenant_semantic_layers
		WHERE tenant_id = $1
		  AND schema_version_id = $2
		  AND status = 'draft'
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, schemaVersionID)
}

func (r *TenantSemanticLayerRepository) LatestApprovedBySchemaVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return r.selectOne(ctx, r.db, `
		SELECT
			id,
			tenant_id,
			schema_version_id,
			status::text,
			content,
			created_at,
			approved_at,
			approved_by_user_id
		FROM tenant_semantic_layers
		WHERE tenant_id = $1
		  AND schema_version_id = $2
		  AND status = 'approved'
		ORDER BY approved_at DESC NULLS LAST, created_at DESC
		LIMIT 1
	`, tenantID, schemaVersionID)
}

func (r *TenantSemanticLayerRepository) ListApprovedHistoryByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.TenantSemanticLayer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			tenant_id,
			schema_version_id,
			status::text,
			content,
			created_at,
			approved_at,
			approved_by_user_id
		FROM tenant_semantic_layers
		WHERE tenant_id = $1
		  AND approved_at IS NOT NULL
		ORDER BY approved_at DESC, created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list semantic layer history: %w", err)
	}
	defer rows.Close()

	var out []model.TenantSemanticLayer
	for rows.Next() {
		var rec model.TenantSemanticLayer
		if err := rows.Scan(
			&rec.ID,
			&rec.TenantID,
			&rec.SchemaVersionID,
			&rec.Status,
			&rec.Content,
			&rec.CreatedAt,
			&rec.ApprovedAt,
			&rec.ApprovedByUserID,
		); err != nil {
			return nil, fmt.Errorf("scan semantic layer history row: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate semantic layer history: %w", err)
	}
	return out, nil
}

func (r *TenantSemanticLayerRepository) GetByID(
	ctx context.Context,
	tenantID, id uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return r.selectOne(ctx, r.db, `
		SELECT
			id,
			tenant_id,
			schema_version_id,
			status::text,
			content,
			created_at,
			approved_at,
			approved_by_user_id
		FROM tenant_semantic_layers
		WHERE tenant_id = $1
		  AND id = $2
	`, tenantID, id)
}

func (r *TenantSemanticLayerRepository) CreateDraftVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
	content []byte,
) (model.TenantSemanticLayer, error) {
	var created model.TenantSemanticLayer
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE tenant_semantic_layers
			SET status = 'archived'
			WHERE tenant_id = $1
			  AND schema_version_id = $2
			  AND status = 'draft'
		`, tenantID, schemaVersionID); err != nil {
			return fmt.Errorf("archive prior drafts: %w", err)
		}

		rec, err := r.insert(
			ctx,
			tx,
			tenantID,
			schemaVersionID,
			model.SemanticLayerStatusDraft,
			content,
			nil,
			nil,
		)
		if err != nil {
			return err
		}
		created = rec
		return nil
	})
	if err != nil {
		return model.TenantSemanticLayer{}, err
	}
	return created, nil
}

func (r *TenantSemanticLayerRepository) Approve(
	ctx context.Context,
	tenantID, id uuid.UUID,
	approvedAt time.Time,
	approvedByUserID string,
) (model.TenantSemanticLayer, error) {
	var approved model.TenantSemanticLayer
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		target, err := r.selectOne(ctx, tx, `
			SELECT
				id,
				tenant_id,
				schema_version_id,
				status::text,
				content,
				created_at,
				approved_at,
				approved_by_user_id
			FROM tenant_semantic_layers
			WHERE tenant_id = $1
			  AND id = $2
			FOR UPDATE
		`, tenantID, id)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE tenant_semantic_layers
			SET status = 'archived'
			WHERE tenant_id = $1
			  AND schema_version_id = $2
			  AND id <> $3
			  AND status IN ('draft', 'approved')
		`, tenantID, target.SchemaVersionID, target.ID); err != nil {
			return fmt.Errorf("archive semantic layer peers: %w", err)
		}

		targetApprovedBy := approvedByUserID
		rec, err := r.updateApproval(
			ctx,
			tx,
			target.ID,
			approvedAt.UTC(),
			&targetApprovedBy,
		)
		if err != nil {
			return err
		}
		approved = rec
		return nil
	})
	if err != nil {
		return model.TenantSemanticLayer{}, err
	}
	return approved, nil
}

func (r *TenantSemanticLayerRepository) insert(
	ctx context.Context,
	db semanticLayerQueryer,
	tenantID, schemaVersionID uuid.UUID,
	status model.SemanticLayerStatus,
	content []byte,
	approvedAt *time.Time,
	approvedByUserID *string,
) (model.TenantSemanticLayer, error) {
	var rec model.TenantSemanticLayer
	err := db.QueryRow(ctx, `
		INSERT INTO tenant_semantic_layers (
			tenant_id,
			schema_version_id,
			status,
			content,
			approved_at,
			approved_by_user_id
		) VALUES ($1, $2, $3::semantic_layer_status, $4::jsonb, $5, $6)
		RETURNING
			id,
			tenant_id,
			schema_version_id,
			status::text,
			content,
			created_at,
			approved_at,
			approved_by_user_id
	`, tenantID, schemaVersionID, string(status), content, approvedAt, approvedByUserID).Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.SchemaVersionID,
		&rec.Status,
		&rec.Content,
		&rec.CreatedAt,
		&rec.ApprovedAt,
		&rec.ApprovedByUserID,
	)
	if err != nil {
		return model.TenantSemanticLayer{}, fmt.Errorf("insert semantic layer: %w", err)
	}
	return rec, nil
}

func (r *TenantSemanticLayerRepository) updateApproval(
	ctx context.Context,
	db semanticLayerQueryer,
	id uuid.UUID,
	approvedAt time.Time,
	approvedByUserID *string,
) (model.TenantSemanticLayer, error) {
	var rec model.TenantSemanticLayer
	err := db.QueryRow(ctx, `
		UPDATE tenant_semantic_layers
		SET
			status = 'approved',
			approved_at = $2,
			approved_by_user_id = $3
		WHERE id = $1
		RETURNING
			id,
			tenant_id,
			schema_version_id,
			status::text,
			content,
			created_at,
			approved_at,
			approved_by_user_id
	`, id, approvedAt, approvedByUserID).Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.SchemaVersionID,
		&rec.Status,
		&rec.Content,
		&rec.CreatedAt,
		&rec.ApprovedAt,
		&rec.ApprovedByUserID,
	)
	if err != nil {
		return model.TenantSemanticLayer{}, fmt.Errorf("approve semantic layer: %w", err)
	}
	return rec, nil
}

func (r *TenantSemanticLayerRepository) selectOne(
	ctx context.Context,
	db interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	},
	sql string,
	args ...any,
) (model.TenantSemanticLayer, error) {
	var rec model.TenantSemanticLayer
	err := db.QueryRow(ctx, sql, args...).Scan(
		&rec.ID,
		&rec.TenantID,
		&rec.SchemaVersionID,
		&rec.Status,
		&rec.Content,
		&rec.CreatedAt,
		&rec.ApprovedAt,
		&rec.ApprovedByUserID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.TenantSemanticLayer{}, ErrNotFound
	}
	if err != nil {
		return model.TenantSemanticLayer{}, fmt.Errorf("select semantic layer: %w", err)
	}
	return rec, nil
}
