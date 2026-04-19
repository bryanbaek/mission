package handler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	tenantv1 "github.com/bryanbaek/mission/gen/go/tenant/v1"
	"github.com/bryanbaek/mission/gen/go/tenant/v1/tenantv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type TenantHandler struct {
	tenantv1connect.UnimplementedTenantServiceHandler
	ctrl *controller.TenantController
}

func NewTenantHandler(ctrl *controller.TenantController) *TenantHandler {
	return &TenantHandler{ctrl: ctrl}
}

func (h *TenantHandler) CreateTenant(
	ctx context.Context,
	req *connect.Request[tenantv1.CreateTenantRequest],
) (*connect.Response[tenantv1.CreateTenantResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	t, err := h.ctrl.Create(ctx, user.ID, req.Msg.Slug, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenantv1.CreateTenantResponse{
		Tenant: tenantToProto(t),
	}), nil
}

func (h *TenantHandler) ListTenants(
	ctx context.Context,
	_ *connect.Request[tenantv1.ListTenantsRequest],
) (*connect.Response[tenantv1.ListTenantsResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenants, err := h.ctrl.ListForUser(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*tenantv1.Tenant, len(tenants))
	for i, t := range tenants {
		out[i] = tenantToProto(t)
	}
	return connect.NewResponse(&tenantv1.ListTenantsResponse{Tenants: out}), nil
}

func (h *TenantHandler) IssueAgentToken(
	ctx context.Context,
	req *connect.Request[tenantv1.IssueAgentTokenRequest],
) (*connect.Response[tenantv1.IssueAgentTokenResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}

	if _, err := h.ctrl.EnsureMembership(ctx, tenantID, user.ID); err != nil {
		return nil, membershipError(err)
	}

	rec, plaintext, err := h.ctrl.IssueAgentToken(ctx, tenantID, req.Msg.Label)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenantv1.IssueAgentTokenResponse{
		Token:     tokenToProto(rec),
		Plaintext: plaintext,
	}), nil
}

func (h *TenantHandler) ListAgentTokens(
	ctx context.Context,
	req *connect.Request[tenantv1.ListAgentTokensRequest],
) (*connect.Response[tenantv1.ListAgentTokensResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}

	if _, err := h.ctrl.EnsureMembership(ctx, tenantID, user.ID); err != nil {
		return nil, membershipError(err)
	}

	tokens, err := h.ctrl.ListAgentTokens(ctx, tenantID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*tenantv1.TenantTokenSummary, len(tokens))
	for i, tt := range tokens {
		out[i] = tokenToProto(tt)
	}
	return connect.NewResponse(&tenantv1.ListAgentTokensResponse{Tokens: out}), nil
}

func (h *TenantHandler) RevokeAgentToken(
	ctx context.Context,
	req *connect.Request[tenantv1.RevokeAgentTokenRequest],
) (*connect.Response[tenantv1.RevokeAgentTokenResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthenticated"),
		)
	}

	tenantID, err := uuid.Parse(req.Msg.TenantId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid tenant_id"),
		)
	}
	tokenID, err := uuid.Parse(req.Msg.TokenId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid token_id"),
		)
	}

	if _, err := h.ctrl.EnsureMembership(ctx, tenantID, user.ID); err != nil {
		return nil, membershipError(err)
	}

	if err := h.ctrl.RevokeAgentToken(ctx, tenantID, tokenID); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenantv1.RevokeAgentTokenResponse{}), nil
}

// --- converters ---

func tenantToProto(t model.Tenant) *tenantv1.Tenant {
	return &tenantv1.Tenant{
		Id:        t.ID.String(),
		Slug:      t.Slug,
		Name:      t.Name,
		CreatedAt: timestamppb.New(t.CreatedAt),
	}
}

func tokenToProto(tt model.TenantToken) *tenantv1.TenantTokenSummary {
	out := &tenantv1.TenantTokenSummary{
		Id:        tt.ID.String(),
		Label:     tt.Label,
		CreatedAt: timestamppb.New(tt.CreatedAt),
	}
	if tt.LastUsedAt != nil {
		out.LastUsedAt = timestamppb.New(*tt.LastUsedAt)
	}
	if tt.RevokedAt != nil {
		out.RevokedAt = timestamppb.New(*tt.RevokedAt)
	}
	return out
}

func membershipError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return connect.NewError(
			connect.CodePermissionDenied,
			errors.New("not a member of this tenant"),
		)
	}
	return connect.NewError(connect.CodeInternal, err)
}

func toConnectError(err error) error {
	switch {
	case errors.Is(err, controller.ErrInvalidSlug),
		errors.Is(err, controller.ErrInvalidName),
		errors.Is(err, controller.ErrInvalidLabel):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, repository.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
