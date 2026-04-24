package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

const (
	OnboardingStepWelcome       int32 = 1
	OnboardingStepAgentInstall  int32 = 2
	OnboardingStepDatabase      int32 = 3
	OnboardingStepSchema        int32 = 4
	OnboardingStepSemantic      int32 = 5
	OnboardingStepStarter       int32 = 6
	OnboardingStepDone          int32 = 7
	defaultDBPort               int32 = 3306
	defaultDatabaseUsername           = "okta_ai_ro"
	defaultOnboardingTokenLabel       = "onboarding-agent"
	defaultEdgeAgentVersion           = "v0.1.0"
	defaultEdgeAgentImage             = "registry.digitalocean.com/mission/edge-agent:v0.1.0"
)

var (
	ErrOnboardingAccessDenied            = errors.New("not a member of this tenant")
	ErrOnboardingOwnerRequired           = errors.New("owner role required")
	ErrOnboardingInvalidStep             = errors.New("onboarding step is not available yet")
	ErrOnboardingStepLocked              = errors.New("earlier onboarding steps are locked after approval")
	ErrOnboardingInvalidWorkspaceName    = errors.New("workspace name is required")
	ErrOnboardingPrimaryLanguageRequired = errors.New("primary language must be confirmed as Korean")
	ErrOnboardingInvalidSemanticLayer    = errors.New("semantic layer is not approved")
	ErrOnboardingDatabaseConnection      = errors.New("database configuration failed")
	ErrOnboardingInviteEmailInvalid      = errors.New("one or more invite emails are invalid")
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type onboardingStateStore interface {
	GetByTenant(ctx context.Context, tenantID uuid.UUID) (model.TenantOnboardingState, error)
	Upsert(
		ctx context.Context,
		tenantID uuid.UUID,
		currentStep int32,
		payload []byte,
		completedAt *time.Time,
	) (model.TenantOnboardingState, error)
	ListWorkspacesForUser(ctx context.Context, clerkUserID string) ([]model.OnboardingWorkspace, error)
}

type onboardingInviteStore interface {
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.TenantInvite, error)
	CreateMany(ctx context.Context, tenantID uuid.UUID, emails []string, createdByUserID string) ([]model.TenantInvite, error)
}

type onboardingTenantController interface {
	EnsureMembership(ctx context.Context, tenantID uuid.UUID, clerkUserID string) (model.TenantUser, error)
	IssueAgentToken(ctx context.Context, tenantID uuid.UUID, label string) (model.TenantToken, string, error)
	UpdateName(ctx context.Context, tenantID uuid.UUID, name string) (model.Tenant, error)
}

type onboardingAgentSessions interface {
	LatestSessionForTenant(tenantID uuid.UUID) (AgentSessionSnapshot, bool)
	LatestSessionForToken(tokenID uuid.UUID) (AgentSessionSnapshot, bool)
	ConfigureDatabase(ctx context.Context, tokenID uuid.UUID, dsn string) (AgentConfigureDatabaseResult, error)
}

type onboardingSchemaCapturer interface {
	Capture(ctx context.Context, tenantID uuid.UUID) (SchemaCaptureResult, error)
}

type onboardingSemanticStore interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (model.TenantSemanticLayer, error)
}

type OnboardingControllerConfig struct {
	Now                   func() time.Time
	EdgeAgentImage        string
	EdgeAgentVersion      string
	PublicControlPlaneURL string
}

type OnboardingStateView struct {
	Workspace               model.OnboardingWorkspace
	State                   model.TenantOnboardingState
	Payload                 model.OnboardingPayload
	DockerRunCommand        string
	AgentConnected          bool
	AgentConnectionTimedOut bool
	DBSetupSQL              string
	Invites                 []model.TenantInvite
	CanEdit                 bool
	WaitingForOwner         bool
}

type OnboardingController struct {
	states                onboardingStateStore
	invites               onboardingInviteStore
	tenants               onboardingTenantController
	sessions              onboardingAgentSessions
	schemas               onboardingSchemaCapturer
	semanticLayers        onboardingSemanticStore
	now                   func() time.Time
	edgeAgentImage        string
	edgeAgentVersion      string
	publicControlPlaneURL string
}

