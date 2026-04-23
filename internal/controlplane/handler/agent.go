package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/bryanbaek/mission/gen/go/agent/v1"
	"github.com/bryanbaek/mission/gen/go/agent/v1/agentv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/queryerror"
)

type AgentHandler struct {
	agentv1connect.UnimplementedAgentServiceHandler
	sessions *controller.AgentSessionManager
}

func NewAgentHandler(sessions *controller.AgentSessionManager) *AgentHandler {
	return &AgentHandler{sessions: sessions}
}

func (h *AgentHandler) OpenCommandStream(
	ctx context.Context,
	req *connect.Request[agentv1.OpenCommandStreamRequest],
	stream *connect.ServerStream[agentv1.OpenCommandStreamResponse],
) error {
	agent, ok := auth.AgentFromContext(ctx)
	if !ok {
		return connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	session, err := h.sessions.RegisterSession(
		model.TenantToken{
			ID:       agent.TokenID,
			TenantID: agent.TenantID,
			Label:    agent.Label,
		},
		req.Msg.GetSessionId(),
		req.Msg.GetHostname(),
		req.Msg.GetAgentVersion(),
	)
	if err != nil {
		return connectErrorForSession(err)
	}
	slog.Info(
		"agent stream registered",
		"session_id",
		req.Msg.GetSessionId(),
		"tenant_id",
		agent.TenantID,
		"token_id",
		agent.TokenID,
		"label",
		agent.Label,
		"hostname",
		req.Msg.GetHostname(),
		"agent_version",
		req.Msg.GetAgentVersion(),
	)
	defer session.Disconnect()
	defer slog.Info(
		"agent stream closing",
		"session_id",
		req.Msg.GetSessionId(),
		"ctx_err",
		ctx.Err(),
	)

	// Connect's server-streaming client call doesn't complete until the server
	// emits the first frame. Send an immediate payloadless ack so the agent can
	// start heartbeating before any real commands are queued.
	if err := stream.Send(&agentv1.OpenCommandStreamResponse{
		SessionId: req.Msg.GetSessionId(),
	}); err != nil {
		slog.Warn(
			"agent stream ack failed",
			"session_id",
			req.Msg.GetSessionId(),
			"err",
			err,
		)
		return err
	}
	slog.Info("agent stream ack sent", "session_id", req.Msg.GetSessionId())

	for {
		select {
		case <-ctx.Done():
			slog.Warn(
				"agent stream context done",
				"session_id",
				req.Msg.GetSessionId(),
				"err",
				ctx.Err(),
			)
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case <-session.Done:
			slog.Warn("agent stream session done", "session_id", req.Msg.GetSessionId())
			return nil
		case command := <-session.Commands:
			slog.Info(
				"agent stream sending command",
				"session_id",
				command.SessionID,
				"command_id",
				command.CommandID,
				"kind",
				command.Kind,
			)
			if err := stream.Send(commandToProto(command)); err != nil {
				slog.Warn(
					"agent stream send failed",
					"session_id",
					command.SessionID,
					"command_id",
					command.CommandID,
					"err",
					err,
				)
				return err
			}
		}
	}
}

func (h *AgentHandler) Heartbeat(
	ctx context.Context,
	req *connect.Request[agentv1.HeartbeatRequest],
) (*connect.Response[agentv1.HeartbeatResponse], error) {
	agent, ok := auth.AgentFromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	sentAt := time.Now().UTC()
	if req.Msg.GetSentAt() != nil {
		sentAt = req.Msg.GetSentAt().AsTime().UTC()
	}

	if err := h.sessions.MarkHeartbeat(
		agent.TokenID,
		req.Msg.GetSessionId(),
		sentAt,
	); err != nil {
		slog.Warn(
			"agent heartbeat rejected",
			"session_id",
			req.Msg.GetSessionId(),
			"token_id",
			agent.TokenID,
			"err",
			err,
		)
		return nil, connectErrorForSession(err)
	}
	slog.Info(
		"agent heartbeat accepted",
		"session_id",
		req.Msg.GetSessionId(),
		"token_id",
		agent.TokenID,
	)
	return connect.NewResponse(&agentv1.HeartbeatResponse{}), nil
}

func (h *AgentHandler) SubmitCommandResult(
	ctx context.Context,
	req *connect.Request[agentv1.SubmitCommandResultRequest],
) (*connect.Response[agentv1.SubmitCommandResultResponse], error) {
	agent, ok := auth.AgentFromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}
	completedAt := time.Now().UTC()
	if req.Msg.GetCompletedAt() != nil {
		completedAt = req.Msg.GetCompletedAt().AsTime().UTC()
	}

	var err error
	switch result := req.Msg.Result.(type) {
	case *agentv1.SubmitCommandResultRequest_Ping:
		err = h.sessions.SubmitPingResult(
			agent.TokenID,
			req.Msg.GetSessionId(),
			req.Msg.GetCommandId(),
			completedAt,
			result.Ping.GetRoundTripMs(),
		)
	case *agentv1.SubmitCommandResultRequest_ExecuteQuery:
		err = h.sessions.SubmitExecuteQueryResult(
			agent.TokenID,
			req.Msg.GetSessionId(),
			req.Msg.GetCommandId(),
			completedAt,
			result.ExecuteQuery.GetColumns(),
			protoRowsToMaps(result.ExecuteQuery.GetRows()),
			result.ExecuteQuery.GetElapsedMs(),
			result.ExecuteQuery.GetDatabaseUser(),
			result.ExecuteQuery.GetDatabaseName(),
			result.ExecuteQuery.GetError(),
			queryErrorCodeFromProto(result.ExecuteQuery.GetErrorCode()),
			result.ExecuteQuery.GetErrorReason(),
			result.ExecuteQuery.GetBlockedConstructs(),
		)
	case *agentv1.SubmitCommandResultRequest_IntrospectSchema:
		err = h.sessions.SubmitIntrospectSchemaResult(
			agent.TokenID,
			req.Msg.GetSessionId(),
			req.Msg.GetCommandId(),
			completedAt,
			protoSchemaToModel(result.IntrospectSchema.GetSchema()),
			result.IntrospectSchema.GetElapsedMs(),
			result.IntrospectSchema.GetDatabaseUser(),
			result.IntrospectSchema.GetDatabaseName(),
			result.IntrospectSchema.GetError(),
		)
	case *agentv1.SubmitCommandResultRequest_ConfigureDatabase:
		err = h.sessions.SubmitConfigureDatabaseResult(
			agent.TokenID,
			req.Msg.GetSessionId(),
			req.Msg.GetCommandId(),
			completedAt,
			result.ConfigureDatabase.GetElapsedMs(),
			result.ConfigureDatabase.GetDatabaseUser(),
			result.ConfigureDatabase.GetDatabaseName(),
			result.ConfigureDatabase.GetError(),
			configureDatabaseErrorCodeFromProto(
				result.ConfigureDatabase.GetErrorCode(),
			),
		)
	default:
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("missing result payload"),
		)
	}
	if err != nil {
		return nil, connectErrorForSession(err)
	}
	return connect.NewResponse(&agentv1.SubmitCommandResultResponse{}), nil
}

