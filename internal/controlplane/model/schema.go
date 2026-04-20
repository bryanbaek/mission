package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SchemaBlob struct {
	DatabaseName string             `json:"database_name"`
	Tables       []SchemaTable      `json:"tables"`
	Columns      []SchemaColumn     `json:"columns"`
	PrimaryKeys  []SchemaPrimaryKey `json:"primary_keys"`
	ForeignKeys  []SchemaForeignKey `json:"foreign_keys"`
}

type SchemaTable struct {
	TableSchema  string `json:"table_schema"`
	TableName    string `json:"table_name"`
	TableType    string `json:"table_type"`
	TableComment string `json:"table_comment"`
}

type SchemaColumn struct {
	TableSchema     string `json:"table_schema"`
	TableName       string `json:"table_name"`
	ColumnName      string `json:"column_name"`
	OrdinalPosition int32  `json:"ordinal_position"`
	DataType        string `json:"data_type"`
	ColumnType      string `json:"column_type"`
	IsNullable      bool   `json:"is_nullable"`
	HasDefault      bool   `json:"has_default"`
	DefaultValue    string `json:"default_value"`
	ColumnComment   string `json:"column_comment"`
}

type SchemaPrimaryKey struct {
	TableSchema     string `json:"table_schema"`
	TableName       string `json:"table_name"`
	ConstraintName  string `json:"constraint_name"`
	ColumnName      string `json:"column_name"`
	OrdinalPosition int32  `json:"ordinal_position"`
}

type SchemaForeignKey struct {
	TableSchema           string `json:"table_schema"`
	TableName             string `json:"table_name"`
	ConstraintName        string `json:"constraint_name"`
	ColumnName            string `json:"column_name"`
	OrdinalPosition       int32  `json:"ordinal_position"`
	ReferencedTableSchema string `json:"referenced_table_schema"`
	ReferencedTableName   string `json:"referenced_table_name"`
	ReferencedColumnName  string `json:"referenced_column_name"`
}

type TenantSchemaVersion struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	CapturedAt time.Time
	SchemaHash string
	Blob       json.RawMessage
	CreatedAt  time.Time
}
