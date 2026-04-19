package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	tenantv1 "github.com/bryanbaek/mission/gen/go/tenant/v1"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type unitTenantStore struct {
	createFn     func(context.Context, string, string, string) (model.Tenant, error)
	listFn       func(context.Context, string) ([]model.Tenant, error)
	membershipFn func(context.Context, uuid.UUID, string) (model.TenantUser, error)
}

func (s unitTenantStore) CreateWithOwner(
	ctx context.Context,
	slug, name, ownerClerkID string,
) (model.Tenant, error) {
	return s.createFn(ctx, slug, name, ownerClerkID)
}

func (s unitTenantStore) ListForUser(
	ctx context.Context,
	clerkUserID string,
) ([]model.Tenant, error) {
	return s.listFn(ctx, clerkUserID)
}

func (s unitTenantStore) GetMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (model.TenantUser, error) {
	return s.membershipFn(ctx, tenantID, clerkUserID)
}

type unitTokenStore struct {
	createFn func(context.Context, uuid.UUID, string, []byte) (model.TenantToken, error)
	listFn   func(context.Context, uuid.UUID) ([]model.TenantToken, error)
	revokeFn func(context.Context, uuid.UUID, uuid.UUID) error
}

func (s unitTokenStore) Create(
	ctx context.Context,
	tenantID uuid.UUID,
	label string,
	tokenHash []byte,
) (model.TenantToken, error) {
	return s.createFn(ctx, tenantID, label, tokenHash)
}

func (s unitTokenStore) ListByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.TenantToken, error) {
	return s.listFn(ctx, tenantID)
}

func (s unitTokenStore) Revoke(
	ctx context.Context,
	tenantID, tokenID uuid.UUID,
) error {
	return s.revokeFn(ctx, tenantID, tokenID)
}

func testTenantHandler(
	tenantStore unitTenantStore,
	tokenStore unitTokenStore,
) *TenantHandler {
	ctrl := controller.NewTenantController(tenantStore, tokenStore)
	return NewTenantHandler(ctrl)
}

func userContext() context.Context {
	return auth.WithUser(context.Background(), auth.User{ID: "user_123"})
}

func requireConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("err = %v, want connect error", err)
	}
	if connectErr.Code() != want {
		t.Fatalf("code = %v, want %v", connectErr.Code(), want)
	}
}

