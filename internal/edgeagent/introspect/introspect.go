package introspect

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

type SchemaBlob struct {
	DatabaseName string
	Tables       []SchemaTable
	Columns      []SchemaColumn
	PrimaryKeys  []SchemaPrimaryKey
	ForeignKeys  []SchemaForeignKey
}

type SchemaTable struct {
	TableSchema  string
	TableName    string
	TableType    string
	TableComment string
}

type SchemaColumn struct {
	TableSchema     string
	TableName       string
	ColumnName      string
	OrdinalPosition int32
	DataType        string
	ColumnType      string
	IsNullable      bool
	HasDefault      bool
	DefaultValue    string
	ColumnComment   string
}

type SchemaPrimaryKey struct {
	TableSchema     string
	TableName       string
	ConstraintName  string
	ColumnName      string
	OrdinalPosition int32
}

type SchemaForeignKey struct {
	TableSchema           string
	TableName             string
	ConstraintName        string
	ColumnName            string
	OrdinalPosition       int32
	ReferencedTableSchema string
	ReferencedTableName   string
	ReferencedColumnName  string
}

func Load(
	ctx context.Context,
	db *sql.DB,
	databaseName string,
) (SchemaBlob, error) {
	if db == nil {
		return SchemaBlob{}, fmt.Errorf("load schema: nil db")
	}
	if databaseName == "" {
		return SchemaBlob{}, fmt.Errorf("load schema: empty database name")
	}

	out := SchemaBlob{DatabaseName: databaseName}

	tables, err := loadTables(ctx, db, databaseName)
	if err != nil {
		return SchemaBlob{}, err
	}
	out.Tables = tables

	columns, err := loadColumns(ctx, db, databaseName)
	if err != nil {
		return SchemaBlob{}, err
	}
	out.Columns = columns

	primaryKeys, err := loadPrimaryKeys(ctx, db, databaseName)
	if err != nil {
		return SchemaBlob{}, err
	}
	out.PrimaryKeys = primaryKeys

	foreignKeys, err := loadForeignKeys(ctx, db, databaseName)
	if err != nil {
		return SchemaBlob{}, err
	}
	out.ForeignKeys = foreignKeys

	sortSchema(&out)
	return out, nil
}

func loadTables(
	ctx context.Context,
	db *sql.DB,
	databaseName string,
) (tables []SchemaTable, err error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			TABLE_TYPE,
			TABLE_COMMENT
		FROM information_schema.tables
		WHERE TABLE_SCHEMA = ?
		  AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`, databaseName)
	if err != nil {
		return nil, fmt.Errorf("load tables: %w", err)
	}
	defer func() {
		appendCloseError(&err, "close table rows", rows.Close)
	}()

	var out []SchemaTable
	for rows.Next() {
		var table SchemaTable
		if err := rows.Scan(
			&table.TableSchema,
			&table.TableName,
			&table.TableType,
			&table.TableComment,
		); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		out = append(out, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}
	return out, nil
}

func loadColumns(
	ctx context.Context,
	db *sql.DB,
	databaseName string,
) (columns []SchemaColumn, err error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			COLUMN_NAME,
			ORDINAL_POSITION,
			DATA_TYPE,
			COLUMN_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_COMMENT
		FROM information_schema.columns
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION
	`, databaseName)
	if err != nil {
		return nil, fmt.Errorf("load columns: %w", err)
	}
	defer func() {
		appendCloseError(&err, "close column rows", rows.Close)
	}()

	var out []SchemaColumn
	for rows.Next() {
		var (
			column      SchemaColumn
			isNullable  string
			defaultText sql.NullString
		)
		if err := rows.Scan(
			&column.TableSchema,
			&column.TableName,
			&column.ColumnName,
			&column.OrdinalPosition,
			&column.DataType,
			&column.ColumnType,
			&isNullable,
			&defaultText,
			&column.ColumnComment,
		); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		column.IsNullable = isNullable == "YES"
		column.HasDefault = defaultText.Valid
		if defaultText.Valid {
			column.DefaultValue = defaultText.String
		}
		out = append(out, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns: %w", err)
	}
	return out, nil
}

