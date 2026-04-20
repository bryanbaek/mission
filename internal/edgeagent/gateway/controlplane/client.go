package controlplane

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/bryanbaek/mission/gen/go/agent/v1"
	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	edgecontroller "github.com/bryanbaek/mission/internal/edgeagent/controller"
	"github.com/bryanbaek/mission/internal/edgeagent/introspect"
	"github.com/bryanbaek/mission/internal/queryerror"
)

type Client struct {
	client agentv1connect.AgentServiceClient
}

func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		client: agentv1connect.NewAgentServiceClient(
			httpClient,
			baseURL,
			connect.WithInterceptors(&bearerInterceptor{token: token}),
		),
	}
}

func (c *Client) OpenCommandStream(
	ctx context.Context,
	req edgecontroller.OpenCommandStreamRequest,
) (edgecontroller.CommandStream, error) {
	stream, err := c.client.OpenCommandStream(
		ctx,
		connect.NewRequest(&agentv1.OpenCommandStreamRequest{
			SessionId:    req.SessionID,
			Hostname:     req.Hostname,
			AgentVersion: req.AgentVersion,
			StartedAt:    timestamppb.New(req.StartedAt),
		}),
	)
	if err != nil {
		return nil, err
	}
	return &commandStream{stream: stream}, nil
}

func (c *Client) Heartbeat(
	ctx context.Context,
	req edgecontroller.HeartbeatRequest,
) error {
	_, err := c.client.Heartbeat(
		ctx,
		connect.NewRequest(&agentv1.HeartbeatRequest{
			SessionId: req.SessionID,
			SentAt:    timestamppb.New(req.SentAt),
		}),
	)
	return err
}

func (c *Client) SubmitPingResult(
	ctx context.Context,
	req edgecontroller.SubmitPingResultRequest,
) error {
	_, err := c.client.SubmitCommandResult(
		ctx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   req.SessionID,
			CommandId:   req.CommandID,
			CompletedAt: timestamppb.New(req.CompletedAt),
			Result: &agentv1.SubmitCommandResultRequest_Ping{
				Ping: &agentv1.PingResult{RoundTripMs: req.RoundTripMS},
			},
		}),
	)
	return err
}

func (c *Client) SubmitExecuteQueryResult(
	ctx context.Context,
	req edgecontroller.SubmitExecuteQueryResultRequest,
) error {
	result, err := queryResultToProto(req)
	if err != nil {
		return err
	}

	_, err = c.client.SubmitCommandResult(
		ctx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   req.SessionID,
			CommandId:   req.CommandID,
			CompletedAt: timestamppb.New(req.CompletedAt),
			Result: &agentv1.SubmitCommandResultRequest_ExecuteQuery{
				ExecuteQuery: result,
			},
		}),
	)
	return err
}

func (c *Client) SubmitIntrospectSchemaResult(
	ctx context.Context,
	req edgecontroller.SubmitIntrospectSchemaResultRequest,
) error {
	result := schemaResultToProto(req)
	_, err := c.client.SubmitCommandResult(
		ctx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   req.SessionID,
			CommandId:   req.CommandID,
			CompletedAt: timestamppb.New(req.CompletedAt),
			Result: &agentv1.SubmitCommandResultRequest_IntrospectSchema{
				IntrospectSchema: result,
			},
		}),
	)
	return err
}

func (c *Client) SubmitConfigureDatabaseResult(
	ctx context.Context,
	req edgecontroller.SubmitConfigureDatabaseResultRequest,
) error {
	_, err := c.client.SubmitCommandResult(
		ctx,
		connect.NewRequest(&agentv1.SubmitCommandResultRequest{
			SessionId:   req.SessionID,
			CommandId:   req.CommandID,
			CompletedAt: timestamppb.New(req.CompletedAt),
			Result: &agentv1.SubmitCommandResultRequest_ConfigureDatabase{
				ConfigureDatabase: &agentv1.ConfigureDatabaseResult{
					ElapsedMs:    req.ElapsedMS,
					DatabaseUser: req.DatabaseUser,
					DatabaseName: req.DatabaseName,
					Error:        req.Error,
					ErrorCode:    configureDatabaseErrorCodeToProto(req.ErrorCode),
				},
			},
		}),
	)
	return err
}

