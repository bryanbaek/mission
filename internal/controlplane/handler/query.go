package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
	"github.com/bryanbaek/mission/internal/queryerror"
)

type queryMembershipChecker interface {
	EnsureMembership(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (model.TenantUser, error)
}

type querySessionManager interface {
	ExecuteQuery(
		ctx context.Context,
		tenantID uuid.UUID,
		sql string,
	) (controller.AgentExecuteQueryResult, error)
	LatestSessionForTenant(
		tenantID uuid.UUID,
	) (controller.AgentSessionSnapshot, bool)
}

type QueryDebugHandler struct {
	tenants  queryMembershipChecker
	sessions querySessionManager
}

func NewQueryDebugHandler(
	tenants queryMembershipChecker,
	sessions querySessionManager,
) *QueryDebugHandler {
	return &QueryDebugHandler{tenants: tenants, sessions: sessions}
}

func (h *QueryDebugHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.authorizeOwner(w, r)
	if !ok {
		return
	}

	snapshot, found := h.sessions.LatestSessionForTenant(tenantID)
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "offline",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            snapshot.Status,
		"session_id":        snapshot.SessionID,
		"hostname":          snapshot.Hostname,
		"agent_version":     snapshot.AgentVersion,
		"token_label":       snapshot.TokenLabel,
		"connected_at":      snapshot.ConnectedAt,
		"last_heartbeat_at": snapshot.LastHeartbeatAt,
	})
}

func (h *QueryDebugHandler) ExecuteQuery(
	w http.ResponseWriter,
	r *http.Request,
) {
	tenantID, ok := h.authorizeOwner(w, r)
	if !ok {
		return
	}

	var request struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid json body",
		})
		return
	}

	request.SQL = strings.TrimSpace(request.SQL)
	if request.SQL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "sql is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	result, err := h.sessions.ExecuteQuery(ctx, tenantID, request.SQL)
	switch {
	case errors.Is(err, controller.ErrTenantNotConnected),
		errors.Is(err, controller.ErrSessionNotActive),
		errors.Is(err, controller.ErrCommandRejected):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{
			"error": "query timed out",
		})
		return
	case err != nil:
		slog.Error("execute query failed", "tenant_id", tenantID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal server error",
		})
		return
	}

	if result.ErrorCode == queryerror.CodePermissionDenied {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":              firstNonEmpty(result.ErrorReason, result.Error),
			"error_code":         result.ErrorCode,
			"blocked_constructs": result.BlockedConstructs,
		})
		return
	}

	if result.ErrorCode == queryerror.CodeInternal {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":      firstNonEmpty(result.ErrorReason, result.Error),
			"error_code": result.ErrorCode,
		})
		return
	}

	if result.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": result.Error,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":    result.SessionID,
		"command_id":    result.CommandID,
		"completed_at":  result.CompletedAt,
		"columns":       result.Columns,
		"rows":          result.Rows,
		"elapsed_ms":    result.ElapsedMS,
		"database_user": result.DatabaseUser,
		"database_name": result.DatabaseName,
	})
}

func (h *QueryDebugHandler) authorizeOwner(
	w http.ResponseWriter,
	r *http.Request,
) (uuid.UUID, bool) {
	user, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "unauthenticated",
		})
		return uuid.UUID{}, false
	}

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid tenant id",
		})
		return uuid.UUID{}, false
	}

	membership, err := h.tenants.EnsureMembership(r.Context(), tenantID, user.ID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "not a member of this tenant",
		})
		return uuid.UUID{}, false
	case err != nil:
		slog.Error("membership check failed", "tenant_id", tenantID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal server error",
		})
		return uuid.UUID{}, false
	}

	if membership.Role != model.RoleOwner {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "owner role required",
		})
		return uuid.UUID{}, false
	}

	return tenantID, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
