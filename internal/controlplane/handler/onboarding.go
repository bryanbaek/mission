package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	onboardingv1 "github.com/bryanbaek/mission/gen/go/onboarding/v1"
	"github.com/bryanbaek/mission/gen/go/onboarding/v1/onboardingv1connect"
	"github.com/bryanbaek/mission/internal/controlplane/auth"
	"github.com/bryanbaek/mission/internal/controlplane/controller"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type onboardingController interface {
	ListWorkspaces(ctx context.Context, clerkUserID string) ([]model.OnboardingWorkspace, error)
	GetState(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (controller.OnboardingStateView, error)
	SaveWelcome(ctx context.Context, tenantID uuid.UUID, clerkUserID, workspaceName, primaryLanguage string, confirmPrimaryLanguage, autoSave bool) (controller.OnboardingStateView, error)
	EnsureInstallBundle(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (controller.OnboardingStateView, error)
	GetAgentConnectionStatus(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (controller.OnboardingStateView, error)
	ConfigureDatabase(ctx context.Context, tenantID uuid.UUID, clerkUserID, host string, port int32, databaseName, connectionString string) (controller.OnboardingStateView, error)
	RunSchemaIntrospection(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (controller.OnboardingStateView, error)
	MarkSemanticApproved(ctx context.Context, tenantID uuid.UUID, clerkUserID string, semanticLayerID uuid.UUID) (controller.OnboardingStateView, error)
	CompleteStarterStep(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (controller.OnboardingStateView, error)
	CompleteOnboarding(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (controller.OnboardingStateView, error)
	CreateInvites(ctx context.Context, tenantID uuid.UUID, clerkUserID string, emails []string) (controller.OnboardingStateView, []model.TenantInvite, error)
}

type OnboardingHandler struct {
	onboardingv1connect.UnimplementedOnboardingServiceHandler
	ctrl onboardingController
}

func NewOnboardingHandler(ctrl onboardingController) *OnboardingHandler {
	return &OnboardingHandler{ctrl: ctrl}
}

func (h *OnboardingHandler) ListWorkspaces(
	ctx context.Context,
	_ *connect.Request[onboardingv1.ListWorkspacesRequest],
) (*connect.Response[onboardingv1.ListWorkspacesResponse], error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	workspaces, err := h.ctrl.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	items := make([]*onboardingv1.WorkspaceSummary, 0, len(workspaces))
	for _, workspace := range workspaces {
		items = append(items, workspaceSummaryToProto(workspace))
	}
	return connect.NewResponse(&onboardingv1.ListWorkspacesResponse{
		Workspaces: items,
	}), nil
}

func (h *OnboardingHandler) GetState(
	ctx context.Context,
	req *connect.Request[onboardingv1.GetStateRequest],
) (*connect.Response[onboardingv1.GetStateResponse], error) {
	return connectStateResponse(
		ctx,
		req.Msg.GetTenantId(),
		func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
			return h.ctrl.GetState(ctx, tenantID, userID)
		},
	)
}

func (h *OnboardingHandler) SaveWelcome(
	ctx context.Context,
	req *connect.Request[onboardingv1.SaveWelcomeRequest],
) (*connect.Response[onboardingv1.SaveWelcomeResponse], error) {
	user, tenantID, err := requireOnboardingUserAndTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	state, ctrlErr := h.ctrl.SaveWelcome(
		ctx,
		tenantID,
		user.ID,
		req.Msg.GetWorkspaceName(),
		req.Msg.GetPrimaryLanguage(),
		req.Msg.GetConfirmPrimaryLanguage(),
		req.Msg.GetAutoSave(),
	)
	if ctrlErr != nil {
		return nil, onboardingError(ctrlErr)
	}
	return connect.NewResponse(&onboardingv1.SaveWelcomeResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) EnsureInstallBundle(
	ctx context.Context,
	req *connect.Request[onboardingv1.EnsureInstallBundleRequest],
) (*connect.Response[onboardingv1.EnsureInstallBundleResponse], error) {
	state, err := h.runStateOnly(ctx, req.Msg.GetTenantId(), func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
		return h.ctrl.EnsureInstallBundle(ctx, tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.EnsureInstallBundleResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) GetAgentConnectionStatus(
	ctx context.Context,
	req *connect.Request[onboardingv1.GetAgentConnectionStatusRequest],
) (*connect.Response[onboardingv1.GetAgentConnectionStatusResponse], error) {
	state, err := h.runStateOnly(ctx, req.Msg.GetTenantId(), func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
		return h.ctrl.GetAgentConnectionStatus(ctx, tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.GetAgentConnectionStatusResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) ConfigureDatabase(
	ctx context.Context,
	req *connect.Request[onboardingv1.ConfigureDatabaseRequest],
) (*connect.Response[onboardingv1.ConfigureDatabaseResponse], error) {
	state, err := h.runStateOnly(ctx, req.Msg.GetTenantId(), func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
		return h.ctrl.ConfigureDatabase(
			ctx,
			tenantID,
			userID,
			req.Msg.GetHost(),
			req.Msg.GetPort(),
			req.Msg.GetDatabaseName(),
			req.Msg.GetConnectionString(),
		)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.ConfigureDatabaseResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) RunSchemaIntrospection(
	ctx context.Context,
	req *connect.Request[onboardingv1.RunSchemaIntrospectionRequest],
) (*connect.Response[onboardingv1.RunSchemaIntrospectionResponse], error) {
	state, err := h.runStateOnly(ctx, req.Msg.GetTenantId(), func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
		return h.ctrl.RunSchemaIntrospection(ctx, tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.RunSchemaIntrospectionResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) MarkSemanticApproved(
	ctx context.Context,
	req *connect.Request[onboardingv1.MarkSemanticApprovedRequest],
) (*connect.Response[onboardingv1.MarkSemanticApprovedResponse], error) {
	user, tenantID, err := requireOnboardingUserAndTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	semanticLayerID, parseErr := uuid.Parse(req.Msg.GetSemanticLayerId())
	if parseErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid semantic_layer_id"))
	}
	state, ctrlErr := h.ctrl.MarkSemanticApproved(ctx, tenantID, user.ID, semanticLayerID)
	if ctrlErr != nil {
		return nil, onboardingError(ctrlErr)
	}
	return connect.NewResponse(&onboardingv1.MarkSemanticApprovedResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) CompleteStarterStep(
	ctx context.Context,
	req *connect.Request[onboardingv1.CompleteStarterStepRequest],
) (*connect.Response[onboardingv1.CompleteStarterStepResponse], error) {
	state, err := h.runStateOnly(ctx, req.Msg.GetTenantId(), func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
		return h.ctrl.CompleteStarterStep(ctx, tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.CompleteStarterStepResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) CompleteOnboarding(
	ctx context.Context,
	req *connect.Request[onboardingv1.CompleteOnboardingRequest],
) (*connect.Response[onboardingv1.CompleteOnboardingResponse], error) {
	state, err := h.runStateOnly(ctx, req.Msg.GetTenantId(), func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error) {
		return h.ctrl.CompleteOnboarding(ctx, tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&onboardingv1.CompleteOnboardingResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func (h *OnboardingHandler) CreateInvites(
	ctx context.Context,
	req *connect.Request[onboardingv1.CreateInvitesRequest],
) (*connect.Response[onboardingv1.CreateInvitesResponse], error) {
	user, tenantID, err := requireOnboardingUserAndTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	state, created, ctrlErr := h.ctrl.CreateInvites(ctx, tenantID, user.ID, req.Msg.GetEmails())
	if ctrlErr != nil {
		return nil, onboardingError(ctrlErr)
	}
	items := make([]*onboardingv1.InviteSummary, 0, len(created))
	for _, invite := range created {
		items = append(items, inviteToProto(invite))
	}
	return connect.NewResponse(&onboardingv1.CreateInvitesResponse{
		State:   onboardingStateToProto(state),
		Invites: items,
	}), nil
}

func (h *OnboardingHandler) runStateOnly(
	ctx context.Context,
	tenantIDRaw string,
	run func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error),
) (controller.OnboardingStateView, error) {
	user, tenantID, err := requireOnboardingUserAndTenant(ctx, tenantIDRaw)
	if err != nil {
		return controller.OnboardingStateView{}, err
	}
	state, ctrlErr := run(user.ID, tenantID)
	if ctrlErr != nil {
		return controller.OnboardingStateView{}, onboardingError(ctrlErr)
	}
	return state, nil
}

func connectStateResponse(
	ctx context.Context,
	tenantIDRaw string,
	run func(userID string, tenantID uuid.UUID) (controller.OnboardingStateView, error),
) (*connect.Response[onboardingv1.GetStateResponse], error) {
	user, tenantID, err := requireOnboardingUserAndTenant(ctx, tenantIDRaw)
	if err != nil {
		return nil, err
	}
	state, ctrlErr := run(user.ID, tenantID)
	if ctrlErr != nil {
		return nil, onboardingError(ctrlErr)
	}
	return connect.NewResponse(&onboardingv1.GetStateResponse{
		State: onboardingStateToProto(state),
	}), nil
}

func requireOnboardingUserAndTenant(
	ctx context.Context,
	tenantIDRaw string,
) (auth.User, uuid.UUID, error) {
	user, ok := auth.FromContext(ctx)
	if !ok {
		return auth.User{}, uuid.UUID{}, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw)
	if err != nil {
		return auth.User{}, uuid.UUID{}, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid tenant_id"))
	}
	return user, tenantID, nil
}

func onboardingError(err error) error {
	switch {
	case errors.Is(err, controller.ErrOnboardingAccessDenied),
		errors.Is(err, controller.ErrOnboardingOwnerRequired):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, controller.ErrOnboardingInvalidWorkspaceName),
		errors.Is(err, controller.ErrOnboardingPrimaryLanguageRequired),
		errors.Is(err, controller.ErrOnboardingInviteEmailInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, controller.ErrOnboardingInvalidStep),
		errors.Is(err, controller.ErrOnboardingStepLocked),
		errors.Is(err, controller.ErrOnboardingInvalidSemanticLayer),
		errors.Is(err, controller.ErrTenantNotConnected),
		errors.Is(err, controller.ErrSessionNotActive),
		errors.Is(err, controller.ErrCommandRejected),
		errors.Is(err, controller.ErrAgentSchemaIntrospectionFailed):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, repository.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func workspaceSummaryToProto(workspace model.OnboardingWorkspace) *onboardingv1.WorkspaceSummary {
	return &onboardingv1.WorkspaceSummary{
		TenantId:           workspace.TenantID.String(),
		Slug:               workspace.Slug,
		Name:               workspace.Name,
		Role:               workspaceRoleToProto(workspace.Role),
		OnboardingComplete: workspace.OnboardingComplete,
		CurrentStep:        workspace.CurrentStep,
		UpdatedAt:          timestampOrNil(workspace.UpdatedAt),
	}
}

func onboardingStateToProto(view controller.OnboardingStateView) *onboardingv1.OnboardingState {
	payload := view.Payload
	name := view.Workspace.Name
	if trimmed := strings.TrimSpace(payload.WorkspaceName); trimmed != "" {
		name = trimmed
	}
	out := &onboardingv1.OnboardingState{
		TenantId:                view.Workspace.TenantID.String(),
		Slug:                    view.Workspace.Slug,
		Name:                    name,
		Role:                    workspaceRoleToProto(view.Workspace.Role),
		OnboardingComplete:      view.State.CompletedAt != nil,
		CurrentStep:             view.State.CurrentStep,
		PrimaryLanguage:         payload.PrimaryLanguage,
		InstallSlug:             payload.InstallSlug,
		DockerRunCommand:        view.DockerRunCommand,
		AgentTokenId:            payload.AgentTokenID,
		AgentTokenPlaintext:     payload.AgentTokenPlain,
		AgentSessionId:          payload.AgentSessionID,
		AgentConnected:          view.AgentConnected,
		AgentWaitStartedAt:      timestampPtr(payload.AgentWaitStarted),
		AgentConnectedAt:        timestampPtr(payload.AgentConnectedAt),
		AgentConnectionTimedOut: view.AgentConnectionTimedOut,
		DbHost:                  payload.DatabaseHost,
		DbPort:                  payload.DatabasePort,
		DbName:                  payload.DatabaseName,
		DbUsername:              payload.DatabaseUsername,
		GeneratedPassword:       payload.GeneratedPassword,
		DbSetupSql:              view.DBSetupSQL,
		DbVerifiedAt:            timestampPtr(payload.DBVerifiedAt),
		DbErrorCode:             payload.DBErrorCode,
		DbErrorMessageKo:        payload.DBErrorMessageKO,
		SchemaVersionId:         payload.SchemaVersionID,
		SchemaTableCount:        payload.SchemaTableCount,
		SchemaColumnCount:       payload.SchemaColumnCount,
		SchemaForeignKeyCount:   payload.SchemaFKCount,
		SemanticLayerId:         payload.SemanticLayerID,
		SemanticApprovedAt:      timestampPtr(payload.SemanticApproved),
		UpdatedAt:               timestampOrNil(view.State.UpdatedAt),
		CanEdit:                 view.CanEdit,
		WaitingForOwner:         view.WaitingForOwner,
	}
	out.CompletedAt = timestampPtr(view.State.CompletedAt)
	out.Invites = make([]*onboardingv1.InviteSummary, 0, len(view.Invites))
	for _, invite := range view.Invites {
		out.Invites = append(out.Invites, inviteToProto(invite))
	}
	return out
}

func inviteToProto(invite model.TenantInvite) *onboardingv1.InviteSummary {
	return &onboardingv1.InviteSummary{
		Id:        invite.ID.String(),
		Email:     invite.Email,
		CreatedAt: timestampOrNil(invite.CreatedAt),
	}
}

func workspaceRoleToProto(role model.WorkspaceRole) onboardingv1.WorkspaceRole {
	switch role {
	case model.WorkspaceRoleOwner:
		return onboardingv1.WorkspaceRole_WORKSPACE_ROLE_OWNER
	case model.WorkspaceRoleMember:
		return onboardingv1.WorkspaceRole_WORKSPACE_ROLE_MEMBER
	default:
		return onboardingv1.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED
	}
}

func timestampOrNil(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func timestampPtr(value *time.Time) *timestamppb.Timestamp {
	if value == nil || value.IsZero() {
		return nil
	}
	return timestamppb.New(value.UTC())
}