func NewOnboardingController(
	states onboardingStateStore,
	invites onboardingInviteStore,
	tenants onboardingTenantController,
	sessions onboardingAgentSessions,
	schemas onboardingSchemaCapturer,
	semanticLayers onboardingSemanticStore,
	cfg OnboardingControllerConfig,
) *OnboardingController {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	edgeAgentImage := strings.TrimSpace(cfg.EdgeAgentImage)
	if edgeAgentImage == "" {
		edgeAgentImage = defaultEdgeAgentImage
	}
	edgeAgentVersion := strings.TrimSpace(cfg.EdgeAgentVersion)
	if edgeAgentVersion == "" {
		edgeAgentVersion = defaultEdgeAgentVersion
	}
	publicControlPlaneURL := strings.TrimSpace(cfg.PublicControlPlaneURL)
	return &OnboardingController{
		states:                states,
		invites:               invites,
		tenants:               tenants,
		sessions:              sessions,
		schemas:               schemas,
		semanticLayers:        semanticLayers,
		now:                   now,
		edgeAgentImage:        edgeAgentImage,
		edgeAgentVersion:      edgeAgentVersion,
		publicControlPlaneURL: publicControlPlaneURL,
	}
}

func (c *OnboardingController) ListWorkspaces(
	ctx context.Context,
	clerkUserID string,
) ([]model.OnboardingWorkspace, error) {
	return c.states.ListWorkspacesForUser(ctx, clerkUserID)
}