func TestTenantHandlerValidationErrors(t *testing.T) {
	t.Parallel()

	handler := testTenantHandler(
		unitTenantStore{
			createFn: func(context.Context, string, string, string) (model.Tenant, error) {
				return model.Tenant{}, nil
			},
			listFn: func(context.Context, string) ([]model.Tenant, error) {
				return nil, nil
			},
			membershipFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{}, nil
			},
		},
		unitTokenStore{
			createFn: func(context.Context, uuid.UUID, string, []byte) (model.TenantToken, error) {
				return model.TenantToken{}, nil
			},
			listFn: func(context.Context, uuid.UUID) ([]model.TenantToken, error) {
				return nil, nil
			},
			revokeFn: func(context.Context, uuid.UUID, uuid.UUID) error {
				return nil
			},
		},
	)

	_, err := handler.CreateTenant(
		context.Background(),
		connect.NewRequest(&tenantv1.CreateTenantRequest{}),
	)
	requireConnectCode(t, err, connect.CodeUnauthenticated)

	_, err = handler.ListTenants(
		context.Background(),
		connect.NewRequest(&tenantv1.ListTenantsRequest{}),
	)
	requireConnectCode(t, err, connect.CodeUnauthenticated)

	_, err = handler.IssueAgentToken(
		userContext(),
		connect.NewRequest(&tenantv1.IssueAgentTokenRequest{TenantId: "bad"}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = handler.ListAgentTokens(
		userContext(),
		connect.NewRequest(&tenantv1.ListAgentTokensRequest{TenantId: "bad"}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = handler.RevokeAgentToken(
		userContext(),
		connect.NewRequest(&tenantv1.RevokeAgentTokenRequest{
			TenantId: uuid.NewString(),
			TokenId:  "bad",
		}),
	)
	requireConnectCode(t, err, connect.CodeInvalidArgument)
}

func TestTenantHandlerTokenFlows(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	tokenID := uuid.New()
	createdAt := time.Unix(1_700_000_000, 0).UTC()
	lastUsed := createdAt.Add(time.Minute)
	revoked := createdAt.Add(2 * time.Minute)

	handler := testTenantHandler(
		unitTenantStore{
			createFn: func(context.Context, string, string, string) (model.Tenant, error) {
				return model.Tenant{}, nil
			},
			listFn: func(context.Context, string) ([]model.Tenant, error) {
				return nil, nil
			},
			membershipFn: func(
				_ context.Context,
				gotTenantID uuid.UUID,
				clerkUserID string,
			) (model.TenantUser, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				if clerkUserID != "user_123" {
					t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
				}
				return model.TenantUser{TenantID: tenantID}, nil
			},
		},
		unitTokenStore{
			createFn: func(
				_ context.Context,
				gotTenantID uuid.UUID,
				label string,
				_ []byte,
			) (model.TenantToken, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				if label != "edge-1" {
					t.Fatalf("label = %q, want edge-1", label)
				}
				return model.TenantToken{
					ID:         tokenID,
					TenantID:   tenantID,
					Label:      label,
					CreatedAt:  createdAt,
					LastUsedAt: &lastUsed,
					RevokedAt:  &revoked,
				}, nil
			},
			listFn: func(
				_ context.Context,
				gotTenantID uuid.UUID,
			) ([]model.TenantToken, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				return []model.TenantToken{{
					ID:         tokenID,
					TenantID:   tenantID,
					Label:      "edge-1",
					CreatedAt:  createdAt,
					LastUsedAt: &lastUsed,
					RevokedAt:  &revoked,
				}}, nil
			},
			revokeFn: func(
				_ context.Context,
				gotTenantID, gotTokenID uuid.UUID,
			) error {
				if gotTenantID != tenantID || gotTokenID != tokenID {
					t.Fatalf(
						"revoke IDs = (%s, %s), want (%s, %s)",
						gotTenantID,
						gotTokenID,
						tenantID,
						tokenID,
					)
				}
				return nil
			},
		},
	)

	issueResp, err := handler.IssueAgentToken(
		userContext(),
		connect.NewRequest(&tenantv1.IssueAgentTokenRequest{
			TenantId: tenantID.String(),
			Label:    "edge-1",
		}),
	)
	if err != nil {
		t.Fatalf("IssueAgentToken returned error: %v", err)
	}
	if issueResp.Msg.Plaintext == "" {
		t.Fatal("expected plaintext token")
	}
	if issueResp.Msg.Token.Id != tokenID.String() {
		t.Fatalf("token id = %q, want %s", issueResp.Msg.Token.Id, tokenID)
	}

	listResp, err := handler.ListAgentTokens(
		userContext(),
		connect.NewRequest(&tenantv1.ListAgentTokensRequest{
			TenantId: tenantID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("ListAgentTokens returned error: %v", err)
	}
	if got := len(listResp.Msg.Tokens); got != 1 {
		t.Fatalf("token count = %d, want 1", got)
	}
	if listResp.Msg.Tokens[0].LastUsedAt == nil {
		t.Fatal("expected last_used_at to be populated")
	}
	if listResp.Msg.Tokens[0].RevokedAt == nil {
		t.Fatal("expected revoked_at to be populated")
	}

	_, err = handler.RevokeAgentToken(
		userContext(),
		connect.NewRequest(&tenantv1.RevokeAgentTokenRequest{
			TenantId: tenantID.String(),
			TokenId:  tokenID.String(),
		}),
	)
	if err != nil {
		t.Fatalf("RevokeAgentToken returned error: %v", err)
	}
}

func TestTenantHandlerErrorHelpers(t *testing.T) {
	t.Parallel()

	requireConnectCode(
		t,
		membershipError(repository.ErrNotFound),
		connect.CodePermissionDenied,
	)
	requireConnectCode(
		t,
		membershipError(errors.New("db down")),
		connect.CodeInternal,
	)
	requireConnectCode(
		t,
		toConnectError(controller.ErrInvalidSlug),
		connect.CodeInvalidArgument,
	)
	requireConnectCode(
		t,
		toConnectError(repository.ErrNotFound),
		connect.CodeNotFound,
	)
	requireConnectCode(
		t,
		toConnectError(errors.New("boom")),
		connect.CodeInternal,
	)
}
