package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

var ErrAgentSchemaIntrospectionFailed = errors.New(
	"agent schema introspection failed",
)

type SchemaControllerConfig struct {
	Now func() time.Time
}

type schemaVersionStore interface {
	LatestByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
	) (model.TenantSchemaVersion, error)
	Create(
		ctx context.Context,
		tenantID uuid.UUID,
		capturedAt time.Time,
		schemaHash string,
		blob []byte,
	) (model.TenantSchemaVersion, error)
}

type schemaSessionManager interface {
	IntrospectSchema(
		ctx context.Context,
		tenantID uuid.UUID,
	) (AgentIntrospectSchemaResult, error)
}

type SchemaCaptureResult struct {
	VersionID       uuid.UUID
	Changed         bool
	CapturedAt      time.Time
	SchemaHash      string
	DatabaseName    string
	TableCount      int
	ColumnCount     int
	ForeignKeyCount int
}

type SchemaController struct {
	schemas  schemaVersionStore
	sessions schemaSessionManager
	now      func() time.Time
}

func NewSchemaController(
	schemas schemaVersionStore,
	sessions schemaSessionManager,
	cfg SchemaControllerConfig,
) *SchemaController {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &SchemaController{
		schemas:  schemas,
		sessions: sessions,
		now:      now,
	}
}

func (c *SchemaController) Capture(
	ctx context.Context,
	tenantID uuid.UUID,
) (SchemaCaptureResult, error) {
	result, err := c.sessions.IntrospectSchema(ctx, tenantID)
	if err != nil {
		return SchemaCaptureResult{}, err
	}
	if result.Error != "" {
		return SchemaCaptureResult{}, fmt.Errorf(
			"%w: %s",
			ErrAgentSchemaIntrospectionFailed,
			result.Error,
		)
	}

	normalized := normalizeSchemaBlob(result.Schema)
	if normalized.DatabaseName == "" {
		normalized.DatabaseName = result.DatabaseName
	}

	blob, err := json.Marshal(normalized)
	if err != nil {
		return SchemaCaptureResult{}, fmt.Errorf("marshal schema blob: %w", err)
	}
	hash := hashSchemaBlob(blob)

	tableCount := len(normalized.Tables)
	columnCount := len(normalized.Columns)
	foreignKeyCount := len(normalized.ForeignKeys)
	databaseName := normalized.DatabaseName

	latest, err := c.schemas.LatestByTenant(ctx, tenantID)
	switch {
	case err == nil && latest.SchemaHash == hash:
		return SchemaCaptureResult{
			VersionID:       latest.ID,
			Changed:         false,
			CapturedAt:      latest.CapturedAt,
			SchemaHash:      latest.SchemaHash,
			DatabaseName:    databaseName,
			TableCount:      tableCount,
			ColumnCount:     columnCount,
			ForeignKeyCount: foreignKeyCount,
		}, nil
	case err != nil && !errors.Is(err, repository.ErrNotFound):
		return SchemaCaptureResult{}, err
	}

	capturedAt := result.CompletedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = c.now().UTC()
	}

	created, err := c.schemas.Create(ctx, tenantID, capturedAt, hash, blob)
	if err != nil {
		return SchemaCaptureResult{}, err
	}

	return SchemaCaptureResult{
		VersionID:       created.ID,
		Changed:         true,
		CapturedAt:      created.CapturedAt,
		SchemaHash:      created.SchemaHash,
		DatabaseName:    databaseName,
		TableCount:      tableCount,
		ColumnCount:     columnCount,
		ForeignKeyCount: foreignKeyCount,
	}, nil
}

func hashSchemaBlob(blob []byte) string {
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:])
}

func normalizeSchemaBlob(blob model.SchemaBlob) model.SchemaBlob {
	out := cloneSchemaBlob(blob)
	sort.Slice(out.Tables, func(i, j int) bool {
		left := out.Tables[i]
		right := out.Tables[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		return left.TableName < right.TableName
	})
	sort.Slice(out.Columns, func(i, j int) bool {
		left := out.Columns[i]
		right := out.Columns[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		if left.TableName != right.TableName {
			return left.TableName < right.TableName
		}
		return left.OrdinalPosition < right.OrdinalPosition
	})
	sort.Slice(out.PrimaryKeys, func(i, j int) bool {
		left := out.PrimaryKeys[i]
		right := out.PrimaryKeys[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		if left.TableName != right.TableName {
			return left.TableName < right.TableName
		}
		if left.ConstraintName != right.ConstraintName {
			return left.ConstraintName < right.ConstraintName
		}
		return left.OrdinalPosition < right.OrdinalPosition
	})
	sort.Slice(out.ForeignKeys, func(i, j int) bool {
		left := out.ForeignKeys[i]
		right := out.ForeignKeys[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		if left.TableName != right.TableName {
			return left.TableName < right.TableName
		}
		if left.ConstraintName != right.ConstraintName {
			return left.ConstraintName < right.ConstraintName
		}
		return left.OrdinalPosition < right.OrdinalPosition
	})
	return out
}
