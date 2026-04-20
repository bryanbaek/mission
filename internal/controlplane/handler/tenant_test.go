package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	tenantv1 "github.com/bryanbaek/mission/gen/go/tenant/v1"
	"github.com/bryanbaek/mission/gen/go/tenant/v1/tenantv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

// --- fakes for controller deps ---

type fakeTenantStore struct {
	tenants     map[string][]model.Tenant   // clerkUserID → tenants
	memberships map[string]model.TenantUser // "tenantID:clerkUserID" → membership
}

func newFakeTenantStore() *fakeTenantStore {
	return &fakeTenantStore{
		tenants:     make(map[string][]model.Tenant),
		memberships: make(map[string]model.TenantUser),
	}
}

func (f *fakeTenantStore) CreateWithOwner(_ context.Context, slug, name, ownerClerkID string) (model.Tenant, error) {
	t := model.Tenant{ID: uuid.New(), Slug: slug, Name: name}
	f.tenants[ownerClerkID] = append(f.tenants[ownerClerkID], t)
	f.memberships[t.ID.String()+":"+ownerClerkID] = model.TenantUser{
		TenantID: t.ID, ClerkUserID: ownerClerkID, Role: model.RoleOwner,
	}
	return t, nil
}

func (f *fakeTenantStore) ListForUser(_ context.Context, clerkUserID string) ([]model.Tenant, error) {
	return f.tenants[clerkUserID], nil
}

func (f *fakeTenantStore) GetMembership(_ context.Context, tenantID uuid.UUID, clerkUserID string) (model.TenantUser, error) {
	key := tenantID.String() + ":" + clerkUserID
	if m, ok := f.memberships[key]; ok {
		return m, nil
	}
	return model.TenantUser{}, repository.ErrNotFound
}

type fakeTenantTokenStore struct {
	tokens map[uuid.UUID][]model.TenantToken
}

func newFakeTenantTokenStore() *fakeTenantTokenStore {
	return &fakeTenantTokenStore{tokens: make(map[uuid.UUID][]model.TenantToken)}
}

func (f *fakeTenantTokenStore) Create(_ context.Context, tenantID uuid.UUID, label string, _ []byte) (model.TenantToken, error) {
	tt := model.TenantToken{ID: uuid.New(), TenantID: tenantID, Label: label}
	f.tokens[tenantID] = append(f.tokens[tenantID], tt)
	return tt, nil
}

func (f *fakeTenantTokenStore) ListByTenant(_ context.Context, tenantID uuid.UUID) ([]model.TenantToken, error) {
	return f.tokens[tenantID], nil
}

func (f *fakeTenantTokenStore) Revoke(_ context.Context, tenantID, tokenID uuid.UUID) error {
	for _, tt := range f.tokens[tenantID] {
		if tt.ID == tokenID {
			return nil
		}
	}
	return repository.ErrNotFound
}

// --- helper: set up server + client ---

type testEnv struct {
	url   string
	store *fakeTenantStore
}

func setupTestServer(t *testing.T, verifier auth.Verifier) testEnv {
	t.Helper()

	tenantStore := newFakeTenantStore()
	tokenStore := newFakeTenantTokenStore()
	ctrl := controller.NewTenantController(tenantStore, tokenStore)
	h := NewTenantHandler(ctrl)

	mux := http.NewServeMux()
	path, svc := tenantv1connect.NewTenantServiceHandler(h)
	mux.Handle(path, auth.RequireAuth(verifier)(svc))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return testEnv{url: srv.URL, store: tenantStore}
}

func clientFor(url, token string) tenantv1connect.TenantServiceClient {
	return tenantv1connect.NewTenantServiceClient(
		http.DefaultClient,
		url,
		connect.WithInterceptors(&bearerInterceptor{token: token}),
	)
}

type bearerInterceptor struct {
	token string
}

func (b *bearerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+b.token)
		return next(ctx, req)
	}
}

