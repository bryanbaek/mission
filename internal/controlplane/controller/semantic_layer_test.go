package controller

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeSemanticMembershipChecker struct {
	ensureFn func(context.Context, uuid.UUID, string) (model.TenantUser, error)
}

func (f fakeSemanticMembershipChecker) EnsureMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (model.TenantUser, error) {
	return f.ensureFn(ctx, tenantID, clerkUserID)
}

type fakeSemanticSchemaStore struct {
	latestFn func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error)
	getFn    func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSchemaVersion, error)
}

func (f fakeSemanticSchemaStore) LatestByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantSchemaVersion, error) {
	return f.latestFn(ctx, tenantID)
}

func (f fakeSemanticSchemaStore) GetByTenantAndID(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSchemaVersion, error) {
	return f.getFn(ctx, tenantID, schemaVersionID)
}

type fakeSemanticLayerStore struct {
	latestDraftFn    func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error)
	latestApprovedFn func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error)
	getByIDFn        func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error)
	listApprovedFn   func(context.Context, uuid.UUID) ([]model.TenantSemanticLayer, error)
	createDraftFn    func(context.Context, uuid.UUID, uuid.UUID, []byte) (model.TenantSemanticLayer, error)
	approveFn        func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) (model.TenantSemanticLayer, error)
}

func (f fakeSemanticLayerStore) LatestDraftBySchemaVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return f.latestDraftFn(ctx, tenantID, schemaVersionID)
}

func (f fakeSemanticLayerStore) LatestApprovedBySchemaVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return f.latestApprovedFn(ctx, tenantID, schemaVersionID)
}

func (f fakeSemanticLayerStore) GetByID(
	ctx context.Context,
	tenantID, id uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return f.getByIDFn(ctx, tenantID, id)
}

func (f fakeSemanticLayerStore) ListApprovedHistoryByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.TenantSemanticLayer, error) {
	if f.listApprovedFn != nil {
		return f.listApprovedFn(ctx, tenantID)
	}
	return nil, errors.New("unexpected ListApprovedHistoryByTenant call")
}

func (f fakeSemanticLayerStore) CreateDraftVersion(
	ctx context.Context,
	tenantID, schemaVersionID uuid.UUID,
	content []byte,
) (model.TenantSemanticLayer, error) {
	return f.createDraftFn(ctx, tenantID, schemaVersionID, content)
}

func (f fakeSemanticLayerStore) Approve(
	ctx context.Context,
	tenantID, id uuid.UUID,
	approvedAt time.Time,
	approvedByUserID string,
) (model.TenantSemanticLayer, error) {
	return f.approveFn(ctx, tenantID, id, approvedAt, approvedByUserID)
}

type fakeSemanticCompleter struct {
	gotRequest llm.CompletionRequest
	response   llm.CompletionResponse
	err        error
}

func (f *fakeSemanticCompleter) Complete(
	_ context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	f.gotRequest = req
	if f.err != nil {
		return llm.CompletionResponse{}, f.err
	}
	return f.response, nil
}

func semanticTestSchemaBlob() model.SchemaBlob {
	return model.SchemaBlob{
		DatabaseName: "mission_app",
		Tables: []model.SchemaTable{
			{
				TableSchema:  "mission_app",
				TableName:    "customers",
				TableType:    "BASE TABLE",
				TableComment: "Customer master data",
			},
			{
				TableSchema:  "mission_app",
				TableName:    "orders",
				TableType:    "BASE TABLE",
				TableComment: "Sales orders",
			},
		},
		Columns: []model.SchemaColumn{
			{
				TableSchema:     "mission_app",
				TableName:       "customers",
				ColumnName:      "customer_code",
				OrdinalPosition: 1,
				DataType:        "varchar",
				ColumnType:      "varchar(64)",
				IsNullable:      false,
				ColumnComment:   "External customer code",
			},
			{
				TableSchema:     "mission_app",
				TableName:       "customers",
				ColumnName:      "name",
				OrdinalPosition: 2,
				DataType:        "varchar",
				ColumnType:      "varchar(255)",
				IsNullable:      false,
				ColumnComment:   "Display name",
			},
			{
				TableSchema:     "mission_app",
				TableName:       "orders",
				ColumnName:      "order_total",
				OrdinalPosition: 1,
				DataType:        "decimal",
				ColumnType:      "decimal(12,2)",
				IsNullable:      false,
				ColumnComment:   "Total amount",
			},
		},
	}
}