type commandStream struct {
	stream *connect.ServerStreamForClient[agentv1.ControlMessage]
}

func (s *commandStream) Receive() bool {
	return s.stream.Receive()
}

func (s *commandStream) Message() edgecontroller.ControlMessage {
	msg := s.stream.Msg()
	command := edgecontroller.ControlMessage{
		SessionID: msg.GetSessionId(),
		CommandID: msg.GetCommandId(),
		Kind:      commandKind(msg),
		SQL:       commandSQL(msg),
		DSN:       commandDSN(msg),
	}
	if msg.GetIssuedAt() != nil {
		command.IssuedAt = msg.GetIssuedAt().AsTime().UTC()
	}
	return command
}

func (s *commandStream) Err() error {
	return s.stream.Err()
}

type bearerInterceptor struct {
	token string
}

func (b *bearerInterceptor) WrapUnary(
	next connect.UnaryFunc,
) connect.UnaryFunc {
	return func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+b.token)
		return next(ctx, req)
	}
}

func (b *bearerInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+b.token)
		return conn
	}
}

func (b *bearerInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return next
}

func commandKind(msg *agentv1.ControlMessage) edgecontroller.CommandKind {
	switch msg.Payload.(type) {
	case *agentv1.ControlMessage_Ping:
		return edgecontroller.CommandKindPing
	case *agentv1.ControlMessage_ExecuteQuery:
		return edgecontroller.CommandKindExecuteQuery
	case *agentv1.ControlMessage_IntrospectSchema:
		return edgecontroller.CommandKindIntrospectSchema
	case *agentv1.ControlMessage_ConfigureDatabase:
		return edgecontroller.CommandKindConfigureDatabase
	default:
		return ""
	}
}

func commandSQL(msg *agentv1.ControlMessage) string {
	switch payload := msg.Payload.(type) {
	case *agentv1.ControlMessage_ExecuteQuery:
		return payload.ExecuteQuery.GetSql()
	default:
		return ""
	}
}

func commandDSN(msg *agentv1.ControlMessage) string {
	switch payload := msg.Payload.(type) {
	case *agentv1.ControlMessage_ConfigureDatabase:
		return payload.ConfigureDatabase.GetDsn()
	default:
		return ""
	}
}

func queryResultToProto(
	req edgecontroller.SubmitExecuteQueryResultRequest,
) (*agentv1.ExecuteQueryResult, error) {
	rows := make([]*agentv1.ExecuteQueryRow, 0, len(req.Rows))
	for _, row := range req.Rows {
		values := make(map[string]*structpb.Value, len(row))
		for key, value := range row {
			pbValue, err := structpb.NewValue(value)
			if err != nil {
				return nil, fmt.Errorf("convert row value %q: %w", key, err)
			}
			values[key] = pbValue
		}
		rows = append(rows, &agentv1.ExecuteQueryRow{Values: values})
	}

	return &agentv1.ExecuteQueryResult{
		Columns:           append([]string(nil), req.Columns...),
		Rows:              rows,
		ElapsedMs:         req.ElapsedMS,
		DatabaseUser:      req.DatabaseUser,
		DatabaseName:      req.DatabaseName,
		Error:             req.Error,
		ErrorCode:         queryErrorCodeToProto(req.ErrorCode),
		ErrorReason:       req.ErrorReason,
		BlockedConstructs: append([]string(nil), req.BlockedConstructs...),
	}, nil
}

func schemaResultToProto(
	req edgecontroller.SubmitIntrospectSchemaResultRequest,
) *agentv1.IntrospectSchemaResult {
	return &agentv1.IntrospectSchemaResult{
		Schema:       schemaBlobToProto(req.Schema),
		ElapsedMs:    req.ElapsedMS,
		DatabaseUser: req.DatabaseUser,
		DatabaseName: req.DatabaseName,
		Error:        req.Error,
	}
}