func (b *bearerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (b *bearerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// --- tests ---

func TestCreateAndListTenants(t *testing.T) {
	verifier := &auth.FakeVerifier{Tokens: map[string]auth.User{
		"user-a-token": {ID: "user_a"},
		"user-b-token": {ID: "user_b"},
	}}
	env := setupTestServer(t, verifier)
	clientA := clientFor(env.url, "user-a-token")
	clientB := clientFor(env.url, "user-b-token")

	// User A creates a tenant
	createResp, err := clientA.CreateTenant(
		context.Background(),
		connect.NewRequest(&tenantv1.CreateTenantRequest{Slug: "acme-corp", Name: "Acme Corp"}),
	)
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if createResp.Msg.Tenant.Slug != "acme-corp" {
		t.Fatalf("expected slug=acme-corp, got %s", createResp.Msg.Tenant.Slug)
	}

	// User A can list their tenant
	listResp, err := clientA.ListTenants(
		context.Background(),
		connect.NewRequest(&tenantv1.ListTenantsRequest{}),
	)
	if err != nil {
		t.Fatalf("ListTenants user_a: %v", err)
	}
	if len(listResp.Msg.Tenants) != 1 {
		t.Fatalf("expected 1 tenant for user_a, got %d", len(listResp.Msg.Tenants))
	}

	// User B sees zero tenants — cross-tenant isolation
	listRespB, err := clientB.ListTenants(
		context.Background(),
		connect.NewRequest(&tenantv1.ListTenantsRequest{}),
	)
	if err != nil {
		t.Fatalf("ListTenants user_b: %v", err)
	}
	if len(listRespB.Msg.Tenants) != 0 {
		t.Fatalf("expected 0 tenants for user_b, got %d", len(listRespB.Msg.Tenants))
	}
}

func TestCrossTenantTokenIsolation(t *testing.T) {
	verifier := &auth.FakeVerifier{Tokens: map[string]auth.User{
		"user-a-token": {ID: "user_a"},
		"user-b-token": {ID: "user_b"},
	}}
	env := setupTestServer(t, verifier)
	clientA := clientFor(env.url, "user-a-token")
	clientB := clientFor(env.url, "user-b-token")

	// User A creates a tenant and issues a token
	createResp, err := clientA.CreateTenant(
		context.Background(),
		connect.NewRequest(&tenantv1.CreateTenantRequest{Slug: "water-co", Name: "Water Co"}),
	)
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	tenantID := createResp.Msg.Tenant.Id

	issueResp, err := clientA.IssueAgentToken(
		context.Background(),
		connect.NewRequest(&tenantv1.IssueAgentTokenRequest{TenantId: tenantID, Label: "edge-1"}),
	)
	if err != nil {
		t.Fatalf("IssueAgentToken: %v", err)
	}
	if issueResp.Msg.Plaintext == "" {
		t.Fatal("expected plaintext token")
	}

	// User B cannot list tokens for User A's tenant
	_, err = clientB.ListAgentTokens(
		context.Background(),
		connect.NewRequest(&tenantv1.ListAgentTokensRequest{TenantId: tenantID}),
	)
	if err == nil {
		t.Fatal("expected permission denied for user_b listing user_a's tokens")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodePermissionDenied {
		t.Fatalf("expected CodePermissionDenied, got %v", err)
	}

	// User B cannot revoke User A's token either
	_, err = clientB.RevokeAgentToken(
		context.Background(),
		connect.NewRequest(&tenantv1.RevokeAgentTokenRequest{
			TenantId: tenantID,
			TokenId:  issueResp.Msg.Token.Id,
		}),
	)
	if err == nil {
		t.Fatal("expected permission denied for user_b revoking user_a's token")
	}
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodePermissionDenied {
		t.Fatalf("expected CodePermissionDenied, got %v", err)
	}
}

func TestUnauthenticatedRejected(t *testing.T) {
	verifier := &auth.FakeVerifier{Tokens: map[string]auth.User{}}
	env := setupTestServer(t, verifier)
	client := clientFor(env.url, "bad-token")

	_, err := client.CreateTenant(
		context.Background(),
		connect.NewRequest(&tenantv1.CreateTenantRequest{Slug: "test", Name: "Test"}),
	)
	if err == nil {
		t.Fatal("expected error for unauthenticated request")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeUnauthenticated {
		t.Fatalf("expected CodeUnauthenticated, got %v", err)
	}
}