func (c *OnboardingController) GetState(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	record, payload, workspace, err = c.refreshAgentConnection(ctx, workspace, record, payload)
	if err != nil {
		return OnboardingStateView{}, err
	}
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) SaveWelcome(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID, workspaceName, primaryLanguage string,
	confirmPrimaryLanguage, autoSave bool,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep >= OnboardingStepStarter {
		return OnboardingStateView{}, ErrOnboardingStepLocked
	}

	trimmedName := strings.TrimSpace(workspaceName)
	if !autoSave && trimmedName == "" {
		return OnboardingStateView{}, ErrOnboardingInvalidWorkspaceName
	}
	if confirmPrimaryLanguage && primaryLanguage != "ko" {
		return OnboardingStateView{}, ErrOnboardingPrimaryLanguageRequired
	}

	if trimmedName != "" {
		payload.WorkspaceName = trimmedName
		payload.InstallSlug = slugifyWorkspaceName(trimmedName, workspace.Slug)
	}
	if primaryLanguage != "" {
		payload.PrimaryLanguage = primaryLanguage
	}

	currentStep := record.CurrentStep
	if currentStep == 0 {
		currentStep = OnboardingStepWelcome
	}
	if confirmPrimaryLanguage {
		if payload.PrimaryLanguage != "ko" {
			return OnboardingStateView{}, ErrOnboardingPrimaryLanguageRequired
		}
		if payload.WorkspaceName == "" {
			return OnboardingStateView{}, ErrOnboardingInvalidWorkspaceName
		}
		if _, err := c.tenants.UpdateName(ctx, tenantID, payload.WorkspaceName); err != nil {
			return OnboardingStateView{}, err
		}
		workspace.Name = payload.WorkspaceName
		currentStep = OnboardingStepAgentInstall
	}

	record, payload, err = c.persistState(
		ctx,
		tenantID,
		currentStep,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.CurrentStep = record.CurrentStep
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) EnsureInstallBundle(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepAgentInstall {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	if record.CurrentStep >= OnboardingStepStarter {
		return OnboardingStateView{}, ErrOnboardingStepLocked
	}

	if payload.InstallSlug == "" {
		payload.InstallSlug = slugifyWorkspaceName(
			firstNonEmpty(payload.WorkspaceName, workspace.Name),
			workspace.Slug,
		)
	}
	if payload.AgentWaitStarted == nil {
		now := c.now().UTC()
		payload.AgentWaitStarted = &now
	}

	record, payload, workspace, err = c.refreshAgentConnection(ctx, workspace, record, payload)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if payload.AgentTokenID == "" {
		token, plaintext, err := c.tenants.IssueAgentToken(
			ctx,
			tenantID,
			defaultOnboardingTokenLabel,
		)
		if err != nil {
			return OnboardingStateView{}, err
		}
		payload.AgentTokenID = token.ID.String()
		payload.AgentTokenPlain = plaintext
	}

	currentStep := maxStep(record.CurrentStep, OnboardingStepAgentInstall)
	record, payload, err = c.persistState(
		ctx,
		tenantID,
		currentStep,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.CurrentStep = record.CurrentStep

	record, payload, workspace, err = c.refreshAgentConnection(ctx, workspace, record, payload)
	if err != nil {
		return OnboardingStateView{}, err
	}
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) GetAgentConnectionStatus(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepAgentInstall {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	record, payload, workspace, err = c.refreshAgentConnection(ctx, workspace, record, payload)
	if err != nil {
		return OnboardingStateView{}, err
	}
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) ConfigureDatabase(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID, host string,
	port int32,
	databaseName, connectionString string,
	locale model.Locale,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepDatabase {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	if record.CurrentStep >= OnboardingStepStarter {
		return OnboardingStateView{}, ErrOnboardingStepLocked
	}

	payload.DatabaseHost = strings.TrimSpace(firstNonEmpty(host, payload.DatabaseHost))
	payload.DatabaseName = strings.TrimSpace(firstNonEmpty(databaseName, payload.DatabaseName))
	if port > 0 {
		payload.DatabasePort = port
	}
	if payload.DatabasePort <= 0 {
		payload.DatabasePort = defaultDBPort
	}
	if payload.DatabaseUsername == "" {
		payload.DatabaseUsername = defaultDatabaseUsername
	}
	if payload.GeneratedPassword == "" {
		password, err := generateDatabasePassword()
		if err != nil {
			return OnboardingStateView{}, fmt.Errorf("generate database password: %w", err)
		}
		payload.GeneratedPassword = password
	}

	if payload.DatabaseName == "" {
		return OnboardingStateView{}, ErrOnboardingInvalidWorkspaceName
	}

	if strings.TrimSpace(connectionString) == "" {
		record, payload, err = c.persistState(
			ctx,
			tenantID,
			OnboardingStepDatabase,
			payload,
			record.CompletedAt,
		)
		if err != nil {
			return OnboardingStateView{}, err
		}
		workspace.CurrentStep = record.CurrentStep
		return c.buildStateView(ctx, workspace, record, payload)
	}

	tokenID, err := parseOptionalUUID(payload.AgentTokenID)
	if err != nil {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	result, err := c.sessions.ConfigureDatabase(ctx, tokenID, strings.TrimSpace(connectionString))
	if err != nil {
		return OnboardingStateView{}, err
	}

	if result.Error != "" || result.ErrorCode != AgentConfigureDatabaseErrorCodeUnspecified {
		payload.DBErrorCode = string(result.ErrorCode)
		payload.DBErrorMessageKO = databaseErrorMessage(result.ErrorCode, locale)
		record, payload, err = c.persistState(
			ctx,
			tenantID,
			OnboardingStepDatabase,
			payload,
			record.CompletedAt,
		)
		if err != nil {
			return OnboardingStateView{}, err
		}
		workspace.CurrentStep = record.CurrentStep
		return c.buildStateView(ctx, workspace, record, payload)
	}

	verifiedAt := result.CompletedAt.UTC()
	if verifiedAt.IsZero() {
		verifiedAt = c.now().UTC()
	}
	payload.DatabaseUsername = firstNonEmpty(result.DatabaseUser, payload.DatabaseUsername)
	payload.DatabaseName = firstNonEmpty(result.DatabaseName, payload.DatabaseName)
	payload.DBVerifiedAt = &verifiedAt
	payload.DBErrorCode = ""
	payload.DBErrorMessageKO = ""

	record, payload, err = c.persistState(
		ctx,
		tenantID,
		OnboardingStepSchema,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.CurrentStep = record.CurrentStep
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) RunSchemaIntrospection(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepSchema {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	if record.CurrentStep >= OnboardingStepStarter {
		return OnboardingStateView{}, ErrOnboardingStepLocked
	}

	result, err := c.schemas.Capture(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}

	payload.SchemaVersionID = result.VersionID.String()
	payload.SchemaTableCount = int32(result.TableCount)
	payload.SchemaColumnCount = int32(result.ColumnCount)
	payload.SchemaFKCount = int32(result.ForeignKeyCount)
	record, payload, err = c.persistState(
		ctx,
		tenantID,
		OnboardingStepSemantic,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.CurrentStep = record.CurrentStep
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) MarkSemanticApproved(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	semanticLayerID uuid.UUID,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepSemantic {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	if record.CurrentStep >= OnboardingStepStarter {
		return OnboardingStateView{}, ErrOnboardingStepLocked
	}

	layer, err := c.semanticLayers.GetByID(ctx, tenantID, semanticLayerID)
	if errors.Is(err, repository.ErrNotFound) {
		return OnboardingStateView{}, ErrOnboardingInvalidSemanticLayer
	}
	if err != nil {
		return OnboardingStateView{}, err
	}
	if layer.Status != model.SemanticLayerStatusApproved {
		return OnboardingStateView{}, ErrOnboardingInvalidSemanticLayer
	}

	approvedAt := c.now().UTC()
	if layer.ApprovedAt != nil {
		approvedAt = layer.ApprovedAt.UTC()
	}
	payload.SemanticLayerID = semanticLayerID.String()
	payload.SemanticApproved = &approvedAt
	record, payload, err = c.persistState(
		ctx,
		tenantID,
		OnboardingStepStarter,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.CurrentStep = record.CurrentStep
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) CompleteStarterStep(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepStarter {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}
	record, payload, err = c.persistState(
		ctx,
		tenantID,
		OnboardingStepDone,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.CurrentStep = record.CurrentStep
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) CompleteOnboarding(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (OnboardingStateView, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	if record.CurrentStep < OnboardingStepDone {
		return OnboardingStateView{}, ErrOnboardingInvalidStep
	}

	completedAt := c.now().UTC()
	if record.CompletedAt != nil {
		completedAt = record.CompletedAt.UTC()
	}
	record, payload, err = c.persistState(
		ctx,
		tenantID,
		OnboardingStepDone,
		payload,
		&completedAt,
	)
	if err != nil {
		return OnboardingStateView{}, err
	}
	workspace.OnboardingComplete = true
	workspace.CurrentStep = record.CurrentStep
	workspace.UpdatedAt = record.UpdatedAt
	return c.buildStateView(ctx, workspace, record, payload)
}

func (c *OnboardingController) CreateInvites(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	emails []string,
) (OnboardingStateView, []model.TenantInvite, error) {
	workspace, err := c.loadWorkspace(ctx, tenantID, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, nil, err
	}
	if workspace.Role != model.WorkspaceRoleOwner {
		return OnboardingStateView{}, nil, ErrOnboardingOwnerRequired
	}
	record, payload, err := c.loadRecord(ctx, tenantID)
	if err != nil {
		return OnboardingStateView{}, nil, err
	}
	if record.CurrentStep < OnboardingStepDone {
		return OnboardingStateView{}, nil, ErrOnboardingInvalidStep
	}

	normalized, err := normalizeInviteEmails(emails)
	if err != nil {
		return OnboardingStateView{}, nil, err
	}
	created, err := c.invites.CreateMany(ctx, tenantID, normalized, clerkUserID)
	if err != nil {
		return OnboardingStateView{}, nil, err
	}

	view, err := c.buildStateView(ctx, workspace, record, payload)
	if err != nil {
		return OnboardingStateView{}, nil, err
	}
	return view, created, nil
}

func (c *OnboardingController) loadWorkspace(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (model.OnboardingWorkspace, error) {
	workspaces, err := c.states.ListWorkspacesForUser(ctx, clerkUserID)
	if err != nil {
		return model.OnboardingWorkspace{}, err
	}
	for _, workspace := range workspaces {
		if workspace.TenantID == tenantID {
			return workspace, nil
		}
	}
	return model.OnboardingWorkspace{}, ErrOnboardingAccessDenied
}

func (c *OnboardingController) loadRecord(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantOnboardingState, model.OnboardingPayload, error) {
	record, err := c.states.GetByTenant(ctx, tenantID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return model.TenantOnboardingState{
			TenantID:    tenantID,
			CurrentStep: OnboardingStepWelcome,
			Payload:     []byte(`{}`),
		}, model.OnboardingPayload{}, nil
	case err != nil:
		return model.TenantOnboardingState{}, model.OnboardingPayload{}, err
	}

	payload, err := decodeOnboardingPayload(record.Payload)
	if err != nil {
		return model.TenantOnboardingState{}, model.OnboardingPayload{}, err
	}
	if record.CurrentStep == 0 {
		record.CurrentStep = OnboardingStepWelcome
	}
	return record, payload, nil
}

func (c *OnboardingController) refreshAgentConnection(
	ctx context.Context,
	workspace model.OnboardingWorkspace,
	record model.TenantOnboardingState,
	payload model.OnboardingPayload,
) (model.TenantOnboardingState, model.OnboardingPayload, model.OnboardingWorkspace, error) {
	snapshot, ok := c.latestOnlineAgentSession(record.TenantID, payload.AgentTokenID)
	changed := false
	currentStep := record.CurrentStep

	if ok {
		connectedAt := snapshot.ConnectedAt.UTC()
		if payload.AgentTokenID != snapshot.TokenID.String() {
			payload.AgentTokenID = snapshot.TokenID.String()
			changed = true
		}
		if payload.AgentSessionID != snapshot.SessionID {
			payload.AgentSessionID = snapshot.SessionID
			changed = true
		}
		if payload.AgentConnectedAt == nil || !payload.AgentConnectedAt.Equal(connectedAt) {
			payload.AgentConnectedAt = &connectedAt
			changed = true
		}
		if payload.AgentTokenPlain != "" {
			payload.AgentTokenPlain = ""
			changed = true
		}
		if currentStep < OnboardingStepDatabase {
			currentStep = OnboardingStepDatabase
			changed = true
		}
	} else {
		if payload.AgentSessionID != "" {
			payload.AgentSessionID = ""
			changed = true
		}
		if payload.AgentConnectedAt != nil {
			payload.AgentConnectedAt = nil
			changed = true
		}
	}
	if !changed {
		return record, payload, workspace, nil
	}

	var err error
	record, payload, err = c.persistState(
		ctx,
		record.TenantID,
		currentStep,
		payload,
		record.CompletedAt,
	)
	if err != nil {
		return model.TenantOnboardingState{}, model.OnboardingPayload{}, model.OnboardingWorkspace{}, err
	}
	workspace.CurrentStep = record.CurrentStep
	workspace.UpdatedAt = record.UpdatedAt
	return record, payload, workspace, nil
}

func (c *OnboardingController) latestOnlineAgentSession(
	tenantID uuid.UUID,
	rawTokenID string,
) (AgentSessionSnapshot, bool) {
	tokenID, err := parseOptionalUUID(rawTokenID)
	if err == nil {
		snapshot, ok := c.sessions.LatestSessionForToken(tokenID)
		if ok && snapshot.Status == "online" {
			return snapshot, true
		}
	}

	snapshot, ok := c.sessions.LatestSessionForTenant(tenantID)
	if !ok || snapshot.Status != "online" {
		return AgentSessionSnapshot{}, false
	}
	return snapshot, true
}

func (c *OnboardingController) buildStateView(
	ctx context.Context,
	workspace model.OnboardingWorkspace,
	record model.TenantOnboardingState,
	payload model.OnboardingPayload,
) (OnboardingStateView, error) {
	invites, err := c.invites.ListByTenant(ctx, workspace.TenantID)
	if err != nil {
		return OnboardingStateView{}, err
	}
	dockerRunCommand := ""
	if payload.InstallSlug != "" && payload.AgentTokenPlain != "" && c.publicControlPlaneURL != "" {
		dockerRunCommand = buildDockerRunCommand(
			payload.InstallSlug,
			c.publicControlPlaneURL,
			payload.AgentTokenPlain,
			c.edgeAgentVersion,
			c.edgeAgentImage,
		)
	}

	agentConnected := payload.AgentSessionID != "" && payload.AgentConnectedAt != nil
	agentTimedOut := false
	if !agentConnected && payload.AgentWaitStarted != nil {
		agentTimedOut = c.now().UTC().Sub(payload.AgentWaitStarted.UTC()) >= 5*time.Minute
	}

	return OnboardingStateView{
		Workspace:               workspace,
		State:                   record,
		Payload:                 payload,
		DockerRunCommand:        dockerRunCommand,
		AgentConnected:          agentConnected,
		AgentConnectionTimedOut: agentTimedOut,
		DBSetupSQL:              buildDatabaseSetupSQL(payload.DatabaseName, payload.GeneratedPassword),
		Invites:                 invites,
		CanEdit:                 workspace.Role == model.WorkspaceRoleOwner,
		WaitingForOwner:         workspace.Role == model.WorkspaceRoleMember && !workspace.OnboardingComplete,
	}, nil
}

func buildDockerRunCommand(
	installSlug, controlPlaneURL, tenantToken, agentVersion, edgeAgentImage string,
) string {
	return fmt.Sprintf(
		"docker run -d --name %s-agent \\\n"+
			"  --restart unless-stopped \\\n"+
			"  -e CONTROL_PLANE_URL=%s \\\n"+
			"  -e TENANT_TOKEN=%s \\\n"+
			"  -e AGENT_VERSION=%s \\\n"+
			"  -v /etc/%s-agent:/etc/agent \\\n"+
			"  -v /var/lib/%s-agent:/var/lib/agent \\\n"+
			"  %s",
		installSlug,
		controlPlaneURL,
		tenantToken,
		agentVersion,
		installSlug,
		installSlug,
		edgeAgentImage,
	)
}

func (c *OnboardingController) persistState(
	ctx context.Context,
	tenantID uuid.UUID,
	currentStep int32,
	payload model.OnboardingPayload,
	completedAt *time.Time,
) (model.TenantOnboardingState, model.OnboardingPayload, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return model.TenantOnboardingState{}, model.OnboardingPayload{}, fmt.Errorf("marshal onboarding payload: %w", err)
	}
	record, err := c.states.Upsert(ctx, tenantID, currentStep, body, completedAt)
	if err != nil {
		return model.TenantOnboardingState{}, model.OnboardingPayload{}, err
	}
	decoded, err := decodeOnboardingPayload(record.Payload)
	if err != nil {
		return model.TenantOnboardingState{}, model.OnboardingPayload{}, err
	}
	return record, decoded, nil
}

func decodeOnboardingPayload(body []byte) (model.OnboardingPayload, error) {
	if len(body) == 0 {
		return model.OnboardingPayload{}, nil
	}
	var payload model.OnboardingPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return model.OnboardingPayload{}, fmt.Errorf("decode onboarding payload: %w", err)
	}
	return payload, nil
}

func slugifyWorkspaceName(name, fallback string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var out strings.Builder
	lastDash := false
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case !lastDash:
			out.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(out.String(), "-")
	if slug == "" {
		slug = strings.TrimSpace(strings.ToLower(fallback))
	}
	if slug == "" {
		slug = "workspace"
	}
	return slug
}

func generateDatabasePassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "="), nil
}

func parseOptionalUUID(raw string) (uuid.UUID, error) {
	if strings.TrimSpace(raw) == "" {
		return uuid.UUID{}, errors.New("empty uuid")
	}
	return uuid.Parse(raw)
}

func buildDatabaseSetupSQL(databaseName, password string) string {
	databaseName = strings.TrimSpace(databaseName)
	password = strings.TrimSpace(password)
	if databaseName == "" || password == "" {
		return ""
	}
	return fmt.Sprintf(
		"CREATE USER '%s'@'%%' IDENTIFIED BY '%s';\nGRANT SELECT, SHOW VIEW ON `%s`.* TO '%s'@'%%';\nFLUSH PRIVILEGES;",
		defaultDatabaseUsername,
		password,
		escapeMySQLIdentifier(databaseName),
		defaultDatabaseUsername,
	)
}

func escapeMySQLIdentifier(identifier string) string {
	return strings.ReplaceAll(strings.TrimSpace(identifier), "`", "``")
}

func databaseErrorMessageKO(code AgentConfigureDatabaseErrorCode) string {
	switch code {
	case AgentConfigureDatabaseErrorCodeInvalidDSN:
		return "연결 문자열 형식이 올바르지 않습니다. 복사한 값을 다시 확인해 주세요."
	case AgentConfigureDatabaseErrorCodeConnectFailed:
		return "MySQL 서버에 연결하지 못했습니다. 호스트, 포트, 방화벽, 네트워크 접근을 확인해 주세요."
	case AgentConfigureDatabaseErrorCodeAuthFailed:
		return "MySQL 인증에 실패했습니다. 생성한 사용자와 비밀번호를 다시 확인해 주세요."
	case AgentConfigureDatabaseErrorCodePrivilegeError:
		return "읽기 전용 권한 확인에 실패했습니다. SELECT 권한만 부여되었는지 확인해 주세요."
	case AgentConfigureDatabaseErrorCodeWriteConfig:
		return "에이전트 로컬 설정 파일을 저장하지 못했습니다. Docker 볼륨 경로와 쓰기 권한을 확인해 주세요."
	case AgentConfigureDatabaseErrorCodeTimeout:
		return "데이터베이스 확인 시간이 초과되었습니다. 네트워크 상태와 서버 응답 속도를 확인해 주세요."
	default:
		return "데이터베이스 연결 확인에 실패했습니다. 입력값과 MySQL 권한을 다시 확인해 주세요."
	}
}

func normalizeInviteEmails(values []string) ([]string, error) {
	fields := make([]string, 0)
	for _, value := range values {
		for _, item := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';'
		}) {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			if !emailRE.MatchString(trimmed) {
				return nil, ErrOnboardingInviteEmailInvalid
			}
			fields = append(fields, trimmed)
		}
	}
	return fields, nil
}

func maxStep(left, right int32) int32 {
	if left > right {
		return left
	}
	return right
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
