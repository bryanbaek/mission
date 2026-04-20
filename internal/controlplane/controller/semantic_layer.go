package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

var (
	ErrSchemaVersionNotFound     = errors.New("schema version not found")
	ErrSemanticLayerNotFound     = errors.New("semantic layer not found")
	ErrSemanticLayerArchived     = errors.New("semantic layer is archived")
	ErrInvalidSemanticLayer      = errors.New("semantic layer content is invalid")
	ErrSemanticLayerAccessDenied = errors.New("not a member of this tenant")
)

type semanticLayerMembershipChecker interface {
	EnsureMembership(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (model.TenantUser, error)
}

type semanticLayerSchemaStore interface {
	LatestByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
	) (model.TenantSchemaVersion, error)
	GetByTenantAndID(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
	) (model.TenantSchemaVersion, error)
}

type semanticLayerStore interface {
	LatestDraftBySchemaVersion(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
	) (model.TenantSemanticLayer, error)
	LatestApprovedBySchemaVersion(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
	) (model.TenantSemanticLayer, error)
	GetByID(
		ctx context.Context,
		tenantID, id uuid.UUID,
	) (model.TenantSemanticLayer, error)
	ListApprovedHistoryByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
	) ([]model.TenantSemanticLayer, error)
	CreateDraftVersion(
		ctx context.Context,
		tenantID, schemaVersionID uuid.UUID,
		content []byte,
	) (model.TenantSemanticLayer, error)
	Approve(
		ctx context.Context,
		tenantID, id uuid.UUID,
		approvedAt time.Time,
		approvedByUserID string,
	) (model.TenantSemanticLayer, error)
}

type semanticLayerCompleter interface {
	Complete(
		ctx context.Context,
		req llm.CompletionRequest,
	) (llm.CompletionResponse, error)
}

type SemanticLayerControllerConfig struct {
	Now       func() time.Time
	Model     string
	MaxTokens int
}

type SemanticLayerRecord struct {
	Layer   model.TenantSemanticLayer
	Content model.SemanticLayerContent
}

type GetSemanticLayerResult struct {
	HasSchema        bool
	NeedsDraft       bool
	LatestSchema     *model.TenantSchemaVersion
	CurrentLayer     *SemanticLayerRecord
	ApprovedBaseline *SemanticLayerRecord
}

type DraftSemanticLayerResult struct {
	Layer SemanticLayerRecord
	Usage llm.Usage
}

type SemanticLayerController struct {
	tenants   semanticLayerMembershipChecker
	schemas   semanticLayerSchemaStore
	layers    semanticLayerStore
	completer semanticLayerCompleter
	now       func() time.Time
	model     string
	maxTokens int
}

func NewSemanticLayerController(
	tenants semanticLayerMembershipChecker,
	schemas semanticLayerSchemaStore,
	layers semanticLayerStore,
	completer semanticLayerCompleter,
	cfg SemanticLayerControllerConfig,
) *SemanticLayerController {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = "claude-sonnet-4-6"
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 16_384
	}
	return &SemanticLayerController{
		tenants:   tenants,
		schemas:   schemas,
		layers:    layers,
		completer: completer,
		now:       now,
		model:     modelName,
		maxTokens: maxTokens,
	}
}