func semanticGeneratedContent() model.SemanticLayerContent {
	return model.SemanticLayerContent{
		Tables: []model.SemanticTable{
			{
				TableSchema: "mission_app",
				TableName:   "customers",
				Description: "고객 기본 정보",
				Columns: []model.SemanticColumn{
					{
						TableSchema: "mission_app",
						TableName:   "customers",
						ColumnName:  "customer_code",
						Description: "고객 코드",
					},
					{
						TableSchema: "mission_app",
						TableName:   "customers",
						ColumnName:  "name",
						Description: "고객명",
					},
				},
			},
			{
				TableSchema: "mission_app",
				TableName:   "orders",
				Description: "[추정] 주문 기록",
				Columns: []model.SemanticColumn{
					{
						TableSchema: "mission_app",
						TableName:   "orders",
						ColumnName:  "order_total",
						Description: "주문 총액",
					},
				},
			},
		},
		Entities: []model.SemanticEntity{
			{
				Name:         "고객",
				Description:  "핵심 고객 엔터티",
				SourceTables: []string{"mission_app.customers"},
			},
		},
		CandidateMetrics: []model.CandidateMetric{
			{
				Name:         "주문 금액 합계",
				Description:  "주문 총액의 합계",
				SourceTables: []string{"mission_app.orders"},
			},
		},
	}
}

func semanticLayerBytes(t *testing.T, content model.SemanticLayerContent) []byte {
	t.Helper()

	body, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return body
}

func semanticLayerRecord(
	t *testing.T,
	layerID, tenantID, schemaVersionID uuid.UUID,
	status model.SemanticLayerStatus,
	content model.SemanticLayerContent,
) model.TenantSemanticLayer {
	t.Helper()

	return model.TenantSemanticLayer{
		ID:              layerID,
		TenantID:        tenantID,
		SchemaVersionID: schemaVersionID,
		Status:          status,
		Content:         semanticLayerBytes(t, content),
		CreatedAt:       time.Unix(1_700_000_000, 0).UTC(),
	}
}

