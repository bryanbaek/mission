package mysqlgateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
)

const (
	connectTimeout = 5 * time.Second
	queryTimeout   = 30 * time.Second
	schemaTimeout  = 55 * time.Second
)

var allowedPrivileges = map[string]struct{}{
	"USAGE":     {},
	"SELECT":    {},
	"SHOW VIEW": {},
}

type Result struct {
	Columns      []string
	Rows         []map[string]any
	ElapsedMS    int64
	DatabaseUser string
	DatabaseName string
}

type Gateway struct {
	db           *sql.DB
	databaseUser string
	databaseName string
}

func Open(ctx context.Context, rawDSN string) (*Gateway, error) {
	normalizedDSN, err := NormalizeDSN(rawDSN)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", normalizedDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := withTimeoutCap(ctx, connectTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	databaseUser, databaseName, err := validateConnection(pingCtx, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Gateway{
		db:           db,
		databaseUser: databaseUser,
		databaseName: databaseName,
	}, nil
}

func NormalizeDSN(raw string) (string, error) {
	cfg, err := mysql.ParseDSN(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse mysql dsn: %w", err)
	}

	cfg.ParseTime = true
	cfg.MultiStatements = false
	cfg.Timeout = connectTimeout
	cfg.ReadTimeout = queryTimeout
	cfg.WriteTimeout = connectTimeout
	cfg.Loc = time.UTC

	return cfg.FormatDSN(), nil
}

func ValidateGrants(grants []string) error {
	hasSelect := false

	for _, grant := range grants {
		normalized := strings.ToUpper(strings.TrimSpace(grant))
		if normalized == "" {
			return errors.New("grant output contained an empty row")
		}
		if strings.Contains(normalized, "WITH GRANT OPTION") {
			return errors.New("WITH GRANT OPTION is not allowed")
		}
		if !strings.HasPrefix(normalized, "GRANT ") {
			return fmt.Errorf("unsupported grant format: %q", grant)
		}

		onIndex := strings.Index(normalized, " ON ")
		if onIndex == -1 {
			return fmt.Errorf("malformed grant: %q", grant)
		}

		privilegeList := normalized[len("GRANT "):onIndex]
		for _, privilege := range strings.Split(privilegeList, ",") {
			name := normalizePrivilege(privilege)
			if name == "ALL PRIVILEGES" {
				return errors.New("ALL PRIVILEGES is not allowed")
			}
			if _, ok := allowedPrivileges[name]; !ok {
				return fmt.Errorf("privilege %q is not allowed", name)
			}
			if name == "SELECT" {
				hasSelect = true
			}
		}
	}

	if !hasSelect {
		return errors.New("at least one SELECT grant is required")
	}
	return nil
}

func (g *Gateway) Close() error {
	if g == nil || g.db == nil {
		return nil
	}
	return g.db.Close()
}

func (g *Gateway) ExecuteQuery(ctx context.Context, sqlText string) (Result, error) {
	queryCtx, cancel := withTimeoutCap(ctx, queryTimeout)
	defer cancel()

	conn, err := g.db.Conn(queryCtx)
	if err != nil {
		return Result{}, fmt.Errorf("acquire mysql connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(
		queryCtx,
		"SET SESSION max_execution_time = 30000",
	); err != nil {
		return Result{}, fmt.Errorf("set max_execution_time: %w", err)
	}

	startedAt := time.Now()
	rows, err := conn.QueryContext(queryCtx, sqlText)
	if err != nil {
		if errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
			return Result{}, fmt.Errorf("execute query: %w", queryCtx.Err())
		}
		if queryTimedOut(err) {
			return Result{}, fmt.Errorf("execute query: %w", context.DeadlineExceeded)
		}
		return Result{}, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	resultRows, columns, err := scanRows(rows)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Columns:      columns,
		Rows:         resultRows,
		ElapsedMS:    time.Since(startedAt).Milliseconds(),
		DatabaseUser: g.databaseUser,
		DatabaseName: g.databaseName,
	}, nil
}

