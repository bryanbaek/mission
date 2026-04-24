package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type schemaMembershipChecker interface {
	EnsureMembership(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (model.TenantUser, error)
}

type schemaCapturer interface {
	Capture(
		ctx context.Context,
		tenantID uuid.UUID,
	) (controller.SchemaCaptureResult, error)
}

type SchemaDebugHandler struct {
	tenants schemaMembershipChecker
	schemas schemaCapturer
}

func NewSchemaDebugHandler(
	tenants schemaMembershipChecker,
	schemas schemaCapturer,
) *SchemaDebugHandler {
	return &SchemaDebugHandler{tenants: tenants, schemas: schemas}
}

func (h *SchemaDebugHandler) Introspect(
	w http.ResponseWriter,
	r *http.Request,
) {
	tenantID, ok := h.authorizeOwner(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	result, err := h.schemas.Capture(ctx, tenantID)
	switch {
	case errors.Is(err, controller.ErrTenantNotConnected),
		errors.Is(err, controller.ErrSessionNotActive),
		errors.Is(err, controller.ErrCommandRejected):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	case errors.Is(err, controller.ErrAgentSchemaIntrospectionFailed):
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{
			"error": "schema introspection timed out",
		})
		return
	case err != nil:
		slog.Error("schema introspection failed", "tenant_id", tenantID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal server error",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version_id":        result.VersionID.String(),
		"changed":           result.Changed,
		"captured_at":       result.CapturedAt,
		"schema_hash":       result.SchemaHash,
		"database_name":     result.DatabaseName,
		"table_count":       result.TableCount,
		"column_count":      result.ColumnCount,
		"foreign_key_count": result.ForeignKeyCount,
	})
}

func (h *SchemaDebugHandler) authorizeOwner(
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