func queryErrorCodeFromProto(code agentv1.ExecuteQueryErrorCode) queryerror.Code {
	switch code {
	case agentv1.ExecuteQueryErrorCode_EXECUTE_QUERY_ERROR_CODE_PERMISSION_DENIED:
		return queryerror.CodePermissionDenied
	case agentv1.ExecuteQueryErrorCode_EXECUTE_QUERY_ERROR_CODE_INTERNAL:
		return queryerror.CodeInternal
	default:
		return queryerror.CodeUnspecified
	}
}

func commandToProto(command controller.AgentCommand) *agentv1.OpenCommandStreamResponse {
	out := &agentv1.OpenCommandStreamResponse{
		SessionId: command.SessionID,
		CommandId: command.CommandID,
		IssuedAt:  timestamppb.New(command.IssuedAt),
	}
	switch command.Kind {
	case controller.AgentCommandKindPing:
		out.Payload = &agentv1.OpenCommandStreamResponse_Ping{
			Ping: &agentv1.PingCommand{},
		}
	case controller.AgentCommandKindExecuteQuery:
		out.Payload = &agentv1.OpenCommandStreamResponse_ExecuteQuery{
			ExecuteQuery: &agentv1.ExecuteQueryCommand{Sql: command.SQL},
		}
	case controller.AgentCommandKindIntrospectSchema:
		out.Payload = &agentv1.OpenCommandStreamResponse_IntrospectSchema{
			IntrospectSchema: &agentv1.IntrospectSchemaCommand{},
		}
	case controller.AgentCommandKindConfigureDatabase:
		out.Payload = &agentv1.OpenCommandStreamResponse_ConfigureDatabase{
			ConfigureDatabase: &agentv1.ConfigureDatabaseCommand{
				Dsn: command.DSN,
			},
		}
	}
	return out
}