func TestSemanticLayerControllerGetLatestSchemaStates(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	schemaBlob, err := json.Marshal(semanticTestSchemaBlob())
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	t.Run("needs draft when the latest schema has no layers", func(t *testing.T) {
		t.Parallel()

		ctrl := NewSemanticLayerController(
			fakeSemanticMembershipChecker{
				ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
					return model.TenantUser{TenantID: tenantID}, nil
				},
			},
			fakeSemanticSchemaStore{
				latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
					return model.TenantSchemaVersion{
						ID:         schemaVersionID,
						TenantID:   tenantID,
						SchemaHash: "hash-123",
						Blob:       schemaBlob,
					}, nil
				},
				getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSchemaVersion, error) {
					return model.TenantSchemaVersion{}, errors.New("unexpected GetByTenantAndID call")
				},
			},
			fakeSemanticLayerStore{
				latestDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, repository.ErrNotFound
				},
				latestApprovedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, repository.ErrNotFound
				},
				getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, errors.New("unexpected GetByID call")
				},
				createDraftFn: func(context.Context, uuid.UUID, uuid.UUID, []byte) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, errors.New("unexpected CreateDraftVersion call")
				},
				approveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, errors.New("unexpected Approve call")
				},
			},
			&fakeSemanticCompleter{},
			SemanticLayerControllerConfig{},
		)

		got, err := ctrl.Get(context.Background(), tenantID, "user_123")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if !got.HasSchema || !got.NeedsDraft {
			t.Fatalf("Get returned %+v, want hasSchema=true needsDraft=true", got)
		}
		if got.CurrentLayer != nil || got.ApprovedBaseline != nil {
			t.Fatalf("unexpected layers in %+v", got)
		}
	})

	t.Run("uses draft as current layer and approved as baseline", func(t *testing.T) {
		t.Parallel()

		draftID := uuid.New()
		approvedID := uuid.New()
		draftContent := semanticGeneratedContent()
		approvedContent := semanticGeneratedContent()
		approvedContent.Tables[0].Description = "이전 고객 설명"

		ctrl := NewSemanticLayerController(
			fakeSemanticMembershipChecker{
				ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
					return model.TenantUser{TenantID: tenantID}, nil
				},
			},
			fakeSemanticSchemaStore{
				latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
					return model.TenantSchemaVersion{
						ID:         schemaVersionID,
						TenantID:   tenantID,
						SchemaHash: "hash-123",
						Blob:       schemaBlob,
					}, nil
				},
				getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSchemaVersion, error) {
					return model.TenantSchemaVersion{}, errors.New("unexpected GetByTenantAndID call")
				},
			},
			fakeSemanticLayerStore{
				latestDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
					return semanticLayerRecord(
						t,
						draftID,
						tenantID,
						schemaVersionID,
						model.SemanticLayerStatusDraft,
						draftContent,
					), nil
				},
				latestApprovedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
					return semanticLayerRecord(
						t,
						approvedID,
						tenantID,
						schemaVersionID,
						model.SemanticLayerStatusApproved,
						approvedContent,
					), nil
				},
				getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, errors.New("unexpected GetByID call")
				},
				createDraftFn: func(context.Context, uuid.UUID, uuid.UUID, []byte) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, errors.New("unexpected CreateDraftVersion call")
				},
				approveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) (model.TenantSemanticLayer, error) {
					return model.TenantSemanticLayer{}, errors.New("unexpected Approve call")
				},
			},
			&fakeSemanticCompleter{},
			SemanticLayerControllerConfig{},
		)

		got, err := ctrl.Get(context.Background(), tenantID, "user_123")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.NeedsDraft {
			t.Fatal("NeedsDraft = true, want false")
		}
		if got.CurrentLayer == nil || got.CurrentLayer.Layer.ID != draftID {
			t.Fatalf("CurrentLayer = %+v, want draft %s", got.CurrentLayer, draftID)
		}
		if got.ApprovedBaseline == nil || got.ApprovedBaseline.Layer.ID != approvedID {
			t.Fatalf("ApprovedBaseline = %+v, want approved %s", got.ApprovedBaseline, approvedID)
		}
	})
}

