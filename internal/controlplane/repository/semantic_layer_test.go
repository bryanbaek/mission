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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

func TestTenantSemanticLayerRepositoryLatestApprovedBySchemaVersion(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()
	createdAt := time.Unix(1_700_000_100, 0).UTC()
	approvedAt := createdAt.Add(time.Minute)
	approvedBy := "user_123"
	content := json.RawMessage(`{"tables":[{"table_schema":"mission_app","table_name":"customers","table_type":"BASE TABLE","table_comment":"","description":"고객","columns":[{"table_schema":"mission_app","table_name":"customers","column_name":"customer_code","ordinal_position":1,"data_type":"varchar","column_type":"varchar(64)","is_nullable":false,"column_comment":"","description":"고객 코드"}]}],"entities":[],"candidate_metrics":[]}`)

	repo := &TenantSemanticLayerRepository{
		db: &fakeTenantDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "status = 'approved'") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 2 || args[0] != tenantID || args[1] != schemaVersionID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = layerID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*uuid.UUID)) = schemaVersionID
					*(dest[3].(*model.SemanticLayerStatus)) = model.SemanticLayerStatusApproved
					*(dest[4].(*json.RawMessage)) = content
					*(dest[5].(*time.Time)) = createdAt
					*(dest[6].(**time.Time)) = &approvedAt
					*(dest[7].(**string)) = &approvedBy
					return nil
				}}
			},
		},
	}

	got, err := repo.LatestApprovedBySchemaVersion(
		context.Background(),
		tenantID,
		schemaVersionID,
	)
	if err != nil {
		t.Fatalf("LatestApprovedBySchemaVersion returned error: %v", err)
	}
	if got.ID != layerID || got.Status != model.SemanticLayerStatusApproved {
		t.Fatalf("LatestApprovedBySchemaVersion returned %+v", got)
	}
	if got.ApprovedAt == nil || !got.ApprovedAt.Equal(approvedAt) {
		t.Fatalf("ApprovedAt = %v, want %v", got.ApprovedAt, approvedAt)
	}
	if got.ApprovedByUserID == nil || *got.ApprovedByUserID != approvedBy {
		t.Fatalf("ApprovedByUserID = %v, want %q", got.ApprovedByUserID, approvedBy)
	}
}

func TestTenantSemanticLayerRepositoryLatestApprovedByTenant(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()
	createdAt := time.Unix(1_700_000_150, 0).UTC()
	approvedAt := createdAt.Add(time.Minute)
	approvedBy := "user_321"
	content := json.RawMessage(`{"tables":[{"table_schema":"mission_app","table_name":"orders","table_type":"BASE TABLE","table_comment":"","description":"주문","columns":[{"table_schema":"mission_app","table_name":"orders","column_name":"order_id","ordinal_position":1,"data_type":"bigint","column_type":"bigint","is_nullable":false,"column_comment":"","description":"주문 ID"}]}],"entities":[],"candidate_metrics":[]}`)

	repo := &TenantSemanticLayerRepository{
		db: &fakeTenantDB{
			queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "status = 'approved'") ||
					!strings.Contains(sql, "WHERE tenant_id = $1") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != tenantID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = layerID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*uuid.UUID)) = schemaVersionID
					*(dest[3].(*model.SemanticLayerStatus)) = model.SemanticLayerStatusApproved
					*(dest[4].(*json.RawMessage)) = content
					*(dest[5].(*time.Time)) = createdAt
					*(dest[6].(**time.Time)) = &approvedAt
					*(dest[7].(**string)) = &approvedBy
					return nil
				}}
			},
		},
	}

	got, err := repo.LatestApprovedByTenant(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("LatestApprovedByTenant returned error: %v", err)
	}
	if got.ID != layerID || got.SchemaVersionID != schemaVersionID {
		t.Fatalf("LatestApprovedByTenant returned %+v", got)
	}
	if got.ApprovedAt == nil || !got.ApprovedAt.Equal(approvedAt) {
		t.Fatalf("ApprovedAt = %v, want %v", got.ApprovedAt, approvedAt)
	}
}

func TestTenantSemanticLayerRepositoryCreateDraftVersionArchivesPriorDraft(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()
	createdAt := time.Unix(1_700_000_200, 0).UTC()
	content := []byte(`{"tables":[{"table_schema":"mission_app","table_name":"customers","table_type":"BASE TABLE","table_comment":"","description":"고객","columns":[{"table_schema":"mission_app","table_name":"customers","column_name":"customer_code","ordinal_position":1,"data_type":"varchar","column_type":"varchar(64)","is_nullable":false,"column_comment":"","description":"고객 코드"}]}],"entities":[],"candidate_metrics":[]}`)

	archiveCalled := false
	tx := &fakeTx{
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "SET status = 'archived'") {
				t.Fatalf("unexpected SQL: %q", sql)
			}
			if len(args) != 2 || args[0] != tenantID || args[1] != schemaVersionID {
				t.Fatalf("unexpected args: %#v", args)
			}
			archiveCalled = true
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			if !strings.Contains(sql, "INSERT INTO tenant_semantic_layers") {
				t.Fatalf("unexpected SQL: %q", sql)
			}
			if len(args) != 6 {
				t.Fatalf("unexpected arg count: %d", len(args))
			}
			if args[0] != tenantID || args[1] != schemaVersionID {
				t.Fatalf("unexpected IDs: %#v", args[:2])
			}
			if got := args[2]; got != string(model.SemanticLayerStatusDraft) {
				t.Fatalf("status = %v, want draft", got)
			}
			if got := args[3]; string(got.([]byte)) != string(content) {
				t.Fatalf("content mismatch")
			}
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = layerID
				*(dest[1].(*uuid.UUID)) = tenantID
				*(dest[2].(*uuid.UUID)) = schemaVersionID
				*(dest[3].(*model.SemanticLayerStatus)) = model.SemanticLayerStatusDraft
				*(dest[4].(*json.RawMessage)) = json.RawMessage(content)
				*(dest[5].(*time.Time)) = createdAt
				*(dest[6].(**time.Time)) = nil
				*(dest[7].(**string)) = nil
				return nil
			}}
		},
	}

	repo := &TenantSemanticLayerRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	got, err := repo.CreateDraftVersion(
		context.Background(),
		tenantID,
		schemaVersionID,
		content,
	)
	if err != nil {
		t.Fatalf("CreateDraftVersion returned error: %v", err)
	}
	if !archiveCalled {
		t.Fatal("expected prior drafts to be archived")
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	if got.ID != layerID || got.Status != model.SemanticLayerStatusDraft {
		t.Fatalf("CreateDraftVersion returned %+v", got)
	}
}

