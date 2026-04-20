package handler

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	semanticv1 "github.com/bryanbaek/mission/gen/go/semantic/v1"
	"github.com/bryanbaek/mission/gen/go/semantic/v1/semanticv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type semanticLayerController interface {
	Get(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (controller.GetSemanticLayerResult, error)
	Draft(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		schemaVersionID uuid.UUID,
	) (controller.DraftSemanticLayerResult, error)
	Update(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		id uuid.UUID,
		content model.SemanticLayerContent,
	) (controller.SemanticLayerRecord, error)
	Approve(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
		id uuid.UUID,
	) (controller.SemanticLayerRecord, error)
}

type SemanticLayerHandler struct {
	semanticv1connect.UnimplementedSemanticLayerServiceHandler
	ctrl semanticLayerController
}

func NewSemanticLayerHandler(
	ctrl semanticLayerController,
) *SemanticLayerHandler {
	return &SemanticLayerHandler{ctrl: ctrl}
}

func (h *SemanticLayerHandler) GetSemanticLayer(
	ctx context.Context,
	req *connect.Request[semanticv1.GetSemanticLayerRequest],
) (*connect.Response[semanticv1.GetSemanticLayerResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}

	result, err := h.ctrl.Get(ctx, tenantID, user.ID)
	if err != nil {
		return nil, semanticLayerError(err)
	}

	resp := &semanticv1.GetSemanticLayerResponse{
		HasSchema:  result.HasSchema,
		NeedsDraft: result.NeedsDraft,
	}
	if result.LatestSchema != nil {
		resp.LatestSchema = schemaVersionToProto(*result.LatestSchema)
	}
	if result.CurrentLayer != nil {
		resp.CurrentLayer = semanticLayerToProto(*result.CurrentLayer)
	}
	if result.ApprovedBaseline != nil {
		resp.ApprovedBaseline = semanticLayerToProto(*result.ApprovedBaseline)
	}

	return connect.NewResponse(resp), nil
}

func (h *SemanticLayerHandler) DraftSemanticLayer(
	ctx context.Context,
	req *connect.Request[semanticv1.DraftSemanticLayerRequest],
) (*connect.Response[semanticv1.DraftSemanticLayerResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}
	schemaVersionID, err := uuid.Parse(req.Msg.SchemaVersionId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid schema_version_id"),
		)
	}

	result, err := h.ctrl.Draft(
		ctx,
		tenantID,
		user.ID,
		schemaVersionID,
	)
	if err != nil {
		return nil, semanticLayerError(err)
	}

	return connect.NewResponse(&semanticv1.DraftSemanticLayerResponse{
		Layer: semanticLayerToProto(result.Layer),
		Usage: &semanticv1.CompletionUsage{
			Provider:                 result.Usage.Provider,
			Model:                    result.Usage.Model,
			InputTokens:              int32(result.Usage.InputTokens),
			OutputTokens:             int32(result.Usage.OutputTokens),
			CacheCreationInputTokens: int32(result.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int32(result.Usage.CacheReadInputTokens),
		},
	}), nil
}

func (h *SemanticLayerHandler) UpdateSemanticLayer(
	ctx context.Context,
	req *connect.Request[semanticv1.UpdateSemanticLayerRequest],
) (*connect.Response[semanticv1.UpdateSemanticLayerResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}
	layerID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid semantic layer id"),
		)
	}

	result, err := h.ctrl.Update(
		ctx,
		tenantID,
		user.ID,
		layerID,
		contentFromProto(req.Msg.Content),
	)
	if err != nil {
		return nil, semanticLayerError(err)
	}

	return connect.NewResponse(&semanticv1.UpdateSemanticLayerResponse{
		Layer: semanticLayerToProto(result),
	}), nil
}

func (h *SemanticLayerHandler) ApproveSemanticLayer(
	ctx context.Context,
	req *connect.Request[semanticv1.ApproveSemanticLayerRequest],
) (*connect.Response[semanticv1.ApproveSemanticLayerResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}
	layerID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid semantic layer id"),
		)
	}

	result, err := h.ctrl.Approve(ctx, tenantID, user.ID, layerID)
	if err != nil {
		return nil, semanticLayerError(err)
	}

	return connect.NewResponse(&semanticv1.ApproveSemanticLayerResponse{
		Layer: semanticLayerToProto(result),
	}), nil
}

