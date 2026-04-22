package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	semanticv1 "github.com/bryanbaek/mission/gen/go/semantic/v1"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeSemanticLayerController struct {
	getFn     func(context.Context, uuid.UUID, string) (controller.GetSemanticLayerResult, error)
	draftFn   func(context.Context, uuid.UUID, string, uuid.UUID) (controller.DraftSemanticLayerResult, error)
	updateFn  func(context.Context, uuid.UUID, string, uuid.UUID, model.SemanticLayerContent) (controller.SemanticLayerRecord, error)
	approveFn func(context.Context, uuid.UUID, string, uuid.UUID) (controller.SemanticLayerRecord, error)
}

func (f fakeSemanticLayerController) Get(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (controller.GetSemanticLayerResult, error) {
	return f.getFn(ctx, tenantID, clerkUserID)
}

func (f fakeSemanticLayerController) Draft(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	schemaVersionID uuid.UUID,
) (controller.DraftSemanticLayerResult, error) {
	return f.draftFn(ctx, tenantID, clerkUserID, schemaVersionID)
}

func (f fakeSemanticLayerController) Update(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	id uuid.UUID,
	content model.SemanticLayerContent,
) (controller.SemanticLayerRecord, error) {
	return f.updateFn(ctx, tenantID, clerkUserID, id, content)
}

func (f fakeSemanticLayerController) Approve(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	id uuid.UUID,
) (controller.SemanticLayerRecord, error) {
	return f.approveFn(ctx, tenantID, clerkUserID, id)
}

func semanticHandlerContext() context.Context {
	return auth.WithUser(context.Background(), auth.User{ID: "user_123"})
}

func semanticHandlerContent() model.SemanticLayerContent {
	return model.SemanticLayerContent{
		Tables: []model.SemanticTable{{
			TableSchema:  "mission_app",
			TableName:    "customers",
			TableType:    "BASE TABLE",
			TableComment: "Customer master data",
			Description:  "고객 기본 정보",
			Columns: []model.SemanticColumn{{
				TableSchema:     "mission_app",
				TableName:       "customers",
				ColumnName:      "customer_code",
				OrdinalPosition: 1,
				DataType:        "varchar",
				ColumnType:      "varchar(64)",
				IsNullable:      false,
				ColumnComment:   "External customer code",
				Description:     "고객 코드",
			}},
		}},
		Entities: []model.SemanticEntity{{
			Name:         "고객",
			Description:  "핵심 고객 엔터티",
			SourceTables: []string{"mission_app.customers"},
		}},
		CandidateMetrics: []model.CandidateMetric{{
			Name:         "고객 수",
			Description:  "전체 고객 건수",
			SourceTables: []string{"mission_app.customers"},
		}},
	}
}

func semanticHandlerRecord(
	layerID, tenantID, schemaVersionID uuid.UUID,
	status model.SemanticLayerStatus,
) controller.SemanticLayerRecord {
	createdAt := time.Unix(1_700_000_000, 0).UTC()
	return controller.SemanticLayerRecord{
		Layer: model.TenantSemanticLayer{
			ID:              layerID,
			TenantID:        tenantID,
			SchemaVersionID: schemaVersionID,
			Status:          status,
			CreatedAt:       createdAt,
		},
		Content: semanticHandlerContent(),
	}
}

func TestSemanticLayerHandlerValidationAndErrorMapping(t *testing.T) {
	t.Parallel()

	handler := NewSemanticLayerHandler(fakeSemanticLayerController{
		getFn: func(context.Context, uuid.UUID, string) (controller.GetSemanticLayerResult, error) {
			return controller.GetSemanticLayerResult{}, controller.ErrSemanticLayerAccessDenied
		},
		draftFn: func(context.Context, uuid.UUID, string, uuid.UUID) (controller.DraftSemanticLayerResult, error) {
			return controller.DraftSemanticLayerResult{}, controller.ErrSchemaVersionNotFound
		},
		updateFn: func(context.Context, uuid.UUID, string, uuid.UUID, model.SemanticLayerContent) (controller.SemanticLayerRecord, error) {
			return controller.SemanticLayerRecord{}, controller.ErrInvalidSemanticLayer
		},
		approveFn: func(context.Context, uuid.UUID, string, uuid.UUID) (controller.SemanticLayerRecord, error) {
			return controller.SemanticLayerRecord{}, controller.ErrSemanticLayerArchived
		},
	})

	_, err := handler.GetSemanticLayer(
		context.Background(),
		connect.NewRequest(&semanticv1.GetSemanticLayerRequest{
			TenantId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodeUnauthenticated)

	_, err = handler.GetSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.GetSemanticLayerRequest{
			TenantId: "bad",
		}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = handler.GetSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.GetSemanticLayerRequest{
			TenantId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodePermissionDenied)

	_, err = handler.DraftSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.DraftSemanticLayerRequest{
			TenantId:        uuid.NewString(),
			SchemaVersionId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodeNotFound)

	_, err = handler.UpdateSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.UpdateSemanticLayerRequest{
			TenantId: uuid.NewString(),
			Id:       uuid.NewString(),
			Content:  &semanticv1.SemanticLayerContent{},
		}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = handler.ApproveSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.ApproveSemanticLayerRequest{
			TenantId: uuid.NewString(),
			Id:       uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodeFailedPrecondition)
}

func TestSemanticLayerHandlerDraftResponseIncludesUsageAndKoreanContent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	schemaVersionID := uuid.New()
	layerID := uuid.New()

	handler := NewSemanticLayerHandler(fakeSemanticLayerController{
		getFn: func(context.Context, uuid.UUID, string) (controller.GetSemanticLayerResult, error) {
			return controller.GetSemanticLayerResult{}, errors.New("unexpected Get call")
		},
		draftFn: func(_ context.Context, gotTenantID uuid.UUID, clerkUserID string, gotSchemaVersionID uuid.UUID) (controller.DraftSemanticLayerResult, error) {
			if gotTenantID != tenantID || gotSchemaVersionID != schemaVersionID {
				t.Fatalf("got IDs = (%s, %s), want (%s, %s)", gotTenantID, gotSchemaVersionID, tenantID, schemaVersionID)
			}
			if clerkUserID != "user_123" {
				t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
			}
			return controller.DraftSemanticLayerResult{
				Layer: semanticHandlerRecord(
					layerID,
					tenantID,
					schemaVersionID,
					model.SemanticLayerStatusDraft,
				),
				Usage: llm.Usage{
					Provider:                 "anthropic",
					Model:                    "claude-sonnet-4-6",
					InputTokens:              300,
					OutputTokens:             120,
					CacheCreationInputTokens: 300,
					CacheReadInputTokens:     44,
				},
			}, nil
		},
		updateFn: func(context.Context, uuid.UUID, string, uuid.UUID, model.SemanticLayerContent) (controller.SemanticLayerRecord, error) {
			return controller.SemanticLayerRecord{}, errors.New("unexpected Update call")
		},
		approveFn: func(context.Context, uuid.UUID, string, uuid.UUID) (controller.SemanticLayerRecord, error) {
			return controller.SemanticLayerRecord{}, errors.New("unexpected Approve call")
		},
	})

	resp, err := handler.DraftSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.DraftSemanticLayerRequest{
			TenantId:        tenantID.String(),
			SchemaVersionId: schemaVersionID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("DraftSemanticLayer returned error: %v", err)
	}
	if resp.Msg.Layer == nil {
		t.Fatal("expected layer in response")
	}
	if resp.Msg.Layer.Id != layerID.String() {
		t.Fatalf("layer id = %q, want %s", resp.Msg.Layer.Id, layerID)
	}
	if resp.Msg.Layer.Content.GetTables()[0].GetDescription() != "고객 기본 정보" {
		t.Fatalf("table description = %q, want Korean content", resp.Msg.Layer.Content.GetTables()[0].GetDescription())
	}
	if resp.Msg.Usage == nil || resp.Msg.Usage.CacheReadInputTokens != 44 {
		t.Fatalf("usage = %+v, want cache_read_input_tokens=44", resp.Msg.Usage)
	}
	if resp.Msg.Usage.Provider != "anthropic" || resp.Msg.Usage.Model != "claude-sonnet-4-6" {
		t.Fatalf("usage provider/model = %+v", resp.Msg.Usage)
	}
}

func TestSemanticLayerHandlerMapsLLMUnavailableToUnavailable(t *testing.T) {
	t.Parallel()

	handler := NewSemanticLayerHandler(fakeSemanticLayerController{
		getFn: func(context.Context, uuid.UUID, string) (controller.GetSemanticLayerResult, error) {
			return controller.GetSemanticLayerResult{}, errors.New("unexpected Get call")
		},
		draftFn: func(context.Context, uuid.UUID, string, uuid.UUID) (controller.DraftSemanticLayerResult, error) {
			return controller.DraftSemanticLayerResult{}, llm.NewUnavailableError(
				[]string{"anthropic"},
				errors.New("provider outage"),
			)
		},
		updateFn: func(context.Context, uuid.UUID, string, uuid.UUID, model.SemanticLayerContent) (controller.SemanticLayerRecord, error) {
			return controller.SemanticLayerRecord{}, errors.New("unexpected Update call")
		},
		approveFn: func(context.Context, uuid.UUID, string, uuid.UUID) (controller.SemanticLayerRecord, error) {
			return controller.SemanticLayerRecord{}, errors.New("unexpected Approve call")
		},
	})

	_, err := handler.DraftSemanticLayer(
		semanticHandlerContext(),
		connect.NewRequest(&semanticv1.DraftSemanticLayerRequest{
			TenantId:        uuid.NewString(),
			SchemaVersionId: uuid.NewString(),
		}),
	)
	requireConnectCode(t, err, connect.CodeUnavailable)
}
