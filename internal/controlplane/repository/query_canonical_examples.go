package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

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
	`,
		tenantID,
		schemaVersionID,
		sourceQueryRunID,
		createdByUserID,
		strings.TrimSpace(question),
		strings.TrimSpace(sql),
		strings.TrimSpace(notes),
	).Scan(
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
		return model.TenantCanonicalQueryExample{}, fmt.Errorf(
			"insert canonical query example: %w",
			err,
		)
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
			return nil, fmt.Errorf(
				"scan canonical query example search result: %w",
				err,
			)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate canonical query example search result: %w",
			err,
		)
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