func loadPrimaryKeys(
	ctx context.Context,
	db *sql.DB,
	databaseName string,
) (primaryKeys []SchemaPrimaryKey, err error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			kcu.TABLE_SCHEMA,
			kcu.TABLE_NAME,
			kcu.CONSTRAINT_NAME,
			kcu.COLUMN_NAME,
			kcu.ORDINAL_POSITION
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
		 AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
		 AND tc.TABLE_NAME = kcu.TABLE_NAME
		 AND tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA = ?
		  AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
		ORDER BY
			kcu.TABLE_SCHEMA,
			kcu.TABLE_NAME,
			kcu.CONSTRAINT_NAME,
			kcu.ORDINAL_POSITION
	`, databaseName)
	if err != nil {
		return nil, fmt.Errorf("load primary keys: %w", err)
	}
	defer func() {
		appendCloseError(&err, "close primary key rows", rows.Close)
	}()

	var out []SchemaPrimaryKey
	for rows.Next() {
		var key SchemaPrimaryKey
		if err := rows.Scan(
			&key.TableSchema,
			&key.TableName,
			&key.ConstraintName,
			&key.ColumnName,
			&key.OrdinalPosition,
		); err != nil {
			return nil, fmt.Errorf("scan primary key: %w", err)
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate primary keys: %w", err)
	}
	return out, nil
}

func loadForeignKeys(
	ctx context.Context,
	db *sql.DB,
	databaseName string,
) (foreignKeys []SchemaForeignKey, err error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			kcu.TABLE_SCHEMA,
			kcu.TABLE_NAME,
			kcu.CONSTRAINT_NAME,
			kcu.COLUMN_NAME,
			kcu.ORDINAL_POSITION,
			kcu.REFERENCED_TABLE_SCHEMA,
			kcu.REFERENCED_TABLE_NAME,
			kcu.REFERENCED_COLUMN_NAME
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
		 AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
		 AND tc.TABLE_NAME = kcu.TABLE_NAME
		 AND tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA = ?
		  AND tc.CONSTRAINT_TYPE = 'FOREIGN KEY'
		ORDER BY
			kcu.TABLE_SCHEMA,
			kcu.TABLE_NAME,
			kcu.CONSTRAINT_NAME,
			kcu.ORDINAL_POSITION
	`, databaseName)
	if err != nil {
		return nil, fmt.Errorf("load foreign keys: %w", err)
	}
	defer func() {
		appendCloseError(&err, "close foreign key rows", rows.Close)
	}()

	var out []SchemaForeignKey
	for rows.Next() {
		var key SchemaForeignKey
		if err := rows.Scan(
			&key.TableSchema,
			&key.TableName,
			&key.ConstraintName,
			&key.ColumnName,
			&key.OrdinalPosition,
			&key.ReferencedTableSchema,
			&key.ReferencedTableName,
			&key.ReferencedColumnName,
		); err != nil {
			return nil, fmt.Errorf("scan foreign key: %w", err)
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate foreign keys: %w", err)
	}
	return out, nil
}

func appendCloseError(target *error, label string, closeFn func() error) {
	if closeErr := closeFn(); closeErr != nil {
		wrapped := fmt.Errorf("%s: %w", label, closeErr)
		if *target == nil {
			*target = wrapped
			return
		}
		*target = errors.Join(*target, wrapped)
	}
}

func sortSchema(blob *SchemaBlob) {
	sort.Slice(blob.Tables, func(i, j int) bool {
		left := blob.Tables[i]
		right := blob.Tables[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		return left.TableName < right.TableName
	})
	sort.Slice(blob.Columns, func(i, j int) bool {
		left := blob.Columns[i]
		right := blob.Columns[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		if left.TableName != right.TableName {
			return left.TableName < right.TableName
		}
		return left.OrdinalPosition < right.OrdinalPosition
	})
	sort.Slice(blob.PrimaryKeys, func(i, j int) bool {
		left := blob.PrimaryKeys[i]
		right := blob.PrimaryKeys[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		if left.TableName != right.TableName {
			return left.TableName < right.TableName
		}
		if left.ConstraintName != right.ConstraintName {
			return left.ConstraintName < right.ConstraintName
		}
		return left.OrdinalPosition < right.OrdinalPosition
	})
	sort.Slice(blob.ForeignKeys, func(i, j int) bool {
		left := blob.ForeignKeys[i]
		right := blob.ForeignKeys[j]
		if left.TableSchema != right.TableSchema {
			return left.TableSchema < right.TableSchema
		}
		if left.TableName != right.TableName {
			return left.TableName < right.TableName
		}
		if left.ConstraintName != right.ConstraintName {
			return left.ConstraintName < right.ConstraintName
		}
		return left.OrdinalPosition < right.OrdinalPosition
	})
}
