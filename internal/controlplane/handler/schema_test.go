package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeSchemaCapturer struct {
	captureFn func(context.Context, uuid.UUID) (controller.SchemaCaptureResult, error)
}

func (f fakeSchemaCapturer) Capture(
	ctx context.Context,
	tenantID uuid.UUID,
) (controller.SchemaCaptureResult, error) {
	return f.captureFn(ctx, tenantID)
}

func TestSchemaDebugHandlerOwnerFlow(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	versionID := uuid.New()
	handler := NewSchemaDebugHandler(
		controller.NewTenantController(
			unitTenantStore{
				createFn: func(context.Context, string, string, string) (model.Tenant, error) {
					return model.Tenant{}, nil
				},
				listFn: func(context.Context, string) ([]model.Tenant, error) {
					return nil, nil
				},
				membershipFn: func(_ context.Context, gotTenantID uuid.UUID, clerkUserID string) (model.TenantUser, error) {
					if gotTenantID != tenantID {
						t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
					}
					if clerkUserID != "user_123" {
						t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
					}
					return model.TenantUser{
						TenantID:    tenantID,
						ClerkUserID: clerkUserID,
						Role:        model.RoleOwner,
					}, nil
				},
			},
			unitTokenStore{
				createFn: func(context.Context, uuid.UUID, string, []byte) (model.TenantToken, error) {
					return model.TenantToken{}, nil
				},
				listFn: func(context.Context, uuid.UUID) ([]model.TenantToken, error) {
					return nil, nil
				},
				revokeFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
			},
		),
		fakeSchemaCapturer{
			captureFn: func(context.Context, uuid.UUID) (controller.SchemaCaptureResult, error) {
				return controller.SchemaCaptureResult{
					VersionID:       versionID,
					Changed:         true,
					CapturedAt:      time.Unix(1_700_000_000, 0).UTC(),
					SchemaHash:      "hash-123",
					DatabaseName:    "mission_app",
					TableCount:      6,
					ColumnCount:     24,
					ForeignKeyCount: 4,
				}, nil
			},
		},
	)

	rec := httptest.NewRecorder()
	req := queryRequest(
		http.MethodPost,
		"/api/debug/tenants/"+tenantID.String()+"/schema/introspect",
		tenantID.String(),
		nil,
	)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "user_123"}))
	handler.Introspect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got := body["version_id"]; got != versionID.String() {
		t.Fatalf("version_id = %#v, want %q", got, versionID.String())
	}
	if got := body["changed"]; got != true {
		t.Fatalf("changed = %#v, want true", got)
	}
}

func TestSchemaDebugHandlerRejectsUnauthorizedUsers(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	makeHandler := func(role model.Role, membershipErr error) *SchemaDebugHandler {
		return NewSchemaDebugHandler(
			controller.NewTenantController(
				unitTenantStore{
					createFn: func(context.Context, string, string, string) (model.Tenant, error) {
						return model.Tenant{}, nil
					},
					listFn: func(context.Context, string) ([]model.Tenant, error) {
						return nil, nil
					},
					membershipFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
						if membershipErr != nil {
							return model.TenantUser{}, membershipErr
						}
						return model.TenantUser{
							TenantID: tenantID,
							Role:     role,
						}, nil
					},
				},
				unitTokenStore{
					createFn: func(context.Context, uuid.UUID, string, []byte) (model.TenantToken, error) {
						return model.TenantToken{}, nil
					},
					listFn: func(context.Context, uuid.UUID) ([]model.TenantToken, error) {
						return nil, nil
					},
					revokeFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
				},
			),
			fakeSchemaCapturer{
				captureFn: func(context.Context, uuid.UUID) (controller.SchemaCaptureResult, error) {
					return controller.SchemaCaptureResult{}, nil
				},
			},
		)
	}

	memberReq := queryRequest(http.MethodPost, "/api/debug/tenants/"+tenantID.String()+"/schema/introspect", tenantID.String(), nil)
	memberReq = memberReq.WithContext(auth.WithUser(memberReq.Context(), auth.User{ID: "user_123"}))
	memberRec := httptest.NewRecorder()
	makeHandler(model.RoleMember, nil).Introspect(memberRec, memberReq)
	if memberRec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", memberRec.Code)
	}

	nonMemberReq := queryRequest(http.MethodPost, "/api/debug/tenants/"+tenantID.String()+"/schema/introspect", tenantID.String(), nil)
	nonMemberReq = nonMemberReq.WithContext(auth.WithUser(nonMemberReq.Context(), auth.User{ID: "user_123"}))
	nonMemberRec := httptest.NewRecorder()
	makeHandler(model.RoleOwner, repository.ErrNotFound).Introspect(nonMemberRec, nonMemberReq)
	if nonMemberRec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", nonMemberRec.Code)
	}
}

func TestSchemaDebugHandlerMapsControllerErrors(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	makeHandler := func(captureErr error) *SchemaDebugHandler {
		return NewSchemaDebugHandler(
			controller.NewTenantController(
				unitTenantStore{
					createFn: func(context.Context, string, string, string) (model.Tenant, error) {
						return model.Tenant{}, nil
					},
					listFn: func(context.Context, string) ([]model.Tenant, error) {
						return nil, nil
					},
					membershipFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
						return model.TenantUser{
							TenantID: tenantID,
							Role:     model.RoleOwner,
						}, nil
					},
				},
				unitTokenStore{
					createFn: func(context.Context, uuid.UUID, string, []byte) (model.TenantToken, error) {
						return model.TenantToken{}, nil
					},
					listFn: func(context.Context, uuid.UUID) ([]model.TenantToken, error) {
						return nil, nil
					},
					revokeFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
				},
			),
			fakeSchemaCapturer{
				captureFn: func(context.Context, uuid.UUID) (controller.SchemaCaptureResult, error) {
					return controller.SchemaCaptureResult{}, captureErr
				},
			},
		)
	}

	req := queryRequest(http.MethodPost, "/api/debug/tenants/"+tenantID.String()+"/schema/introspect", tenantID.String(), nil)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "user_123"}))

	conflictRec := httptest.NewRecorder()
	makeHandler(controller.ErrTenantNotConnected).Introspect(conflictRec, req)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", conflictRec.Code)
	}

	badGatewayRec := httptest.NewRecorder()
	makeHandler(controller.ErrAgentSchemaIntrospectionFailed).Introspect(badGatewayRec, req)
	if badGatewayRec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", badGatewayRec.Code)
	}

	internalRec := httptest.NewRecorder()
	makeHandler(errors.New("boom")).Introspect(internalRec, req)
	if internalRec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", internalRec.Code)
	}
}