func semanticLayerError(err error) error {
	switch {
	case errors.Is(err, controller.ErrSemanticLayerAccessDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, controller.ErrSchemaVersionNotFound),
		errors.Is(err, controller.ErrSemanticLayerNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, controller.ErrInvalidSemanticLayer):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, controller.ErrSemanticLayerArchived):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func schemaVersionToProto(
	schema model.TenantSchemaVersion,
) *semanticv1.SchemaVersionSummary {
	return &semanticv1.SchemaVersionSummary{
		Id:           schema.ID.String(),
		CapturedAt:   timestamppb.New(schema.CapturedAt),
		SchemaHash:   schema.SchemaHash,
		DatabaseName: databaseNameFromSchemaBlob(schema.Blob),
	}
}

func semanticLayerToProto(
	record controller.SemanticLayerRecord,
) *semanticv1.SemanticLayer {
	out := &semanticv1.SemanticLayer{
		Id:              record.Layer.ID.String(),
		TenantId:        record.Layer.TenantID.String(),
		SchemaVersionId: record.Layer.SchemaVersionID.String(),
		Status:          semanticLayerStatusToProto(record.Layer.Status),
		Content:         contentToProto(record.Content),
		CreatedAt:       timestamppb.New(record.Layer.CreatedAt),
	}
	if record.Layer.ApprovedAt != nil {
		out.ApprovedAt = timestamppb.New(*record.Layer.ApprovedAt)
	}
	if record.Layer.ApprovedByUserID != nil {
		out.ApprovedByUserId = *record.Layer.ApprovedByUserID
	}
	return out
}

func semanticLayerStatusToProto(
	status model.SemanticLayerStatus,
) semanticv1.SemanticLayerStatus {
	switch status {
	case model.SemanticLayerStatusDraft:
		return semanticv1.SemanticLayerStatus_SEMANTIC_LAYER_STATUS_DRAFT
	case model.SemanticLayerStatusApproved:
		return semanticv1.SemanticLayerStatus_SEMANTIC_LAYER_STATUS_APPROVED
	case model.SemanticLayerStatusArchived:
		return semanticv1.SemanticLayerStatus_SEMANTIC_LAYER_STATUS_ARCHIVED
	default:
		return semanticv1.SemanticLayerStatus_SEMANTIC_LAYER_STATUS_UNSPECIFIED
	}
}

func contentToProto(
	content model.SemanticLayerContent,
) *semanticv1.SemanticLayerContent {
	out := &semanticv1.SemanticLayerContent{
		Tables:           make([]*semanticv1.SemanticTable, 0, len(content.Tables)),
		Entities:         make([]*semanticv1.SemanticEntity, 0, len(content.Entities)),
		CandidateMetrics: make([]*semanticv1.CandidateMetric, 0, len(content.CandidateMetrics)),
	}
	for _, table := range content.Tables {
		tableOut := &semanticv1.SemanticTable{
			TableSchema:  table.TableSchema,
			TableName:    table.TableName,
			TableType:    table.TableType,
			TableComment: table.TableComment,
			Description:  table.Description,
			Columns:      make([]*semanticv1.SemanticColumn, 0, len(table.Columns)),
		}
		for _, column := range table.Columns {
			tableOut.Columns = append(tableOut.Columns, &semanticv1.SemanticColumn{
				TableSchema:     column.TableSchema,
				TableName:       column.TableName,
				ColumnName:      column.ColumnName,
				OrdinalPosition: column.OrdinalPosition,
				DataType:        column.DataType,
				ColumnType:      column.ColumnType,
				IsNullable:      column.IsNullable,
				ColumnComment:   column.ColumnComment,
				Description:     column.Description,
			})
		}
		out.Tables = append(out.Tables, tableOut)
	}
	for _, entity := range content.Entities {
		out.Entities = append(out.Entities, &semanticv1.SemanticEntity{
			Name:         entity.Name,
			Description:  entity.Description,
			SourceTables: append([]string(nil), entity.SourceTables...),
		})
	}
	for _, metric := range content.CandidateMetrics {
		out.CandidateMetrics = append(out.CandidateMetrics, &semanticv1.CandidateMetric{
			Name:         metric.Name,
			Description:  metric.Description,
			SourceTables: append([]string(nil), metric.SourceTables...),
		})
	}
	return out
}

func contentFromProto(
	content *semanticv1.SemanticLayerContent,
) model.SemanticLayerContent {
	if content == nil {
		return model.SemanticLayerContent{}
	}
	out := model.SemanticLayerContent{
		Tables:           make([]model.SemanticTable, 0, len(content.Tables)),
		Entities:         make([]model.SemanticEntity, 0, len(content.Entities)),
		CandidateMetrics: make([]model.CandidateMetric, 0, len(content.CandidateMetrics)),
	}
	for _, table := range content.Tables {
		nextTable := model.SemanticTable{
			TableSchema:  table.GetTableSchema(),
			TableName:    table.GetTableName(),
			TableType:    table.GetTableType(),
			TableComment: table.GetTableComment(),
			Description:  table.GetDescription(),
			Columns:      make([]model.SemanticColumn, 0, len(table.GetColumns())),
		}
		for _, column := range table.GetColumns() {
			nextTable.Columns = append(nextTable.Columns, model.SemanticColumn{
				TableSchema:     column.GetTableSchema(),
				TableName:       column.GetTableName(),
				ColumnName:      column.GetColumnName(),
				OrdinalPosition: column.GetOrdinalPosition(),
				DataType:        column.GetDataType(),
				ColumnType:      column.GetColumnType(),
				IsNullable:      column.GetIsNullable(),
				ColumnComment:   column.GetColumnComment(),
				Description:     column.GetDescription(),
			})
		}
		out.Tables = append(out.Tables, nextTable)
	}
	for _, entity := range content.GetEntities() {
		out.Entities = append(out.Entities, model.SemanticEntity{
			Name:         entity.GetName(),
			Description:  entity.GetDescription(),
			SourceTables: append([]string(nil), entity.GetSourceTables()...),
		})
	}
	for _, metric := range content.GetCandidateMetrics() {
		out.CandidateMetrics = append(out.CandidateMetrics, model.CandidateMetric{
			Name:         metric.GetName(),
			Description:  metric.GetDescription(),
			SourceTables: append([]string(nil), metric.GetSourceTables()...),
		})
	}
	return out
}

func databaseNameFromSchemaBlob(blob []byte) string {
	var schema model.SchemaBlob
	if err := json.Unmarshal(blob, &schema); err != nil {
		return ""
	}
	return schema.DatabaseName
}
