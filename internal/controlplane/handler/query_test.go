package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

func queryRequest(
	method string,
	target string,
	tenantID string,
	body []byte,
) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("tenantID", tenantID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	return req
}

func TestQueryDebugHandlerOwnerFlow(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	tenantID := uuid.New()
	token := model.TenantToken{
		ID:       uuid.New(),
		TenantID: tenantID,
		Label:    "edge-1",
	}
	sessions := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{
		Now: func() time.Time { return now },
	})
	stream, err := sessions.RegisterSession(token, "session-1", "host-a", "v1")
	if err != nil {
		t.Fatalf("RegisterSession returned error: %v", err)
	}

	handler := NewQueryDebugHandler(
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
		sessions,
	)

	statusRec := httptest.NewRecorder()
	statusReq := queryRequest(
		http.MethodGet,
		"/api/debug/tenants/"+tenantID.String()+"/query",
		tenantID.String(),
		nil,
	)
	statusReq = statusReq.WithContext(
		auth.WithUser(statusReq.Context(), auth.User{ID: "user_123"}),
	)
	handler.GetStatus(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", statusRec.Code)
	}
	var statusBody map[string]any
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusBody); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got := statusBody["status"]; got != "online" {
		t.Fatalf("status = %#v, want online", got)
	}

	errCh := make(chan error, 1)
	go func() {
		command := <-stream.Commands
		if command.SQL != "SELECT 1" {
			errCh <- errors.New("unexpected sql")
			return
		}
		errCh <- sessions.SubmitExecuteQueryResult(
			token.ID,
			"session-1",
			command.CommandID,
			now.Add(time.Second),
			[]string{"1"},
			[]map[string]any{{"1": int64(1)}},
			11,
			"mission_ro@%",
			"mission_app",
			"",
		)
	}()

	queryRec := httptest.NewRecorder()
	queryReq := queryRequest(
		http.MethodPost,
		"/api/debug/tenants/"+tenantID.String()+"/query",
		tenantID.String(),
		[]byte(`{"sql":"SELECT 1"}`),
	)
	queryReq = queryReq.WithContext(
		auth.WithUser(queryReq.Context(), auth.User{ID: "user_123"}),
	)
	handler.ExecuteQuery(queryRec, queryReq)

	if queryRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", queryRec.Code)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("SubmitExecuteQueryResult returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for query command")
	}

	var queryBody map[string]any
	if err := json.Unmarshal(queryRec.Body.Bytes(), &queryBody); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got := queryBody["database_name"]; got != "mission_app" {
		t.Fatalf("database_name = %#v, want mission_app", got)
	}
}

func TestQueryDebugHandlerRejectsNonOwnersAndNonMembers(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	sessions := controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{})

	makeHandler := func(role model.Role, membershipErr error) *QueryDebugHandler {
		return NewQueryDebugHandler(
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
			sessions,
		)
	}

	memberReq := queryRequest(http.MethodGet, "/api/debug/tenants/"+tenantID.String()+"/query", tenantID.String(), nil)
	memberReq = memberReq.WithContext(auth.WithUser(memberReq.Context(), auth.User{ID: "user_123"}))
	memberRec := httptest.NewRecorder()
	makeHandler(model.RoleMember, nil).GetStatus(memberRec, memberReq)
	if memberRec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", memberRec.Code)
	}

	nonMemberReq := queryRequest(http.MethodPost, "/api/debug/tenants/"+tenantID.String()+"/query", tenantID.String(), []byte(`{"sql":"SELECT 1"}`))
	nonMemberReq = nonMemberReq.WithContext(auth.WithUser(nonMemberReq.Context(), auth.User{ID: "user_123"}))
	nonMemberRec := httptest.NewRecorder()
	makeHandler(model.RoleOwner, repository.ErrNotFound).ExecuteQuery(nonMemberRec, nonMemberReq)
	if nonMemberRec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", nonMemberRec.Code)
	}
}

func TestQueryDebugHandlerOfflineAndValidationErrors(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := NewQueryDebugHandler(
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
		controller.NewAgentSessionManager(controller.AgentSessionManagerConfig{}),
	)

	statusRec := httptest.NewRecorder()
	statusReq := queryRequest(http.MethodGet, "/api/debug/tenants/"+tenantID.String()+"/query", tenantID.String(), nil)
	statusReq = statusReq.WithContext(auth.WithUser(statusReq.Context(), auth.User{ID: "user_123"}))
	handler.GetStatus(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", statusRec.Code)
	}

	queryRec := httptest.NewRecorder()
	queryReq := queryRequest(http.MethodPost, "/api/debug/tenants/"+tenantID.String()+"/query", tenantID.String(), []byte(`{"sql":"SELECT 1"}`))
	queryReq = queryReq.WithContext(auth.WithUser(queryReq.Context(), auth.User{ID: "user_123"}))
	handler.ExecuteQuery(queryRec, queryReq)
	if queryRec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", queryRec.Code)
	}

	emptyRec := httptest.NewRecorder()
	emptyReq := queryRequest(http.MethodPost, "/api/debug/tenants/"+tenantID.String()+"/query", tenantID.String(), []byte(`{"sql":"  "}`))
	emptyReq = emptyReq.WithContext(auth.WithUser(emptyReq.Context(), auth.User{ID: "user_123"}))
	handler.ExecuteQuery(emptyRec, emptyReq)
	if emptyRec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", emptyRec.Code)
	}
}
