package introspect

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestLoadEmptySchema(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New returned error: %v", err)
	}
	t.Cleanup(func() {
		closeAndReport(t, "sqlmock db", db.Close)
	})

	expectSchemaQueries(
		mock,
		sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "TABLE_COMMENT"}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"DATA_TYPE",
			"COLUMN_TYPE",
			"IS_NULLABLE",
			"COLUMN_DEFAULT",
			"COLUMN_COMMENT",
		}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
		}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"REFERENCED_TABLE_SCHEMA",
			"REFERENCED_TABLE_NAME",
			"REFERENCED_COLUMN_NAME",
		}),
	)

	blob, err := Load(context.Background(), db, "mission_app")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(blob.Tables) != 0 || len(blob.Columns) != 0 || len(blob.ForeignKeys) != 0 {
		t.Fatalf("unexpected non-empty blob: %+v", blob)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet returned error: %v", err)
	}
}

func TestLoadCapturesForeignKeysAndSorts(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New returned error: %v", err)
	}
	t.Cleanup(func() {
		closeAndReport(t, "sqlmock db", db.Close)
	})

	expectSchemaQueries(
		mock,
		sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "TABLE_COMMENT"}).
			AddRow("mission_app", "orders", "BASE TABLE", "").
			AddRow("mission_app", "customers", "BASE TABLE", ""),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"DATA_TYPE",
			"COLUMN_TYPE",
			"IS_NULLABLE",
			"COLUMN_DEFAULT",
			"COLUMN_COMMENT",
		}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
		}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"REFERENCED_TABLE_SCHEMA",
			"REFERENCED_TABLE_NAME",
			"REFERENCED_COLUMN_NAME",
		}).
			AddRow("mission_app", "orders", "fk_orders_customers", "customer_id", 1, "mission_app", "customers", "id").
			AddRow("mission_app", "orders", "fk_orders_addresses", "shipping_address_id", 2, "mission_app", "addresses", "id"),
	)

	blob, err := Load(context.Background(), db, "mission_app")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(blob.ForeignKeys) != 2 {
		t.Fatalf("foreign key count = %d, want 2", len(blob.ForeignKeys))
	}
	if blob.Tables[0].TableName != "customers" {
		t.Fatalf("first table = %q, want customers", blob.Tables[0].TableName)
	}
	if blob.ForeignKeys[0].ConstraintName != "fk_orders_addresses" {
		t.Fatalf("first fk = %q, want fk_orders_addresses", blob.ForeignKeys[0].ConstraintName)
	}
}

func TestLoadCapturesComments(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New returned error: %v", err)
	}
	t.Cleanup(func() {
		closeAndReport(t, "sqlmock db", db.Close)
	})

	expectSchemaQueries(
		mock,
		sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "TABLE_COMMENT"}).
			AddRow("mission_app", "customers", "BASE TABLE", "Customer master"),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"DATA_TYPE",
			"COLUMN_TYPE",
			"IS_NULLABLE",
			"COLUMN_DEFAULT",
			"COLUMN_COMMENT",
		}).
			AddRow("mission_app", "customers", "id", 1, "int", "int", "NO", nil, "Primary key").
			AddRow("mission_app", "customers", "notes", 2, "text", "text", "YES", nil, "CSR note"),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
		}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"REFERENCED_TABLE_SCHEMA",
			"REFERENCED_TABLE_NAME",
			"REFERENCED_COLUMN_NAME",
		}),
	)

	blob, err := Load(context.Background(), db, "mission_app")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if blob.Tables[0].TableComment != "Customer master" {
		t.Fatalf("table comment = %q, want Customer master", blob.Tables[0].TableComment)
	}
	if blob.Columns[1].ColumnComment != "CSR note" {
		t.Fatalf("column comment = %q, want CSR note", blob.Columns[1].ColumnComment)
	}
}

func TestLoadCapturesColumnTypes(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New returned error: %v", err)
	}
	t.Cleanup(func() {
		closeAndReport(t, "sqlmock db", db.Close)
	})

	expectSchemaQueries(
		mock,
		sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "TABLE_COMMENT"}).
			AddRow("mission_app", "metrics", "BASE TABLE", ""),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"DATA_TYPE",
			"COLUMN_TYPE",
			"IS_NULLABLE",
			"COLUMN_DEFAULT",
			"COLUMN_COMMENT",
		}).
			AddRow("mission_app", "metrics", "count_value", 1, "int", "int", "NO", nil, "").
			AddRow("mission_app", "metrics", "label", 2, "varchar", "varchar(255)", "NO", "default-label", "").
			AddRow("mission_app", "metrics", "amount", 3, "decimal", "decimal(10,2)", "NO", "0.00", "").
			AddRow("mission_app", "metrics", "captured_at", 4, "datetime", "datetime", "YES", nil, "").
			AddRow("mission_app", "metrics", "payload", 5, "json", "json", "YES", nil, "").
			AddRow("mission_app", "metrics", "notes", 6, "text", "text", "YES", nil, ""),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
		}),
		sqlmock.NewRows([]string{
			"TABLE_SCHEMA",
			"TABLE_NAME",
			"CONSTRAINT_NAME",
			"COLUMN_NAME",
			"ORDINAL_POSITION",
			"REFERENCED_TABLE_SCHEMA",
			"REFERENCED_TABLE_NAME",
			"REFERENCED_COLUMN_NAME",
		}),
	)

	blob, err := Load(context.Background(), db, "mission_app")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(blob.Columns) != 6 {
		t.Fatalf("column count = %d, want 6", len(blob.Columns))
	}
	if blob.Columns[1].DefaultValue != "default-label" || !blob.Columns[1].HasDefault {
		t.Fatalf("label default = %#v, want default-label with HasDefault", blob.Columns[1])
	}
	if blob.Columns[4].DataType != "json" {
		t.Fatalf("json column data type = %q, want json", blob.Columns[4].DataType)
	}
}

func expectSchemaQueries(
	mock sqlmock.Sqlmock,
	tables *sqlmock.Rows,
	columns *sqlmock.Rows,
	primaryKeys *sqlmock.Rows,
	foreignKeys *sqlmock.Rows,
) {
	mock.ExpectQuery(regexp.QuoteMeta("FROM information_schema.tables")).
		WillReturnRows(tables)
	mock.ExpectQuery(regexp.QuoteMeta("FROM information_schema.columns")).
		WillReturnRows(columns)
	mock.ExpectQuery(regexp.QuoteMeta("FROM information_schema.table_constraints tc")).
		WillReturnRows(primaryKeys)
	mock.ExpectQuery(regexp.QuoteMeta("FROM information_schema.table_constraints tc")).
		WillReturnRows(foreignKeys)
}

func closeAndReport(t *testing.T, name string, closeFn func() error) {
	t.Helper()
	if err := closeFn(); err != nil {
		t.Errorf("close %s: %v", name, err)
	}
}
