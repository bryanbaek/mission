package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SemanticLayerStatus string

const (
	SemanticLayerStatusDraft    SemanticLayerStatus = "draft"
	SemanticLayerStatusApproved SemanticLayerStatus = "approved"
	SemanticLayerStatusArchived SemanticLayerStatus = "archived"
)

type SemanticColumn struct {
	TableSchema     string `json:"table_schema" yaml:"table_schema"`
	TableName       string `json:"table_name" yaml:"table_name"`
	ColumnName      string `json:"column_name" yaml:"column_name"`
	OrdinalPosition int32  `json:"ordinal_position" yaml:"ordinal_position"`
	DataType        string `json:"data_type" yaml:"data_type"`
	ColumnType      string `json:"column_type" yaml:"column_type"`
	IsNullable      bool   `json:"is_nullable" yaml:"is_nullable"`
	ColumnComment   string `json:"column_comment" yaml:"column_comment"`
	Description     string `json:"description" yaml:"description"`
}

type SemanticTable struct {
	TableSchema  string           `json:"table_schema" yaml:"table_schema"`
	TableName    string           `json:"table_name" yaml:"table_name"`
	TableType    string           `json:"table_type" yaml:"table_type"`
	TableComment string           `json:"table_comment" yaml:"table_comment"`
	Description  string           `json:"description" yaml:"description"`
	Columns      []SemanticColumn `json:"columns" yaml:"columns"`
}

type SemanticEntity struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	SourceTables []string `json:"source_tables" yaml:"source_tables"`
}

type CandidateMetric struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	SourceTables []string `json:"source_tables" yaml:"source_tables"`
}

type SemanticLayerContent struct {
	Tables           []SemanticTable   `json:"tables" yaml:"tables"`
	Entities         []SemanticEntity  `json:"entities" yaml:"entities"`
	CandidateMetrics []CandidateMetric `json:"candidate_metrics" yaml:"candidate_metrics"`
}

type TenantSemanticLayer struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SchemaVersionID  uuid.UUID
	Status           SemanticLayerStatus
	Content          json.RawMessage
	CreatedAt        time.Time
	ApprovedAt       *time.Time
	ApprovedByUserID *string
}