func TestSemanticLayerControllerDraftBuildsStructuredRequestAndPersistsContent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()
	schema := semanticTestSchemaBlob()
	schemaBlob, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	generated := semanticGeneratedContent()
	generatedJSON, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	completer := &fakeSemanticCompleter{
		response: llm.CompletionResponse{
			Content:  string(generatedJSON),
			Provider: "deepseek",
			Model:    "deepseek-chat",
			Usage: llm.Usage{
				Provider:                 "deepseek",
				Model:                    "deepseek-chat",
				InputTokens:              321,
				OutputTokens:             98,
				CacheCreationInputTokens: 321,
				CacheReadInputTokens:     64,
			},
		},
	}

	var persisted model.SemanticLayerContent
	ctrl := NewSemanticLayerController(
		fakeSemanticMembershipChecker{
			ensureFn: func(_ context.Context, gotTenantID uuid.UUID, clerkUserID string) (model.TenantUser, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				if clerkUserID != "user_123" {
					t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
				}
				return model.TenantUser{TenantID: tenantID}, nil
			},
		},
		fakeSemanticSchemaStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant call")
			},
			getFn: func(_ context.Context, gotTenantID, gotSchemaVersionID uuid.UUID) (model.TenantSchemaVersion, error) {
				if gotTenantID != tenantID || gotSchemaVersionID != schemaVersionID {
					t.Fatalf("got IDs = (%s, %s), want (%s, %s)", gotTenantID, gotSchemaVersionID, tenantID, schemaVersionID)
				}
				return model.TenantSchemaVersion{
					ID:         schemaVersionID,
					TenantID:   tenantID,
					SchemaHash: "hash-123",
					Blob:       schemaBlob,
				}, nil
			},
		},
		fakeSemanticLayerStore{
			latestDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion call")
			},
			latestApprovedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion call")
			},
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected GetByID call")
			},
			createDraftFn: func(_ context.Context, gotTenantID, gotSchemaVersionID uuid.UUID, content []byte) (model.TenantSemanticLayer, error) {
				if gotTenantID != tenantID || gotSchemaVersionID != schemaVersionID {
					t.Fatalf("got IDs = (%s, %s), want (%s, %s)", gotTenantID, gotSchemaVersionID, tenantID, schemaVersionID)
				}
				if err := json.Unmarshal(content, &persisted); err != nil {
					t.Fatalf("json.Unmarshal returned error: %v", err)
				}
				return model.TenantSemanticLayer{
					ID:              layerID,
					TenantID:        tenantID,
					SchemaVersionID: schemaVersionID,
					Status:          model.SemanticLayerStatusDraft,
					Content:         append([]byte(nil), content...),
					CreatedAt:       time.Unix(1_700_000_500, 0).UTC(),
				}, nil
			},
			approveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected Approve call")
			},
		},
		completer,
		SemanticLayerControllerConfig{
			Model:     "deepseek-chat",
			MaxTokens: 2048,
		},
	)

	got, err := ctrl.Draft(context.Background(), tenantID, "user_123", schemaVersionID)
	if err != nil {
		t.Fatalf("Draft returned error: %v", err)
	}
	if got.Layer.Layer.ID != layerID {
		t.Fatalf("layer ID = %s, want %s", got.Layer.Layer.ID, layerID)
	}
	if got.Usage.CacheReadInputTokens != 64 {
		t.Fatalf("CacheReadInputTokens = %d, want 64", got.Usage.CacheReadInputTokens)
	}
	if got.Usage.Provider != "deepseek" || got.Usage.Model != "deepseek-chat" {
		t.Fatalf("usage = %+v, want deepseek/deepseek-chat", got.Usage)
	}
	if completer.gotRequest.Model != "deepseek-chat" {
		t.Fatalf("model = %q, want deepseek-chat", completer.gotRequest.Model)
	}
	if completer.gotRequest.OutputFormat == nil || completer.gotRequest.OutputFormat.Schema == nil {
		t.Fatal("expected structured output schema")
	}
	if completer.gotRequest.CacheControl == nil || completer.gotRequest.CacheControl.TTL != "1h" {
		t.Fatalf("cache control = %+v, want TTL 1h", completer.gotRequest.CacheControl)
	}
	if !strings.Contains(completer.gotRequest.System, "한국어") {
		t.Fatalf("system prompt missing Korean instructions: %q", completer.gotRequest.System)
	}
	if len(completer.gotRequest.Messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(completer.gotRequest.Messages))
	}
	if gotPrompt := completer.gotRequest.Messages[0].CachedContent; gotPrompt != buildSemanticLayerUserPrompt(schemaBlob) {
		t.Fatalf("user prompt mismatch:\n%s", gotPrompt)
	}
	if len(persisted.Tables) != len(schema.Tables) {
		t.Fatalf("persisted table count = %d, want %d", len(persisted.Tables), len(schema.Tables))
	}
	if persisted.Tables[0].Description != "고객 기본 정보" {
		t.Fatalf("table description = %q, want 고객 기본 정보", persisted.Tables[0].Description)
	}
	if persisted.Tables[0].Columns[0].Description != "고객 코드" {
		t.Fatalf("column description = %q, want 고객 코드", persisted.Tables[0].Columns[0].Description)
	}
	if got.Layer.Content.Entities[0].Name != "고객" {
		t.Fatalf("entity name = %q, want 고객", got.Layer.Content.Entities[0].Name)
	}
}