func schemaBlobToProto(blob introspect.SchemaBlob) *agentv1.SchemaBlob {
	out := &agentv1.SchemaBlob{
		DatabaseName: blob.DatabaseName,
		Tables:       make([]*agentv1.SchemaTable, 0, len(blob.Tables)),
		Columns:      make([]*agentv1.SchemaColumn, 0, len(blob.Columns)),
		PrimaryKeys:  make([]*agentv1.SchemaPrimaryKey, 0, len(blob.PrimaryKeys)),
		ForeignKeys:  make([]*agentv1.SchemaForeignKey, 0, len(blob.ForeignKeys)),
	}
	for _, table := range blob.Tables {
		out.Tables = append(out.Tables, &agentv1.SchemaTable{
			TableSchema:  table.TableSchema,
			TableName:    table.TableName,
			TableType:    table.TableType,
			TableComment: table.TableComment,
		})
	}
	for _, column := range blob.Columns {
		out.Columns = append(out.Columns, &agentv1.SchemaColumn{
			TableSchema:     column.TableSchema,
			TableName:       column.TableName,
			ColumnName:      column.ColumnName,
			OrdinalPosition: column.OrdinalPosition,
			DataType:        column.DataType,
			ColumnType:      column.ColumnType,
			IsNullable:      column.IsNullable,
			HasDefault:      column.HasDefault,
			DefaultValue:    column.DefaultValue,
			ColumnComment:   column.ColumnComment,
		})
	}
	for _, key := range blob.PrimaryKeys {
		out.PrimaryKeys = append(out.PrimaryKeys, &agentv1.SchemaPrimaryKey{
			TableSchema:     key.TableSchema,
			TableName:       key.TableName,
			ConstraintName:  key.ConstraintName,
			ColumnName:      key.ColumnName,
			OrdinalPosition: key.OrdinalPosition,
		})
	}
	for _, key := range blob.ForeignKeys {
		out.ForeignKeys = append(out.ForeignKeys, &agentv1.SchemaForeignKey{
			TableSchema:           key.TableSchema,
			TableName:             key.TableName,
			ConstraintName:        key.ConstraintName,
			ColumnName:            key.ColumnName,
			OrdinalPosition:       key.OrdinalPosition,
			ReferencedTableSchema: key.ReferencedTableSchema,
			ReferencedTableName:   key.ReferencedTableName,
			ReferencedColumnName:  key.ReferencedColumnName,
		})
	}
	return out
}

func queryErrorCodeToProto(code queryerror.Code) agentv1.ExecuteQueryErrorCode {
	switch code {
	case queryerror.CodePermissionDenied:
		return agentv1.ExecuteQueryErrorCode_EXECUTE_QUERY_ERROR_CODE_PERMISSION_DENIED
	case queryerror.CodeInternal:
		return agentv1.ExecuteQueryErrorCode_EXECUTE_QUERY_ERROR_CODE_INTERNAL
	default:
		return agentv1.ExecuteQueryErrorCode_EXECUTE_QUERY_ERROR_CODE_UNSPECIFIED
	}
}

func configureDatabaseErrorCodeToProto(
	code edgecontroller.ConfigureDatabaseErrorCode,
) agentv1.ConfigureDatabaseErrorCode {
	switch code {
	case edgecontroller.ConfigureDatabaseErrorCodeInvalidDSN:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_INVALID_DSN
	case edgecontroller.ConfigureDatabaseErrorCodeConnectFailed:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_CONNECT_FAILED
	case edgecontroller.ConfigureDatabaseErrorCodeAuthFailed:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_AUTH_FAILED
	case edgecontroller.ConfigureDatabaseErrorCodePrivilegeError:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_PRIVILEGE_INVALID
	case edgecontroller.ConfigureDatabaseErrorCodeWriteConfig:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_WRITE_CONFIG_FAILED
	case edgecontroller.ConfigureDatabaseErrorCodeTimeout:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_TIMEOUT
	default:
		return agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_UNSPECIFIED
	}
}
