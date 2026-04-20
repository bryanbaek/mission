package controller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeSchemaVersionStore struct {
	latestFn func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error)
	createFn func(context.Context, uuid.UUID, time.Time, string, []byte) (model.TenantSchemaVersion, error)
}

func (f fakeSchemaVersionStore) LatestByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantSchemaVersion, error) {
	return f.latestFn(ctx, tenantID)
}

func (f fakeSchemaVersionStore) Create(
	ctx context.Context,
	tenantID uuid.UUID,
	capturedAt time.Time,
	schemaHash string,
	blob []byte,
) (model.TenantSchemaVersion, error) {
	return f.createFn(ctx, tenantID, capturedAt, schemaHash, blob)
}

type fakeSchemaSessionManager struct {
	introspectFn func(context.Context, uuid.UUID) (AgentIntrospectSchemaResult, error)
}

func (f fakeSchemaSessionManager) IntrospectSchema(
	ctx context.Context,
	tenantID uuid.UUID,
) (AgentIntrospectSchemaResult, error) {
	return f.introspectFn(ctx, tenantID)
}

func TestSchemaControllerCaptureCreatesVersion(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	versionID := uuid.New()
	capturedAt := time.Unix(1_700_000_000, 0).UTC()

	ctrl := NewSchemaController(
		fakeSchemaVersionStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, repository.ErrNotFound
			},
			createFn: func(_ context.Context, gotTenantID uuid.UUID, gotCapturedAt time.Time, gotHash string, gotBlob []byte) (model.TenantSchemaVersion, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				if gotCapturedAt != capturedAt {
					t.Fatalf("capturedAt = %s, want %s", gotCapturedAt, capturedAt)
				}
				var blob model.SchemaBlob
				if err := json.Unmarshal(gotBlob, &blob); err != nil {
					t.Fatalf("json.Unmarshal returned error: %v", err)
				}
				if len(blob.Tables) != 1 {
					t.Fatalf("table count = %d, want 1", len(blob.Tables))
				}
				return model.TenantSchemaVersion{
					ID:         versionID,
					TenantID:   gotTenantID,
					CapturedAt: gotCapturedAt,
					SchemaHash: gotHash,
				}, nil
			},
		},
		fakeSchemaSessionManager{
			introspectFn: func(context.Context, uuid.UUID) (AgentIntrospectSchemaResult, error) {
				return AgentIntrospectSchemaResult{
					CompletedAt:  capturedAt,
					DatabaseName: "mission_app",
					Schema: model.SchemaBlob{
						DatabaseName: "mission_app",
						Tables: []model.SchemaTable{{
							TableSchema: "mission_app",
							TableName:   "customers",
							TableType:   "BASE TABLE",
						}},
						Columns: []model.SchemaColumn{{
							TableSchema:     "mission_app",
							TableName:       "customers",
							ColumnName:      "id",
							OrdinalPosition: 1,
						}},
					},
				}, nil
			},
		},
		SchemaControllerConfig{},
	)

	got, err := ctrl.Capture(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if !got.Changed {
		t.Fatal("Changed = false, want true")
	}
	if got.VersionID != versionID {
		t.Fatalf("VersionID = %s, want %s", got.VersionID, versionID)
	}
}

func TestSchemaControllerCaptureDedupesLatestHash(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	capturedAt := time.Unix(1_700_000_000, 0).UTC()
	versionID := uuid.New()

	var latest model.TenantSchemaVersion
	ctrl := NewSchemaController(
		fakeSchemaVersionStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return latest, nil
			},
			createFn: func(context.Context, uuid.UUID, time.Time, string, []byte) (model.TenantSchemaVersion, error) {
				t.Fatal("Create should not be called when hash is unchanged")
				return model.TenantSchemaVersion{}, nil
			},
		},
		fakeSchemaSessionManager{
			introspectFn: func(context.Context, uuid.UUID) (AgentIntrospectSchemaResult, error) {
				return AgentIntrospectSchemaResult{
					CompletedAt: capturedAt.Add(time.Minute),
					Schema: model.SchemaBlob{
						DatabaseName: "mission_app",
						Tables: []model.SchemaTable{{
							TableSchema: "mission_app",
							TableName:   "customers",
							TableType:   "BASE TABLE",
						}},
					},
				}, nil
			},
		},
		SchemaControllerConfig{},
	)

	normalized := normalizeSchemaBlob(model.SchemaBlob{
		DatabaseName: "mission_app",
		Tables: []model.SchemaTable{{
			TableSchema: "mission_app",
			TableName:   "customers",
			TableType:   "BASE TABLE",
		}},
	})
	blob, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	latest = model.TenantSchemaVersion{
		ID:         versionID,
		TenantID:   tenantID,
		CapturedAt: capturedAt,
		SchemaHash: hashSchemaBlob(blob),
	}

	got, err := ctrl.Capture(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if got.Changed {
		t.Fatal("Changed = true, want false")
	}
	if got.VersionID != versionID {
		t.Fatalf("VersionID = %s, want %s", got.VersionID, versionID)
	}
	if got.CapturedAt != capturedAt {
		t.Fatalf("CapturedAt = %s, want %s", got.CapturedAt, capturedAt)
	}
}

func TestSchemaControllerCapturePropagatesErrors(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	expectedErr := errors.New("store failed")
	ctrl := NewSchemaController(
		fakeSchemaVersionStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, expectedErr
			},
			createFn: func(context.Context, uuid.UUID, time.Time, string, []byte) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, nil
			},
		},
		fakeSchemaSessionManager{
			introspectFn: func(context.Context, uuid.UUID) (AgentIntrospectSchemaResult, error) {
				return AgentIntrospectSchemaResult{
					Schema: model.SchemaBlob{DatabaseName: "mission_app"},
				}, nil
			},
		},
		SchemaControllerConfig{},
	)

	_, err := ctrl.Capture(context.Background(), tenantID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("err = %v, want %v", err, expectedErr)
	}

	ctrl = NewSchemaController(
		fakeSchemaVersionStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, repository.ErrNotFound
			},
			createFn: func(context.Context, uuid.UUID, time.Time, string, []byte) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, nil
			},
		},
		fakeSchemaSessionManager{
			introspectFn: func(context.Context, uuid.UUID) (AgentIntrospectSchemaResult, error) {
				return AgentIntrospectSchemaResult{
					Error: "permission denied",
				}, nil
			},
		},
		SchemaControllerConfig{},
	)

	_, err = ctrl.Capture(context.Background(), tenantID)
	if !errors.Is(err, ErrAgentSchemaIntrospectionFailed) {
		t.Fatalf("err = %v, want ErrAgentSchemaIntrospectionFailed", err)
	}
}
