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

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

type fakeRows struct {
	scans  []func(dest ...any) error
	err    error
	idx    int
	closed bool
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.idx < len(r.scans) {
		r.idx++
		return true
	}
	return false
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.scans) {
		return errors.New("scan called without row")
	}
	return r.scans[r.idx-1](dest...)
}

func (r *fakeRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

type fakeTenantDB struct {
	beginFn    func(ctx context.Context) (pgx.Tx, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f *fakeTenantDB) Begin(ctx context.Context) (pgx.Tx, error) {
	if f.beginFn != nil {
		return f.beginFn(ctx)
	}
	return nil, errors.New("unexpected Begin call")
}

func (f *fakeTenantDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFn != nil {
		return f.queryFn(ctx, sql, args...)
	}
	return nil, errors.New("unexpected Query call")
}

func (f *fakeTenantDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, errors.New("unexpected Exec call")
}

func (f *fakeTenantDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return fakeRow{scanFn: func(dest ...any) error { return errors.New("unexpected QueryRow call") }}
}

type fakeTx struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	committed  bool
	rolledBack bool
}

func (f *fakeTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("unexpected Begin call")
}

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

func (f *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected CopyFrom call")
}

func (f *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (f *fakeTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (f *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("unexpected Prepare call")
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, errors.New("unexpected Exec call")
}

func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return fakeRow{scanFn: func(dest ...any) error { return errors.New("unexpected QueryRow call") }}
}

func (f *fakeTx) Conn() *pgx.Conn {
	return nil
}

func TestTenantRepositoryCreateWithOwner(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	createdAt := time.Unix(1700000000, 0)
	tx := &fakeTx{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			if !strings.Contains(sql, "INSERT INTO tenants") {
				t.Fatalf("unexpected SQL: %q", sql)
			}
			if len(args) != 2 || args[0] != "acme" || args[1] != "Acme" {
				t.Fatalf("unexpected tenant insert args: %#v", args)
			}
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = tenantID
				*(dest[1].(*string)) = "acme"
				*(dest[2].(*string)) = "Acme"
				*(dest[3].(*time.Time)) = createdAt
				return nil
			}}
		},
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "INSERT INTO tenant_users") {
				t.Fatalf("unexpected SQL: %q", sql)
			}
			if len(args) != 3 || args[0] != tenantID || args[1] != "user_123" || args[2] != string(model.RoleOwner) {
				t.Fatalf("unexpected owner insert args: %#v", args)
			}
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	repo := &TenantRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	got, err := repo.CreateWithOwner(context.Background(), "acme", "Acme", "user_123")
	if err != nil {
		t.Fatalf("CreateWithOwner returned error: %v", err)
	}
	if got.ID != tenantID || got.Slug != "acme" || got.Name != "Acme" || !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreateWithOwner returned %+v, unexpected values", got)
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
}

func TestTenantRepositoryCreateWithOwnerWrapsTenantInsertError(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				return errors.New("scan failed")
			}}
		},
	}
	repo := &TenantRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	_, err := repo.CreateWithOwner(context.Background(), "acme", "Acme", "user_123")
	if err == nil {
		t.Fatal("CreateWithOwner returned nil error")
	}
	if !strings.Contains(err.Error(), "insert tenant") {
		t.Fatalf("error = %v, want wrapped insert tenant error", err)
	}
	if !tx.rolledBack {
		t.Fatal("transaction was not rolled back")
	}
}

func TestTenantRepositoryCreateWithOwnerWrapsOwnerInsertError(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = uuid.New()
				*(dest[1].(*string)) = "acme"
				*(dest[2].(*string)) = "Acme"
				*(dest[3].(*time.Time)) = time.Unix(1700000000, 0)
				return nil
			}}
		},
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec failed")
		},
	}
	repo := &TenantRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	_, err := repo.CreateWithOwner(context.Background(), "acme", "Acme", "user_123")
	if err == nil {
		t.Fatal("CreateWithOwner returned nil error")
	}
	if !strings.Contains(err.Error(), "insert owner") {
		t.Fatalf("error = %v, want wrapped insert owner error", err)
	}
	if !tx.rolledBack {
		t.Fatal("transaction was not rolled back")
	}
}

func TestTenantRepositoryListForUser(t *testing.T) {
	t.Parallel()

	firstID := uuid.New()
	secondID := uuid.New()
	firstTime := time.Unix(1700000000, 0)
	secondTime := firstTime.Add(time.Hour)
	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = firstID
				*(dest[1].(*string)) = "acme"
				*(dest[2].(*string)) = "Acme"
				*(dest[3].(*time.Time)) = firstTime
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = secondID
				*(dest[1].(*string)) = "beta"
				*(dest[2].(*string)) = "Beta"
				*(dest[3].(*time.Time)) = secondTime
				return nil
			},
		},
	}
	repo := &TenantRepository{
		db: &fakeTenantDB{
			queryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
				if !strings.Contains(sql, "FROM tenants") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != "user_123" {
					t.Fatalf("unexpected query args: %#v", args)
				}
				return rows, nil
			},
		},
	}

	got, err := repo.ListForUser(context.Background(), "user_123")
	if err != nil {
		t.Fatalf("ListForUser returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != firstID || got[1].ID != secondID {
		t.Fatalf("ListForUser returned %+v, unexpected ids", got)
	}
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
}

func TestTenantRepositoryListForUserPropagatesQueryAndScanErrors(t *testing.T) {
	t.Parallel()

	repo := &TenantRepository{
		db: &fakeTenantDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		},
	}

	_, err := repo.ListForUser(context.Background(), "user_123")
	if err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("error = %v, want query failed", err)
	}

	rows := &fakeRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error { return errors.New("scan failed") },
		},
	}
	repo.db = &fakeTenantDB{
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
			return rows, nil
		},
	}

	_, err = repo.ListForUser(context.Background(), "user_123")
	if err == nil || !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("error = %v, want scan failed", err)
	}
}

func TestTenantRepositoryGetMembership(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	createdAt := time.Unix(1700000000, 0)
	repo := &TenantRepository{
		db: &fakeTenantDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "FROM tenant_users") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 2 || args[0] != tenantID || args[1] != "user_123" {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = tenantID
					*(dest[1].(*string)) = "user_123"
					*(dest[2].(*string)) = string(model.RoleOwner)
					*(dest[3].(*time.Time)) = createdAt
					return nil
				}}
			},
		},
	}

	got, err := repo.GetMembership(context.Background(), tenantID, "user_123")
	if err != nil {
		t.Fatalf("GetMembership returned error: %v", err)
	}
	if got.TenantID != tenantID || got.ClerkUserID != "user_123" || got.Role != model.RoleOwner || !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("GetMembership returned %+v, unexpected values", got)
	}
}

func TestTenantRepositoryGetMembershipMapsAndPropagatesErrors(t *testing.T) {
	t.Parallel()

	repo := &TenantRepository{
		db: &fakeTenantDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}

	_, err := repo.GetMembership(context.Background(), uuid.New(), "user_123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	repo.db = &fakeTenantDB{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				return errors.New("query failed")
			}}
		},
	}

	_, err = repo.GetMembership(context.Background(), uuid.New(), "user_123")
	if err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("error = %v, want query failed", err)
	}
}
