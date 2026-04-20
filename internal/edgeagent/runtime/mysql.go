package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	mysqlgateway "github.com/bryanbaek/mission/internal/edgeagent/gateway/mysql"
	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
	"github.com/bryanbaek/mission/internal/queryerror"
)

var ErrDatabaseNotConfigured = errors.New("database not configured")

type MySQLRuntime struct {
	mu      sync.RWMutex
	dsnPath string
	gateway *mysqlgateway.Gateway
}

func NewMySQLRuntime(
	ctx context.Context,
	dsnPath, initialDSN string,
) (*MySQLRuntime, error) {
	runtime := &MySQLRuntime{dsnPath: dsnPath}
	if strings.TrimSpace(initialDSN) == "" {
		return runtime, nil
	}
	gateway, err := mysqlgateway.Open(ctx, initialDSN)
	if err != nil {
		return nil, fmt.Errorf("open initial mysql gateway: %w", err)
	}
	runtime.gateway = gateway
	return runtime, nil
}

func (r *MySQLRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gateway == nil {
		return nil
	}
	err := r.gateway.Close()
	r.gateway = nil
	return err
}

func (r *MySQLRuntime) ExecuteQuery(
	ctx context.Context,
	sql string,
) (edgecontroller.QueryResult, error) {
	gateway := r.currentGateway()
	if gateway == nil {
		return edgecontroller.QueryResult{}, queryerror.Internal(
			ErrDatabaseNotConfigured.Error(),
			ErrDatabaseNotConfigured,
		)
	}
	result, err := gateway.ExecuteQuery(ctx, sql)
	if err != nil {
		return edgecontroller.QueryResult{}, err
	}
	return edgecontroller.QueryResult{
		Columns:      result.Columns,
		Rows:         result.Rows,
		ElapsedMS:    result.ElapsedMS,
		DatabaseUser: result.DatabaseUser,
		DatabaseName: result.DatabaseName,
	}, nil
}

func (r *MySQLRuntime) IntrospectSchema(
	ctx context.Context,
) (
	introspect.SchemaBlob,
	int64,
	string,
	string,
	error,
) {
	gateway := r.currentGateway()
	if gateway == nil {
		return introspect.SchemaBlob{}, 0, "", "", ErrDatabaseNotConfigured
	}
	return gateway.IntrospectSchema(ctx)
}

func (r *MySQLRuntime) ConfigureDatabase(
	ctx context.Context,
	dsn string,
) (edgecontroller.ConfigureDatabaseResult, error) {
	startedAt := time.Now()
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return edgecontroller.ConfigureDatabaseResult{
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Error:     "dsn is required",
			ErrorCode: edgecontroller.ConfigureDatabaseErrorCodeInvalidDSN,
		}, nil
	}

	normalized, err := mysqlgateway.NormalizeDSN(trimmed)
	if err != nil {
		return edgecontroller.ConfigureDatabaseResult{
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Error:     err.Error(),
			ErrorCode: edgecontroller.ConfigureDatabaseErrorCodeInvalidDSN,
		}, nil
	}

	newGateway, err := mysqlgateway.Open(ctx, normalized)
	if err != nil {
		return edgecontroller.ConfigureDatabaseResult{
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Error:     err.Error(),
			ErrorCode: classifyConfigureDatabaseError(err),
		}, nil
	}
	defer func() {
		if newGateway != nil {
			_ = newGateway.Close()
		}
	}()

	selectResult, err := newGateway.ExecuteQuery(ctx, "SELECT 1")
	if err != nil {
		return edgecontroller.ConfigureDatabaseResult{
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Error:     err.Error(),
			ErrorCode: classifyConfigureDatabaseError(err),
		}, nil
	}

	if err := r.writeDSN(normalized); err != nil {
		return edgecontroller.ConfigureDatabaseResult{
			ElapsedMS:    time.Since(startedAt).Milliseconds(),
			DatabaseUser: selectResult.DatabaseUser,
			DatabaseName: selectResult.DatabaseName,
			Error:        err.Error(),
			ErrorCode:    edgecontroller.ConfigureDatabaseErrorCodeWriteConfig,
		}, nil
	}

	oldGateway := r.swapGateway(newGateway)
	newGateway = nil
	if oldGateway != nil {
		_ = oldGateway.Close()
	}

	return edgecontroller.ConfigureDatabaseResult{
		ElapsedMS:    time.Since(startedAt).Milliseconds(),
		DatabaseUser: selectResult.DatabaseUser,
		DatabaseName: selectResult.DatabaseName,
	}, nil
}

func (r *MySQLRuntime) currentGateway() *mysqlgateway.Gateway {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gateway
}

func (r *MySQLRuntime) swapGateway(next *mysqlgateway.Gateway) *mysqlgateway.Gateway {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.gateway
	r.gateway = next
	return current
}

func (r *MySQLRuntime) writeDSN(dsn string) error {
	if r.dsnPath == "" {
		return errors.New("dsn path is not configured")
	}
	dir := filepath.Dir(r.dsnPath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dsn directory: %w", err)
		}
	}
	if err := os.WriteFile(r.dsnPath, []byte(dsn+"\n"), 0o600); err != nil {
		return fmt.Errorf("write dsn file: %w", err)
	}
	return nil
}

func classifyConfigureDatabaseError(err error) edgecontroller.ConfigureDatabaseErrorCode {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return edgecontroller.ConfigureDatabaseErrorCodeTimeout
	case errors.Is(err, ErrDatabaseNotConfigured):
		return edgecontroller.ConfigureDatabaseErrorCodeConnectFailed
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "parse mysql dsn"):
		return edgecontroller.ConfigureDatabaseErrorCodeInvalidDSN
	case strings.Contains(message, "access denied"):
		return edgecontroller.ConfigureDatabaseErrorCodeAuthFailed
	case strings.Contains(message, "validate mysql grants"),
		strings.Contains(message, "show grants"),
		strings.Contains(message, "at least one select grant"),
		strings.Contains(message, "not allowed"):
		return edgecontroller.ConfigureDatabaseErrorCodePrivilegeError
	case strings.Contains(message, "timeout"),
		strings.Contains(message, "deadline exceeded"),
		strings.Contains(message, "i/o timeout"):
		return edgecontroller.ConfigureDatabaseErrorCodeTimeout
	default:
		return edgecontroller.ConfigureDatabaseErrorCodeConnectFailed
	}
}