func TestTenantSemanticLayerRepositoryApproveArchivesPeersAndStampsApproval(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()
	approvedAt := time.Unix(1_700_000_300, 0).UTC()
	approvedBy := "user_456"
	content := json.RawMessage(`{"tables":[{"table_schema":"mission_app","table_name":"customers","table_type":"BASE TABLE","table_comment":"","description":"고객","columns":[{"table_schema":"mission_app","table_name":"customers","column_name":"customer_code","ordinal_position":1,"data_type":"varchar","column_type":"varchar(64)","is_nullable":false,"column_comment":"","description":"고객 코드"}]}],"entities":[],"candidate_metrics":[]}`)

	callCount := 0
	archiveArgsChecked := false
	tx := &fakeTx{
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "status IN ('draft', 'approved')") {
				t.Fatalf("unexpected SQL: %q", sql)
			}
			if len(args) != 3 || args[0] != tenantID || args[1] != schemaVersionID || args[2] != layerID {
				t.Fatalf("unexpected args: %#v", args)
			}
			archiveArgsChecked = true
			return pgconn.NewCommandTag("UPDATE 2"), nil
		},
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			callCount++
			switch callCount {
			case 1:
				if !strings.Contains(sql, "FOR UPDATE") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = layerID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*uuid.UUID)) = schemaVersionID
					*(dest[3].(*model.SemanticLayerStatus)) = model.SemanticLayerStatusDraft
					*(dest[4].(*json.RawMessage)) = content
					*(dest[5].(*time.Time)) = approvedAt.Add(-time.Minute)
					*(dest[6].(**time.Time)) = nil
					*(dest[7].(**string)) = nil
					return nil
				}}
			case 2:
				if !strings.Contains(sql, "SET") || !strings.Contains(sql, "status = 'approved'") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 3 || args[0] != layerID {
					t.Fatalf("unexpected args: %#v", args)
				}
				gotApprovedAt, ok := args[1].(time.Time)
				if !ok || !gotApprovedAt.Equal(approvedAt) {
					t.Fatalf("approvedAt arg = %#v, want %v", args[1], approvedAt)
				}
				gotApprovedBy, ok := args[2].(*string)
				if !ok || gotApprovedBy == nil || *gotApprovedBy != approvedBy {
					t.Fatalf("approvedBy arg = %#v, want %q", args[2], approvedBy)
				}
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = layerID
					*(dest[1].(*uuid.UUID)) = tenantID
					*(dest[2].(*uuid.UUID)) = schemaVersionID
					*(dest[3].(*model.SemanticLayerStatus)) = model.SemanticLayerStatusApproved
					*(dest[4].(*json.RawMessage)) = content
					*(dest[5].(*time.Time)) = approvedAt.Add(-time.Minute)
					*(dest[6].(**time.Time)) = &approvedAt
					*(dest[7].(**string)) = &approvedBy
					return nil
				}}
			default:
				t.Fatalf("unexpected QueryRow call %d", callCount)
				return fakeRow{}
			}
		},
	}

	repo := &TenantSemanticLayerRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	got, err := repo.Approve(
		context.Background(),
		tenantID,
		layerID,
		approvedAt,
		approvedBy,
	)
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if !archiveArgsChecked {
		t.Fatal("expected peer archival query to run")
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	if got.Status != model.SemanticLayerStatusApproved {
		t.Fatalf("status = %q, want approved", got.Status)
	}
	if got.ApprovedAt == nil || !got.ApprovedAt.Equal(approvedAt) {
		t.Fatalf("ApprovedAt = %v, want %v", got.ApprovedAt, approvedAt)
	}
	if got.ApprovedByUserID == nil || *got.ApprovedByUserID != approvedBy {
		t.Fatalf("ApprovedByUserID = %v, want %q", got.ApprovedByUserID, approvedBy)
	}
}

func TestTenantSemanticLayerRepositorySelectOneMapsNotFound(t *testing.T) {
	t.Parallel()

	repo := &TenantSemanticLayerRepository{
		db: &fakeTenantDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}

	_, err := repo.LatestDraftBySchemaVersion(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