func (c *SemanticLayerController) Get(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (GetSemanticLayerResult, error) {
	if err := c.ensureMembership(ctx, tenantID, clerkUserID); err != nil {
		return GetSemanticLayerResult{}, err
	}

	latestSchema, err := c.schemas.LatestByTenant(ctx, tenantID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return GetSemanticLayerResult{HasSchema: false}, nil
	case err != nil:
		return GetSemanticLayerResult{}, err
	}

	result := GetSemanticLayerResult{
		HasSchema:    true,
		LatestSchema: &latestSchema,
	}

	draft, draftErr := c.layers.LatestDraftBySchemaVersion(
		ctx,
		tenantID,
		latestSchema.ID,
	)
	if draftErr == nil {
		record, err := decodeSemanticLayerRecord(draft)
		if err != nil {
			return GetSemanticLayerResult{}, err
		}
		result.CurrentLayer = &record
	} else if !errors.Is(draftErr, repository.ErrNotFound) {
		return GetSemanticLayerResult{}, draftErr
	}

	approved, approvedErr := c.layers.LatestApprovedBySchemaVersion(
		ctx,
		tenantID,
		latestSchema.ID,
	)
	if approvedErr == nil {
		record, err := decodeSemanticLayerRecord(approved)
		if err != nil {
			return GetSemanticLayerResult{}, err
		}
		if result.CurrentLayer == nil {
			result.CurrentLayer = &record
		} else if result.CurrentLayer.Layer.ID != record.Layer.ID {
			result.ApprovedBaseline = &record
		}
	} else if !errors.Is(approvedErr, repository.ErrNotFound) {
		return GetSemanticLayerResult{}, approvedErr
	}

	if result.CurrentLayer == nil {
		result.NeedsDraft = true
	}

	return result, nil
}

func (c *SemanticLayerController) Draft(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	schemaVersionID uuid.UUID,
) (DraftSemanticLayerResult, error) {
	if err := c.ensureMembership(ctx, tenantID, clerkUserID); err != nil {
		return DraftSemanticLayerResult{}, err
	}

	schemaVersion, err := c.schemas.GetByTenantAndID(
		ctx,
		tenantID,
		schemaVersionID,
	)
	if errors.Is(err, repository.ErrNotFound) {
		return DraftSemanticLayerResult{}, ErrSchemaVersionNotFound
	}
	if err != nil {
		return DraftSemanticLayerResult{}, err
	}

	var schemaBlob model.SchemaBlob
	if err := json.Unmarshal(schemaVersion.Blob, &schemaBlob); err != nil {
		return DraftSemanticLayerResult{}, fmt.Errorf("unmarshal schema blob: %w", err)
	}

	completion, err := c.completer.Complete(ctx, llm.CompletionRequest{
		System: semanticLayerSystemPrompt,
		Messages: []llm.Message{{
			Role: "user",
			Content: buildSemanticLayerUserPrompt(
				schemaVersion.Blob,
			),
		}},
		Model:     c.model,
		MaxTokens: c.maxTokens,
		OutputFormat: &llm.OutputFormat{
			Name:   "semantic_layer_content",
			Schema: semanticLayerOutputSchema(),
		},
		CacheControl: &llm.CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		},
	})
	if err != nil {
		return DraftSemanticLayerResult{}, err
	}

	var generated model.SemanticLayerContent
	if err := json.Unmarshal([]byte(completion.Content), &generated); err != nil {
		return DraftSemanticLayerResult{}, fmt.Errorf("decode semantic layer output: %w", err)
	}

	content := hydrateSemanticLayerFromSchema(schemaBlob, generated)
	if err := validateSemanticLayerContent(content); err != nil {
		return DraftSemanticLayerResult{}, err
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return DraftSemanticLayerResult{}, fmt.Errorf("marshal semantic layer content: %w", err)
	}

	layer, err := c.layers.CreateDraftVersion(
		ctx,
		tenantID,
		schemaVersionID,
		contentJSON,
	)
	if err != nil {
		return DraftSemanticLayerResult{}, err
	}

	return DraftSemanticLayerResult{
		Layer: SemanticLayerRecord{
			Layer:   layer,
			Content: content,
		},
		Usage: completion.Usage,
	}, nil
}