func TestSemanticLayerControllerUpdatePreservesReadOnlySections(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	baseLayerID := uuid.New()
	newDraftID := uuid.New()
	baseContent := semanticGeneratedContent()
	edited := semanticGeneratedContent()
	edited.Tables[0].Description = "수정된 고객 설명"
	edited.Tables[0].Columns[0].Description = "수정된 고객 코드 설명"
	edited.Entities = []model.SemanticEntity{{
		Name:         "변경하면 안 됨",
		Description:  "무시되어야 함",
		SourceTables: []string{"mission_app.orders"},
	}}
	edited.CandidateMetrics = []model.CandidateMetric{{
		Name:         "변경하면 안 됨",
		Description:  "무시되어야 함",
		SourceTables: []string{"mission_app.customers"},
	}}

	var persisted model.SemanticLayerContent
	ctrl := NewSemanticLayerController(
		fakeSemanticMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID}, nil
			},
		},
		fakeSemanticSchemaStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant call")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected GetByTenantAndID call")
			},
		},
		fakeSemanticLayerStore{
			latestDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion call")
			},
			latestApprovedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion call")
			},
			getByIDFn: func(_ context.Context, gotTenantID, gotLayerID uuid.UUID) (model.TenantSemanticLayer, error) {
				if gotTenantID != tenantID || gotLayerID != baseLayerID {
					t.Fatalf("got IDs = (%s, %s), want (%s, %s)", gotTenantID, gotLayerID, tenantID, baseLayerID)
				}
				return semanticLayerRecord(
					t,
					baseLayerID,
					tenantID,
					schemaVersionID,
					model.SemanticLayerStatusDraft,
					baseContent,
				), nil
			},
			createDraftFn: func(_ context.Context, gotTenantID, gotSchemaVersionID uuid.UUID, content []byte) (model.TenantSemanticLayer, error) {
				if gotTenantID != tenantID || gotSchemaVersionID != schemaVersionID {
					t.Fatalf("got IDs = (%s, %s), want (%s, %s)", gotTenantID, gotSchemaVersionID, tenantID, schemaVersionID)
				}
				if err := json.Unmarshal(content, &persisted); err != nil {
					t.Fatalf("json.Unmarshal returned error: %v", err)
				}
				return model.TenantSemanticLayer{
					ID:              newDraftID,
					TenantID:        tenantID,
					SchemaVersionID: schemaVersionID,
					Status:          model.SemanticLayerStatusDraft,
					Content:         append([]byte(nil), content...),
					CreatedAt:       time.Unix(1_700_000_600, 0).UTC(),
				}, nil
			},
			approveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected Approve call")
			},
		},
		&fakeSemanticCompleter{},
		SemanticLayerControllerConfig{},
	)

	got, err := ctrl.Update(context.Background(), tenantID, "user_123", baseLayerID, edited)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got.Layer.ID != newDraftID {
		t.Fatalf("layer ID = %s, want %s", got.Layer.ID, newDraftID)
	}
	if persisted.Tables[0].Description != "수정된 고객 설명" {
		t.Fatalf("table description = %q, want updated value", persisted.Tables[0].Description)
	}
	if persisted.Tables[0].Columns[0].Description != "수정된 고객 코드 설명" {
		t.Fatalf("column description = %q, want updated value", persisted.Tables[0].Columns[0].Description)
	}
	if persisted.Entities[0].Name != baseContent.Entities[0].Name {
		t.Fatalf("entity name = %q, want %q", persisted.Entities[0].Name, baseContent.Entities[0].Name)
	}
	if persisted.CandidateMetrics[0].Name != baseContent.CandidateMetrics[0].Name {
		t.Fatalf("metric name = %q, want %q", persisted.CandidateMetrics[0].Name, baseContent.CandidateMetrics[0].Name)
	}
}