func (g *Gateway) IntrospectSchema(
	ctx context.Context,
) (introspect.SchemaBlob, int64, string, string, error) {
	schemaCtx, cancel := withTimeoutCap(ctx, schemaTimeout)
	defer cancel()

	startedAt := time.Now()
	schema, err := introspect.Load(schemaCtx, g.db, g.databaseName)
	if err != nil {
		return introspect.SchemaBlob{}, 0, "", "", err
	}

	return schema, time.Since(startedAt).Milliseconds(), g.databaseUser, g.databaseName, nil
}

func validateConnection(ctx context.Context, db *sql.DB) (string, string, error) {
	var databaseUser string
	var databaseName sql.NullString

	if err := db.QueryRowContext(
		ctx,
		"SELECT CURRENT_USER(), DATABASE()",
	).Scan(&databaseUser, &databaseName); err != nil {
		return "", "", fmt.Errorf("read mysql identity: %w", err)
	}

	grants, err := loadGrants(ctx, db)
	if err != nil {
		return "", "", err
	}
	if err := ValidateGrants(grants); err != nil {
		return "", "", fmt.Errorf("validate mysql grants: %w", err)
	}

	return databaseUser, databaseName.String, nil
}

func loadGrants(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW GRANTS")
	if err != nil {
		return nil, fmt.Errorf("show grants: %w", err)
	}
	defer rows.Close()

	var grants []string
	for rows.Next() {
		var grant string
		if err := rows.Scan(&grant); err != nil {
			return nil, fmt.Errorf("scan show grants: %w", err)
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate show grants: %w", err)
	}
	return grants, nil
}

func scanRows(rows *sql.Rows) ([]map[string]any, []string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("list columns: %w", err)
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("list column types: %w", err)
	}

	decoded := make([]map[string]any, 0)
	rawValues := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range rawValues {
		scanTargets[i] = &rawValues[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = normalizeValue(rawValues[i], columnTypes[i])
		}
		decoded = append(decoded, row)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate rows: %w", err)
	}

	return decoded, columns, nil
}

func normalizeValue(value any, columnType *sql.ColumnType) any {
	if value == nil {
		return nil
	}

	databaseTypeName := ""
	if columnType != nil {
		databaseTypeName = columnType.DatabaseTypeName()
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case int8, int16, int32, int64, int:
		return typed
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		if typed <= math.MaxInt64 {
			return int64(typed)
		}
		return fmt.Sprint(typed)
	case uint:
		if typed <= math.MaxInt64 {
			return int64(typed)
		}
		return fmt.Sprint(typed)
	case float32:
		return float64(typed)
	case float64:
		return typed
	case string:
		return typed
	case []byte:
		scale, ok := decimalScale(columnType)
		return normalizeBytesValue(typed, databaseTypeName, scale, ok)
	case time.Time:
		return normalizeTimeValue(typed, databaseTypeName)
	default:
		return fmt.Sprint(typed)
	}
}

func normalizeBytesValue(
	value []byte,
	databaseTypeName string,
	decimalDigits int64,
	hasDecimalDigits bool,
) any {
	text := string(value)
	typeName := normalizePrivilege(databaseTypeName)

	switch typeName {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT":
		if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
			return parsed
		}
	case "FLOAT", "DOUBLE", "REAL":
		if parsed, err := strconv.ParseFloat(text, 64); err == nil {
			return parsed
		}
	case "DECIMAL", "NEWDECIMAL", "NUMERIC":
		if hasDecimalDigits && decimalDigits == 0 {
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return parsed
			}
		}
		return text
	}

	return text
}

func normalizeTimeValue(value time.Time, databaseTypeName string) string {
	switch normalizePrivilege(databaseTypeName) {
	case "DATE":
		return value.UTC().Format("2006-01-02")
	case "TIME":
		return value.UTC().Format("15:04:05")
	default:
		return value.UTC().Format(time.RFC3339Nano)
	}
}

func normalizePrivilege(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(strings.ToUpper(value))), " ")
}

func withTimeoutCap(
	ctx context.Context,
	limit time.Duration,
) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= limit {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, limit)
}

func decimalScale(columnType *sql.ColumnType) (int64, bool) {
	if columnType == nil {
		return 0, false
	}
	_, scale, ok := columnType.DecimalSize()
	return scale, ok
}

func queryTimedOut(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "i/o timeout") ||
		strings.Contains(message, "invalid connection") ||
		strings.Contains(message, "maximum statement execution time exceeded") ||
		strings.Contains(message, "interrupted")
}