func (c *SemanticLayerController) Update(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	id uuid.UUID,
	content model.SemanticLayerContent,
) (SemanticLayerRecord, error) {
	if err := c.ensureMembership(ctx, tenantID, clerkUserID); err != nil {
		return SemanticLayerRecord{}, err
	}

	baseLayer, err := c.layers.GetByID(ctx, tenantID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return SemanticLayerRecord{}, ErrSemanticLayerNotFound
	}
	if err != nil {
		return SemanticLayerRecord{}, err
	}
	if baseLayer.Status == model.SemanticLayerStatusArchived {
		return SemanticLayerRecord{}, ErrSemanticLayerArchived
	}

	baseRecord, err := decodeSemanticLayerRecord(baseLayer)
	if err != nil {
		return SemanticLayerRecord{}, err
	}

	merged := mergeEditableDescriptions(baseRecord.Content, content)
	if err := validateSemanticLayerContent(merged); err != nil {
		return SemanticLayerRecord{}, err
	}

	contentJSON, err := json.Marshal(merged)
	if err != nil {
		return SemanticLayerRecord{}, fmt.Errorf("marshal semantic layer content: %w", err)
	}

	updatedLayer, err := c.layers.CreateDraftVersion(
		ctx,
		tenantID,
		baseLayer.SchemaVersionID,
		contentJSON,
	)
	if err != nil {
		return SemanticLayerRecord{}, err
	}

	return SemanticLayerRecord{
		Layer:   updatedLayer,
		Content: merged,
	}, nil
}

func (c *SemanticLayerController) Approve(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	id uuid.UUID,
) (SemanticLayerRecord, error) {
	if err := c.ensureMembership(ctx, tenantID, clerkUserID); err != nil {
		return SemanticLayerRecord{}, err
	}

	layer, err := c.layers.GetByID(ctx, tenantID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return SemanticLayerRecord{}, ErrSemanticLayerNotFound
	}
	if err != nil {
		return SemanticLayerRecord{}, err
	}
	if layer.Status == model.SemanticLayerStatusArchived {
		return SemanticLayerRecord{}, ErrSemanticLayerArchived
	}
	if layer.Status == model.SemanticLayerStatusApproved {
		return decodeSemanticLayerRecord(layer)
	}

	approvedLayer, err := c.layers.Approve(
		ctx,
		tenantID,
		id,
		c.now().UTC(),
		clerkUserID,
	)
	if err != nil {
		return SemanticLayerRecord{}, err
	}
	return decodeSemanticLayerRecord(approvedLayer)
}

func (c *SemanticLayerController) ensureMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) error {
	_, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrSemanticLayerAccessDenied
	}
	return err
}

func decodeSemanticLayerRecord(
	layer model.TenantSemanticLayer,
) (SemanticLayerRecord, error) {
	var content model.SemanticLayerContent
	if err := json.Unmarshal(layer.Content, &content); err != nil {
		return SemanticLayerRecord{}, fmt.Errorf("unmarshal semantic layer content: %w", err)
	}
	if err := validateSemanticLayerContent(content); err != nil {
		return SemanticLayerRecord{}, err
	}
	return SemanticLayerRecord{
		Layer:   layer,
		Content: content,
	}, nil
}

func validateSemanticLayerContent(content model.SemanticLayerContent) error {
	if len(content.Tables) == 0 {
		return fmt.Errorf("%w: at least one table is required", ErrInvalidSemanticLayer)
	}
	for _, table := range content.Tables {
		if strings.TrimSpace(table.TableSchema) == "" ||
			strings.TrimSpace(table.TableName) == "" {
			return fmt.Errorf("%w: table identifiers are required", ErrInvalidSemanticLayer)
		}
		if len(table.Columns) == 0 {
			return fmt.Errorf("%w: table %s.%s must include columns", ErrInvalidSemanticLayer, table.TableSchema, table.TableName)
		}
		for _, column := range table.Columns {
			if strings.TrimSpace(column.ColumnName) == "" {
				return fmt.Errorf("%w: column name is required", ErrInvalidSemanticLayer)
			}
		}
	}
	return nil
}

