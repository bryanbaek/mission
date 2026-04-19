package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeTenantTokenDB struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (f *fakeTenantTokenDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFn != nil {
		return f.queryFn(ctx, sql, args...)
	}
	return nil, errors.New("unexpected Query call")
}

func (f *fakeTenantTokenDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return fakeRow{scanFn: func(dest ...any) error { return errors.New("unexpected QueryRow call") }}
}

func (f *fakeTenantTokenDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, errors.New("unexpected Exec call")
}

func TestTenantTokenRepositoryCreate(t *testing.T) {
	t.Parallel()

	tokenID := uuid.New()
	tenantID := uuid.New()
	createdAt := time.Unix(1700000000, 0)
	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "INSERT INTO tenant_tokens") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 3 || args[0] != tenantID || args[1] != "edge" {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = tokenID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*string)) = "edge"
					*(dest[3].(*time.Time)) = createdAt
					return nil
				}}
			},
		},
	}

	got, err := repo.Create(context.Background(), tenantID, "edge", []byte("hash"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.ID != tokenID || got.TenantID != tenantID || got.Label != "edge" || !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("Create returned %+v, unexpected values", got)
	}
}

func TestTenantTokenRepositoryCreateWrapsScanError(t *testing.T) {
	t.Parallel()

	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					return errors.New("scan failed")
				}}
			},
		},
	}

	_, err := repo.Create(context.Background(), uuid.New(), "edge", []byte("hash"))
	if err == nil || !strings.Contains(err.Error(), "insert token") {
		t.Fatalf("error = %v, want wrapped insert token error", err)
	}
}

func TestTenantTokenRepositoryListByTenant(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	firstTokenID := uuid.New()
	secondTokenID := uuid.New()
	firstTime := time.Unix(1700000000, 0)
	secondTime := firstTime.Add(time.Hour)
	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = firstTokenID
				*(dest[1].(*uuid.UUID)) = tenantID
				*(dest[2].(*string)) = "edge-a"
				*(dest[3].(*time.Time)) = firstTime
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = secondTokenID
				*(dest[1].(*uuid.UUID)) = tenantID
				*(dest[2].(*string)) = "edge-b"
				*(dest[3].(*time.Time)) = secondTime
				return nil
			},
		},
	}
	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			queryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
				if !strings.Contains(sql, "FROM tenant_tokens") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != tenantID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return rows, nil
			},
		},
	}

	got, err := repo.ListByTenant(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListByTenant returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != firstTokenID || got[1].ID != secondTokenID {
		t.Fatalf("ListByTenant returned %+v, unexpected ids", got)
	}
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
}

func TestTenantTokenRepositoryListByTenantPropagatesErrors(t *testing.T) {
	t.Parallel()

	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		},
	}

	_, err := repo.ListByTenant(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("error = %v, want query failed", err)
	}

	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error { return errors.New("scan failed") },
		},
	}
	repo.db = &fakeTenantTokenDB{
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}

	_, err = repo.ListByTenant(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("error = %v, want scan failed", err)
	}
}

func TestTenantTokenRepositoryRevoke(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	tokenID := uuid.New()
	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if !strings.Contains(sql, "UPDATE tenant_tokens SET revoked_at = NOW()") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 2 || args[0] != tokenID || args[1] != tenantID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		},
	}

	if err := repo.Revoke(context.Background(), tenantID, tokenID); err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}
}

func TestTenantTokenRepositoryRevokeMapsAndPropagatesErrors(t *testing.T) {
	t.Parallel()

	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			},
		},
	}

	err := repo.Revoke(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	repo.db = &fakeTenantTokenDB{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec failed")
		},
	}

	err = repo.Revoke(context.Background(), uuid.New(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("error = %v, want exec failed", err)
	}
}

func TestTenantTokenRepositoryLookupActiveByHash(t *testing.T) {
	t.Parallel()

	tokenID := uuid.New()
	tenantID := uuid.New()
	createdAt := time.Unix(1700000000, 0)
	lastUsedAt := createdAt.Add(time.Hour)
	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "FROM tenant_tokens WHERE token_hash = $1") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || string(args[0].([]byte)) != "hash" {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = tokenID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*string)) = "edge"
					*(dest[3].(*time.Time)) = createdAt
					*(dest[4].(**time.Time)) = &lastUsedAt
					*(dest[5].(**time.Time)) = nil
					return nil
				}}
			},
		},
	}

	got, err := repo.LookupActiveByHash(context.Background(), []byte("hash"))
	if err != nil {
		t.Fatalf("LookupActiveByHash returned error: %v", err)
	}
	if got.ID != tokenID || got.TenantID != tenantID || got.Label != "edge" {
		t.Fatalf("LookupActiveByHash returned %+v, unexpected values", got)
	}
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(lastUsedAt) {
		t.Fatalf("LookupActiveByHash last used = %+v, want %s", got.LastUsedAt, lastUsedAt)
	}
	if got.RevokedAt != nil {
		t.Fatalf("LookupActiveByHash revokedAt = %+v, want nil", got.RevokedAt)
	}
}

func TestTenantTokenRepositoryLookupActiveByHashMapsAndPropagatesErrors(t *testing.T) {
	t.Parallel()

	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}

	_, err := repo.LookupActiveByHash(context.Background(), []byte("hash"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	repo.db = &fakeTenantTokenDB{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				return errors.New("query failed")
			}}
		},
	}

	_, err = repo.LookupActiveByHash(context.Background(), []byte("hash"))
	if err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("error = %v, want query failed", err)
	}
}

func TestTenantTokenRepositoryTouchLastUsed(t *testing.T) {
	t.Parallel()

	tokenID := uuid.New()
	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if !strings.Contains(sql, "UPDATE tenant_tokens SET last_used_at = NOW()") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != tokenID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		},
	}

	if err := repo.TouchLastUsed(context.Background(), tokenID); err != nil {
		t.Fatalf("TouchLastUsed returned error: %v", err)
	}
}

func TestTenantTokenRepositoryTouchLastUsedMapsAndPropagatesErrors(t *testing.T) {
	t.Parallel()

	repo := &TenantTokenRepository{
		db: &fakeTenantTokenDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			},
		},
	}

	err := repo.TouchLastUsed(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	repo.db = &fakeTenantTokenDB{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec failed")
		},
	}

	err = repo.TouchLastUsed(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("error = %v, want exec failed", err)
	}
}