func TestSemanticLayerControllerApproveUsesCurrentUserAndTimestamp(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()
	now := time.Unix(1_700_000_700, 0).UTC()
	content := semanticGeneratedContent()

	ctrl := NewSemanticLayerController(
		fakeSemanticMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID}, nil
			},
		},
		fakeSemanticSchemaStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant call")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected GetByTenantAndID call")
			},
		},
		fakeSemanticLayerStore{
			latestDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion call")
			},
			latestApprovedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion call")
			},
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return semanticLayerRecord(
					t,
					layerID,
					tenantID,
					schemaVersionID,
					model.SemanticLayerStatusDraft,
					content,
				), nil
			},
			createDraftFn: func(context.Context, uuid.UUID, uuid.UUID, []byte) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected CreateDraftVersion call")
			},
			approveFn: func(_ context.Context, gotTenantID, gotLayerID uuid.UUID, approvedAt time.Time, approvedBy string) (model.TenantSemanticLayer, error) {
				if gotTenantID != tenantID || gotLayerID != layerID {
					t.Fatalf("got IDs = (%s, %s), want (%s, %s)", gotTenantID, gotLayerID, tenantID, layerID)
				}
				if !approvedAt.Equal(now) {
					t.Fatalf("approvedAt = %v, want %v", approvedAt, now)
				}
				if approvedBy != "user_123" {
					t.Fatalf("approvedBy = %q, want user_123", approvedBy)
				}
				approvedByCopy := approvedBy
				nowCopy := approvedAt
				return model.TenantSemanticLayer{
					ID:               layerID,
					TenantID:         tenantID,
					SchemaVersionID:  schemaVersionID,
					Status:           model.SemanticLayerStatusApproved,
					Content:          semanticLayerBytes(t, content),
					CreatedAt:        now.Add(-time.Minute),
					ApprovedAt:       &nowCopy,
					ApprovedByUserID: &approvedByCopy,
				}, nil
			},
		},
		&fakeSemanticCompleter{},
		SemanticLayerControllerConfig{
			Now: func() time.Time { return now },
		},
	)

	got, err := ctrl.Approve(context.Background(), tenantID, "user_123", layerID)
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if got.Layer.Status != model.SemanticLayerStatusApproved {
		t.Fatalf("status = %q, want approved", got.Layer.Status)
	}
	if got.Layer.ApprovedByUserID == nil || *got.Layer.ApprovedByUserID != "user_123" {
		t.Fatalf("ApprovedByUserID = %v, want user_123", got.Layer.ApprovedByUserID)
	}
}

func TestSemanticLayerControllerRejectsNonMembers(t *testing.T) {
	t.Parallel()

	ctrl := NewSemanticLayerController(
		fakeSemanticMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{}, repository.ErrNotFound
			},
		},
		fakeSemanticSchemaStore{
			latestFn: func(context.Context, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected LatestByTenant call")
			},
			getFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSchemaVersion, error) {
				return model.TenantSchemaVersion{}, errors.New("unexpected GetByTenantAndID call")
			},
		},
		fakeSemanticLayerStore{
			latestDraftFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestDraftBySchemaVersion call")
			},
			latestApprovedFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected LatestApprovedBySchemaVersion call")
			},
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected GetByID call")
			},
			createDraftFn: func(context.Context, uuid.UUID, uuid.UUID, []byte) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected CreateDraftVersion call")
			},
			approveFn: func(context.Context, uuid.UUID, uuid.UUID, time.Time, string) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("unexpected Approve call")
			},
		},
		&fakeSemanticCompleter{},
		SemanticLayerControllerConfig{},
	)

	_, err := ctrl.Get(context.Background(), uuid.New(), "user_123")
	if !errors.Is(err, ErrSemanticLayerAccessDenied) {
		t.Fatalf("err = %v, want ErrSemanticLayerAccessDenied", err)
	}
}