func hydrateSemanticLayerFromSchema(
	schema model.SchemaBlob,
	generated model.SemanticLayerContent,
) model.SemanticLayerContent {
	tableDescriptions := make(map[string]string, len(generated.Tables))
	columnDescriptions := make(map[string]string)
	for _, table := range generated.Tables {
		tableKey := fqTable(table.TableSchema, table.TableName)
		tableDescriptions[tableKey] = strings.TrimSpace(table.Description)
		for _, column := range table.Columns {
			columnKey := fqColumn(
				column.TableSchema,
				column.TableName,
				column.ColumnName,
			)
			columnDescriptions[columnKey] = strings.TrimSpace(column.Description)
		}
	}

	columnsByTable := make(map[string][]model.SemanticColumn)
	for _, column := range schema.Columns {
		columnKey := fqColumn(
			column.TableSchema,
			column.TableName,
			column.ColumnName,
		)
		tableKey := fqTable(column.TableSchema, column.TableName)
		columnsByTable[tableKey] = append(
			columnsByTable[tableKey],
			model.SemanticColumn{
				TableSchema:     column.TableSchema,
				TableName:       column.TableName,
				ColumnName:      column.ColumnName,
				OrdinalPosition: column.OrdinalPosition,
				DataType:        column.DataType,
				ColumnType:      column.ColumnType,
				IsNullable:      column.IsNullable,
				ColumnComment:   column.ColumnComment,
				Description:     columnDescriptions[columnKey],
			},
		)
	}

	out := model.SemanticLayerContent{
		Tables:           make([]model.SemanticTable, 0, len(schema.Tables)),
		Entities:         normalizeEntities(generated.Entities),
		CandidateMetrics: normalizeMetrics(generated.CandidateMetrics),
	}
	for _, table := range schema.Tables {
		tableKey := fqTable(table.TableSchema, table.TableName)
		out.Tables = append(out.Tables, model.SemanticTable{
			TableSchema:  table.TableSchema,
			TableName:    table.TableName,
			TableType:    table.TableType,
			TableComment: table.TableComment,
			Description:  tableDescriptions[tableKey],
			Columns:      columnsByTable[tableKey],
		})
	}
	return out
}

func mergeEditableDescriptions(
	base model.SemanticLayerContent,
	edited model.SemanticLayerContent,
) model.SemanticLayerContent {
	tableDescriptions := make(map[string]string, len(edited.Tables))
	columnDescriptions := make(map[string]string)
	for _, table := range edited.Tables {
		tableDescriptions[fqTable(table.TableSchema, table.TableName)] = table.Description
		for _, column := range table.Columns {
			columnDescriptions[fqColumn(
				column.TableSchema,
				column.TableName,
				column.ColumnName,
			)] = column.Description
		}
	}

	out := model.SemanticLayerContent{
		Tables:           make([]model.SemanticTable, 0, len(base.Tables)),
		Entities:         append([]model.SemanticEntity(nil), base.Entities...),
		CandidateMetrics: append([]model.CandidateMetric(nil), base.CandidateMetrics...),
	}
	for _, table := range base.Tables {
		nextTable := table
		if description, ok := tableDescriptions[fqTable(table.TableSchema, table.TableName)]; ok {
			nextTable.Description = description
		}
		nextTable.Columns = make([]model.SemanticColumn, len(table.Columns))
		for i, column := range table.Columns {
			nextColumn := column
			if description, ok := columnDescriptions[fqColumn(
				column.TableSchema,
				column.TableName,
				column.ColumnName,
			)]; ok {
				nextColumn.Description = description
			}
			nextTable.Columns[i] = nextColumn
		}
		out.Tables = append(out.Tables, nextTable)
	}
	return out
}

func normalizeEntities(entities []model.SemanticEntity) []model.SemanticEntity {
	out := make([]model.SemanticEntity, 0, len(entities))
	for _, entity := range entities {
		if strings.TrimSpace(entity.Name) == "" {
			continue
		}
		next := entity
		next.Name = strings.TrimSpace(next.Name)
		next.Description = strings.TrimSpace(next.Description)
		next.SourceTables = normalizeStrings(next.SourceTables)
		out = append(out, next)
	}
	return out
}

