package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeTenantSchemaDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f *fakeTenantSchemaDB) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return fakeRow{scanFn: func(dest ...any) error { return errors.New("unexpected QueryRow call") }}
}

func TestTenantSchemaRepositoryLatestByTenant(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	versionID := uuid.New()
	capturedAt := time.Unix(1_700_000_000, 0).UTC()
	createdAt := capturedAt.Add(time.Second)
	blob := json.RawMessage(`{"database_name":"mission_app"}`)

	repo := &TenantSchemaRepository{
		db: &fakeTenantSchemaDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "FROM tenant_schemas") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != tenantID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = versionID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*time.Time)) = capturedAt
					*(dest[3].(*string)) = "hash-123"
					*(dest[4].(*json.RawMessage)) = blob
					*(dest[5].(*time.Time)) = createdAt
					return nil
				}}
			},
		},
	}

	got, err := repo.LatestByTenant(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("LatestByTenant returned error: %v", err)
	}
	if got.ID != versionID || got.SchemaHash != "hash-123" {
		t.Fatalf("LatestByTenant returned %+v, unexpected values", got)
	}
}

func TestTenantSchemaRepositoryLatestByTenantMapsNotFound(t *testing.T) {
	t.Parallel()

	repo := &TenantSchemaRepository{
		db: &fakeTenantSchemaDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}

	_, err := repo.LatestByTenant(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestTenantSchemaRepositoryCreate(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	versionID := uuid.New()
	capturedAt := time.Unix(1_700_000_000, 0).UTC()
	createdAt := capturedAt.Add(time.Second)
	blob := []byte(`{"database_name":"mission_app"}`)

	repo := &TenantSchemaRepository{
		db: &fakeTenantSchemaDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "INSERT INTO tenant_schemas") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 4 || args[0] != tenantID || args[2] != "hash-123" {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = versionID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*time.Time)) = capturedAt
					*(dest[3].(*string)) = "hash-123"
					*(dest[4].(*json.RawMessage)) = json.RawMessage(blob)
					*(dest[5].(*time.Time)) = createdAt
					return nil
				}}
			},
		},
	}

	got, err := repo.Create(
		context.Background(),
		tenantID,
		capturedAt,
		"hash-123",
		blob,
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.ID != versionID || got.TenantID != tenantID {
		t.Fatalf("Create returned %+v, unexpected values", got)
	}
}

func TestTenantSchemaRepositoryCreateWrapsScanError(t *testing.T) {
	t.Parallel()

	repo := &TenantSchemaRepository{
		db: &fakeTenantSchemaDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					return errors.New("scan failed")
				}}
			},
		},
	}

	_, err := repo.Create(
		context.Background(),
		uuid.New(),
		time.Now(),
		"hash-123",
		[]byte(`{}`),
	)
	if err == nil || !strings.Contains(err.Error(), "insert tenant schema") {
		t.Fatalf("err = %v, want wrapped insert tenant schema error", err)
	}
}

func TestTenantSchemaRepositoryReturnsTypedRecord(t *testing.T) {
	t.Parallel()

	record := model.TenantSchemaVersion{}
	if len(record.Blob) != 0 {
		t.Fatalf("expected zero blob length, got %d", len(record.Blob))
	}
}