func configureDatabaseErrorCodeFromProto(
	code agentv1.ConfigureDatabaseErrorCode,
) controller.AgentConfigureDatabaseErrorCode {
	switch code {
	case agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_INVALID_DSN:
		return controller.AgentConfigureDatabaseErrorCodeInvalidDSN
	case agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_CONNECT_FAILED:
		return controller.AgentConfigureDatabaseErrorCodeConnectFailed
	case agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_AUTH_FAILED:
		return controller.AgentConfigureDatabaseErrorCodeAuthFailed
	case agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_PRIVILEGE_INVALID:
		return controller.AgentConfigureDatabaseErrorCodePrivilegeError
	case agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_WRITE_CONFIG_FAILED:
		return controller.AgentConfigureDatabaseErrorCodeWriteConfig
	case agentv1.ConfigureDatabaseErrorCode_CONFIGURE_DATABASE_ERROR_CODE_TIMEOUT:
		return controller.AgentConfigureDatabaseErrorCodeTimeout
	default:
		return controller.AgentConfigureDatabaseErrorCodeUnspecified
	}
}

func connectErrorForSession(err error) error {
	switch {
	case errors.Is(err, controller.ErrInvalidSessionID),
		errors.Is(err, controller.ErrInvalidHostname):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, controller.ErrSessionNotFound),
		errors.Is(err, controller.ErrCommandNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, controller.ErrSessionNotActive),
		errors.Is(err, controller.ErrCommandRejected),
		errors.Is(err, controller.ErrTenantNotConnected):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func protoRowsToMaps(rows []*agentv1.ExecuteQueryRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		values := make(map[string]any, len(row.GetValues()))
		for key, value := range row.GetValues() {
			values[key] = value.AsInterface()
		}
		out = append(out, values)
	}
	return out
}