func normalizeMetrics(metrics []model.CandidateMetric) []model.CandidateMetric {
	out := make([]model.CandidateMetric, 0, len(metrics))
	for _, metric := range metrics {
		if strings.TrimSpace(metric.Name) == "" {
			continue
		}
		next := metric
		next.Name = strings.TrimSpace(next.Name)
		next.Description = strings.TrimSpace(next.Description)
		next.SourceTables = normalizeStrings(next.SourceTables)
		out = append(out, next)
	}
	return out
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func fqTable(schema, table string) string {
	return schema + "." + table
}

func fqColumn(schema, table, column string) string {
	return schema + "." + table + "." + column
}

func buildSemanticLayerUserPrompt(schemaBlob []byte) string {
	return strings.TrimSpace(`
다음은 테넌트의 MySQL 스키마 JSON입니다.
이 JSON만 근거로 semantic layer 초안을 작성하세요.

요구사항:
- 모든 설명은 한국어로 작성합니다.
- 확신이 낮으면 설명 앞에 [추정]을 붙입니다.
- 입력 스키마에 있는 모든 테이블과 컬럼을 빠짐없이 포함합니다.
- source_tables는 "schema.table" 형식의 문자열 배열로 작성합니다.
- 추가 설명문이나 마크다운을 붙이지 말고 구조화된 데이터만 반환합니다.

스키마 JSON:
`) + "\n" + string(schemaBlob)
}

const semanticLayerSystemPrompt = `
당신은 한국어로 일하는 매우 보수적인 데이터 분석가입니다.
목표는 MySQL 스키마만 보고 semantic layer 초안을 작성하는 것입니다.

반드시 지킬 규칙:
- 설명은 과장하지 말고 실제 스키마에서 합리적으로 추론 가능한 범위만 작성합니다.
- 추정이 섞이면 설명 앞에 [추정]을 붙입니다.
- 테이블/컬럼 식별자와 메타데이터는 입력 스키마와 정확히 일치해야 합니다.
- business glossary, join recommendation, stored procedure 해석은 만들지 않습니다.
- 결과는 구조화된 데이터만 반환합니다.

논리적 구조는 다음 YAML과 같습니다:
tables:
  - table_schema: "schema"
    table_name: "table"
    table_type: "BASE TABLE"
    table_comment: "원본 코멘트"
    description: "한국어 설명"
    columns:
      - table_schema: "schema"
        table_name: "table"
        column_name: "column"
        ordinal_position: 1
        data_type: "varchar"
        column_type: "varchar(255)"
        is_nullable: false
        column_comment: "원본 코멘트"
        description: "한국어 설명"
entities:
  - name: "핵심 엔터티 이름"
    description: "한국어 설명"
    source_tables: ["schema.table"]
candidate_metrics:
  - name: "후보 지표 이름"
    description: "한국어 설명"
    source_tables: ["schema.table"]
`

func semanticLayerOutputSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	tableRefArray := map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
	columnSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"table_schema":     stringSchema,
			"table_name":       stringSchema,
			"column_name":      stringSchema,
			"ordinal_position": map[string]any{"type": "integer"},
			"data_type":        stringSchema,
			"column_type":      stringSchema,
			"is_nullable":      map[string]any{"type": "boolean"},
			"column_comment":   stringSchema,
			"description":      stringSchema,
		},
		"required": []string{
			"table_schema",
			"table_name",
			"column_name",
			"ordinal_position",
			"data_type",
			"column_type",
			"is_nullable",
			"column_comment",
			"description",
		},
		"additionalProperties": false,
	}
	tableSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"table_schema":  stringSchema,
			"table_name":    stringSchema,
			"table_type":    stringSchema,
			"table_comment": stringSchema,
			"description":   stringSchema,
			"columns": map[string]any{
				"type":  "array",
				"items": columnSchema,
			},
		},
		"required": []string{
			"table_schema",
			"table_name",
			"table_type",
			"table_comment",
			"description",
			"columns",
		},
		"additionalProperties": false,
	}
	entitySchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":          stringSchema,
			"description":   stringSchema,
			"source_tables": tableRefArray,
		},
		"required":             []string{"name", "description", "source_tables"},
		"additionalProperties": false,
	}
	metricSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":          stringSchema,
			"description":   stringSchema,
			"source_tables": tableRefArray,
		},
		"required":             []string{"name", "description", "source_tables"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tables": map[string]any{
				"type":  "array",
				"items": tableSchema,
			},
			"entities": map[string]any{
				"type":  "array",
				"items": entitySchema,
			},
			"candidate_metrics": map[string]any{
				"type":  "array",
				"items": metricSchema,
			},
		},
		"required":             []string{"tables", "entities", "candidate_metrics"},
		"additionalProperties": false,
	}
}