func protoSchemaToModel(in *agentv1.SchemaBlob) model.SchemaBlob {
	if in == nil {
		return model.SchemaBlob{}
	}

	out := model.SchemaBlob{
		DatabaseName: in.GetDatabaseName(),
		Tables:       make([]model.SchemaTable, 0, len(in.GetTables())),
		Columns:      make([]model.SchemaColumn, 0, len(in.GetColumns())),
		PrimaryKeys:  make([]model.SchemaPrimaryKey, 0, len(in.GetPrimaryKeys())),
		ForeignKeys:  make([]model.SchemaForeignKey, 0, len(in.GetForeignKeys())),
	}
	for _, table := range in.GetTables() {
		out.Tables = append(out.Tables, model.SchemaTable{
			TableSchema:  table.GetTableSchema(),
			TableName:    table.GetTableName(),
			TableType:    table.GetTableType(),
			TableComment: table.GetTableComment(),
		})
	}
	for _, column := range in.GetColumns() {
		out.Columns = append(out.Columns, model.SchemaColumn{
			TableSchema:     column.GetTableSchema(),
			TableName:       column.GetTableName(),
			ColumnName:      column.GetColumnName(),
			OrdinalPosition: column.GetOrdinalPosition(),
			DataType:        column.GetDataType(),
			ColumnType:      column.GetColumnType(),
			IsNullable:      column.GetIsNullable(),
			HasDefault:      column.GetHasDefault(),
			DefaultValue:    column.GetDefaultValue(),
			ColumnComment:   column.GetColumnComment(),
		})
	}
	for _, key := range in.GetPrimaryKeys() {
		out.PrimaryKeys = append(out.PrimaryKeys, model.SchemaPrimaryKey{
			TableSchema:     key.GetTableSchema(),
			TableName:       key.GetTableName(),
			ConstraintName:  key.GetConstraintName(),
			ColumnName:      key.GetColumnName(),
			OrdinalPosition: key.GetOrdinalPosition(),
		})
	}
	for _, key := range in.GetForeignKeys() {
		out.ForeignKeys = append(out.ForeignKeys, model.SchemaForeignKey{
			TableSchema:           key.GetTableSchema(),
			TableName:             key.GetTableName(),
			ConstraintName:        key.GetConstraintName(),
			ColumnName:            key.GetColumnName(),
			OrdinalPosition:       key.GetOrdinalPosition(),
			ReferencedTableSchema: key.GetReferencedTableSchema(),
			ReferencedTableName:   key.GetReferencedTableName(),
			ReferencedColumnName:  key.GetReferencedColumnName(),
		})
	}
	return out
}

type AgentDebugHandler struct {
	sessions *controller.AgentSessionManager
}

func NewAgentDebugHandler(
	sessions *controller.AgentSessionManager,
) *AgentDebugHandler {
	return &AgentDebugHandler{sessions: sessions}
}

func (h *AgentDebugHandler) ListSessions(
	w http.ResponseWriter,
	_ *http.Request,
) {
	type sessionResponse struct {
		SessionID       string     `json:"session_id"`
		TenantID        string     `json:"tenant_id"`
		TokenID         string     `json:"token_id"`
		TokenLabel      string     `json:"token_label"`
		Hostname        string     `json:"hostname"`
		AgentVersion    string     `json:"agent_version"`
		ConnectedAt     time.Time  `json:"connected_at"`
		LastHeartbeatAt time.Time  `json:"last_heartbeat_at"`
		DisconnectedAt  *time.Time `json:"disconnected_at,omitempty"`
		Status          string     `json:"status"`
	}

	snapshots := h.sessions.ListSessions()
	out := make([]sessionResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, sessionResponse{
			SessionID:       snapshot.SessionID,
			TenantID:        snapshot.TenantID.String(),
			TokenID:         snapshot.TokenID.String(),
			TokenLabel:      snapshot.TokenLabel,
			Hostname:        snapshot.Hostname,
			AgentVersion:    snapshot.AgentVersion,
			ConnectedAt:     snapshot.ConnectedAt,
			LastHeartbeatAt: snapshot.LastHeartbeatAt,
			DisconnectedAt:  snapshot.DisconnectedAt,
			Status:          snapshot.Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"agents": out})
}

func (h *AgentDebugHandler) PingSession(
	w http.ResponseWriter,
	r *http.Request,
) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing session id",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := h.sessions.Ping(ctx, sessionID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{
			"session_id":    result.SessionID,
			"command_id":    result.CommandID,
			"completed_at":  result.CompletedAt,
			"round_trip_ms": result.RoundTripMS,
		})
	case errors.Is(err, controller.ErrSessionNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, controller.ErrSessionNotActive),
		errors.Is(err, controller.ErrCommandRejected):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{
			"error": "ping timed out",
		})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
