package controller

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeOnboardingStateStore struct {
	baseTime    time.Time
	records     map[uuid.UUID]model.TenantOnboardingState
	workspaces  map[string][]model.OnboardingWorkspace
	upsertCalls int
}

func newFakeOnboardingStateStore(baseTime time.Time) *fakeOnboardingStateStore {
	return &fakeOnboardingStateStore{
		baseTime:   baseTime.UTC(),
		records:    make(map[uuid.UUID]model.TenantOnboardingState),
		workspaces: make(map[string][]model.OnboardingWorkspace),
	}
}

func (s *fakeOnboardingStateStore) GetByTenant(
	_ context.Context,
	tenantID uuid.UUID,
) (model.TenantOnboardingState, error) {
	record, ok := s.records[tenantID]
	if !ok {
		return model.TenantOnboardingState{}, repository.ErrNotFound
	}
	return cloneOnboardingStateRecord(record), nil
}

func (s *fakeOnboardingStateStore) Upsert(
	_ context.Context,
	tenantID uuid.UUID,
	currentStep int32,
	payload []byte,
	completedAt *time.Time,
) (model.TenantOnboardingState, error) {
	s.upsertCalls++

	record, ok := s.records[tenantID]
	if !ok {
		record = model.TenantOnboardingState{
			TenantID:    tenantID,
			CreatedAt:   s.baseTime,
			UpdatedAt:   s.baseTime,
			CurrentStep: OnboardingStepWelcome,
		}
	}
	record.TenantID = tenantID
	record.CurrentStep = currentStep
	record.Payload = append([]byte(nil), payload...)
	record.CompletedAt = cloneTimePtr(completedAt)
	record.UpdatedAt = s.baseTime.Add(time.Duration(s.upsertCalls) * time.Minute)
	s.records[tenantID] = record

	for userID, workspaces := range s.workspaces {
		next := append([]model.OnboardingWorkspace(nil), workspaces...)
		changed := false
		for i := range next {
			if next[i].TenantID != tenantID {
				continue
			}
			next[i].CurrentStep = currentStep
			next[i].UpdatedAt = record.UpdatedAt
			next[i].OnboardingComplete = record.CompletedAt != nil
			changed = true
		}
		if changed {
			s.workspaces[userID] = next
		}
	}

	return cloneOnboardingStateRecord(record), nil
}

func (s *fakeOnboardingStateStore) ListWorkspacesForUser(
	_ context.Context,
	clerkUserID string,
) ([]model.OnboardingWorkspace, error) {
	workspaces := s.workspaces[clerkUserID]
	return append([]model.OnboardingWorkspace(nil), workspaces...), nil
}

type fakeOnboardingInviteStore struct{}

func (*fakeOnboardingInviteStore) ListByTenant(context.Context, uuid.UUID) ([]model.TenantInvite, error) {
	return nil, nil
}

func (*fakeOnboardingInviteStore) CreateMany(context.Context, uuid.UUID, []string, string) ([]model.TenantInvite, error) {
	return nil, nil
}

type issuedAgentTokenCall struct {
	tenantID uuid.UUID
	label    string
}

type fakeOnboardingTenantController struct {
	issueCalls    []issuedAgentTokenCall
	nextToken     model.TenantToken
	nextPlaintext string
	issueErr      error
}

func (*fakeOnboardingTenantController) EnsureMembership(context.Context, uuid.UUID, string) (model.TenantUser, error) {
	return model.TenantUser{}, nil
}

func (f *fakeOnboardingTenantController) IssueAgentToken(
	_ context.Context,
	tenantID uuid.UUID,
	label string,
) (model.TenantToken, string, error) {
	f.issueCalls = append(f.issueCalls, issuedAgentTokenCall{
		tenantID: tenantID,
		label:    label,
	})
	if f.issueErr != nil {
		return model.TenantToken{}, "", f.issueErr
	}

	token := f.nextToken
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	token.TenantID = tenantID
	if token.Label == "" {
		token.Label = label
	}

	plaintext := f.nextPlaintext
	if plaintext == "" {
		plaintext = "mssn_test_plaintext"
	}
	return token, plaintext, nil
}

func (*fakeOnboardingTenantController) UpdateName(context.Context, uuid.UUID, string) (model.Tenant, error) {
	return model.Tenant{}, nil
}

type configureDatabaseCall struct {
	tokenID uuid.UUID
	dsn     string
}

type fakeOnboardingAgentSessions struct {
	tokenSnapshots    map[uuid.UUID]AgentSessionSnapshot
	tenantSnapshots   map[uuid.UUID]AgentSessionSnapshot
	latestTokenCalls  []uuid.UUID
	latestTenantCalls []uuid.UUID
	configureCalls    []configureDatabaseCall
	configureResult   AgentConfigureDatabaseResult
	configureErr      error
}

func newFakeOnboardingAgentSessions() *fakeOnboardingAgentSessions {
	return &fakeOnboardingAgentSessions{
		tokenSnapshots:  make(map[uuid.UUID]AgentSessionSnapshot),
		tenantSnapshots: make(map[uuid.UUID]AgentSessionSnapshot),
	}
}

func (f *fakeOnboardingAgentSessions) LatestSessionForTenant(
	tenantID uuid.UUID,
) (AgentSessionSnapshot, bool) {
	f.latestTenantCalls = append(f.latestTenantCalls, tenantID)
	snapshot, ok := f.tenantSnapshots[tenantID]
	return snapshot, ok
}

func (f *fakeOnboardingAgentSessions) LatestSessionForToken(
	tokenID uuid.UUID,
) (AgentSessionSnapshot, bool) {
	f.latestTokenCalls = append(f.latestTokenCalls, tokenID)
	snapshot, ok := f.tokenSnapshots[tokenID]
	return snapshot, ok
}

func (f *fakeOnboardingAgentSessions) ConfigureDatabase(
	_ context.Context,
	tokenID uuid.UUID,
	dsn string,
) (AgentConfigureDatabaseResult, error) {
	f.configureCalls = append(f.configureCalls, configureDatabaseCall{
		tokenID: tokenID,
		dsn:     dsn,
	})
	if f.configureErr != nil {
		return AgentConfigureDatabaseResult{}, f.configureErr
	}
	return f.configureResult, nil
}

type fakeOnboardingSchemaCapturer struct{}

func (*fakeOnboardingSchemaCapturer) Capture(context.Context, uuid.UUID) (SchemaCaptureResult, error) {
	return SchemaCaptureResult{}, nil
}

type fakeOnboardingSemanticStore struct{}

func (*fakeOnboardingSemanticStore) GetByID(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
	return model.TenantSemanticLayer{}, nil
}

type onboardingControllerFixture struct {
	ctrl     *OnboardingController
	states   *fakeOnboardingStateStore
	tenants  *fakeOnboardingTenantController
	sessions *fakeOnboardingAgentSessions
	now      time.Time
	tenantID uuid.UUID
	userID   string
}

func newOnboardingControllerFixture(
	t *testing.T,
	currentStep int32,
	payload model.OnboardingPayload,
) onboardingControllerFixture {
	t.Helper()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	tenantID := uuid.New()
	userID := "clerk-user-123"
	states := newFakeOnboardingStateStore(now)
	states.records[tenantID] = model.TenantOnboardingState{
		TenantID:    tenantID,
		CurrentStep: currentStep,
		Payload:     mustMarshalOnboardingPayload(t, payload),
		CreatedAt:   now.Add(-time.Hour),
		UpdatedAt:   now.Add(-time.Hour),
	}
	states.workspaces[userID] = []model.OnboardingWorkspace{
		{
			TenantID:    tenantID,
			Slug:        "ecotech",
			Name:        "Ecotech",
			Role:        model.WorkspaceRoleOwner,
			CurrentStep: currentStep,
			UpdatedAt:   now.Add(-time.Hour),
		},
	}

	tenants := &fakeOnboardingTenantController{}
	sessions := newFakeOnboardingAgentSessions()
	ctrl := NewOnboardingController(
		states,
		&fakeOnboardingInviteStore{},
		tenants,
		sessions,
		&fakeOnboardingSchemaCapturer{},
		&fakeOnboardingSemanticStore{},
		OnboardingControllerConfig{
			Now:                   func() time.Time { return now },
			EdgeAgentVersion:      "v-test",
			EdgeAgentImage:        "registry.digitalocean.com/mission/edge-agent:v-test",
			PublicControlPlaneURL: "https://mission.example.com",
		},
	)

	return onboardingControllerFixture{
		ctrl:     ctrl,
		states:   states,
		tenants:  tenants,
		sessions: sessions,
		now:      now,
		tenantID: tenantID,
		userID:   userID,
	}
}

func TestBuildDockerRunCommand(t *testing.T) {
	got := buildDockerRunCommand(
		"ecotech",
		"https://mission.example.com",
		"mssn_token",
		"v0.1.0",
		"registry.digitalocean.com/mission/edge-agent:v0.1.0",
	)

	for _, want := range []string{
		"docker run -d --name ecotech-agent",
		"--restart unless-stopped",
		"-e CONTROL_PLANE_URL=https://mission.example.com",
		"-e TENANT_TOKEN=mssn_token",
		"-e AGENT_VERSION=v0.1.0",
		"-v /etc/ecotech-agent:/etc/agent",
		"-v /var/lib/ecotech-agent:/var/lib/agent",
		"registry.digitalocean.com/mission/edge-agent:v0.1.0",
	} {
		if !containsLine(got, want) {
			t.Fatalf("docker run command %q missing %q", got, want)
		}
	}
}

func TestNewOnboardingControllerAppliesDefaults(t *testing.T) {
	ctrl := NewOnboardingController(
		newFakeOnboardingStateStore(time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)),
		&fakeOnboardingInviteStore{},
		&fakeOnboardingTenantController{},
		newFakeOnboardingAgentSessions(),
		&fakeOnboardingSchemaCapturer{},
		&fakeOnboardingSemanticStore{},
		OnboardingControllerConfig{},
	)

	if ctrl.edgeAgentImage != defaultEdgeAgentImage {
		t.Fatalf("edgeAgentImage = %q, want %q", ctrl.edgeAgentImage, defaultEdgeAgentImage)
	}
	if ctrl.edgeAgentVersion != defaultEdgeAgentVersion {
		t.Fatalf("edgeAgentVersion = %q, want %q", ctrl.edgeAgentVersion, defaultEdgeAgentVersion)
	}
}

func TestBuildDatabaseSetupSQLQuotesDatabaseAndIncludesShowView(t *testing.T) {
	got := buildDatabaseSetupSQL("mysql-1", "X1bXqFdQjYGVEBCvlPgnPRXN")

	for _, want := range []string{
		"CREATE USER 'okta_ai_ro'@'%' IDENTIFIED BY 'X1bXqFdQjYGVEBCvlPgnPRXN';",
		"GRANT SELECT, SHOW VIEW ON `mysql-1`.* TO 'okta_ai_ro'@'%';",
		"FLUSH PRIVILEGES;",
	} {
		if !containsLine(got, want) {
			t.Fatalf("database setup SQL %q missing %q", got, want)
		}
	}
}

func TestRefreshAgentConnectionFallsBackToTenantSession(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{})
	staleTokenID := uuid.New()
	liveTokenID := uuid.New()
	connectedAt := fixture.now.Add(-2 * time.Minute)

	fixture.sessions.tokenSnapshots[staleTokenID] = AgentSessionSnapshot{
		SessionID:   "stale-session",
		TenantID:    fixture.tenantID,
		TokenID:     staleTokenID,
		ConnectedAt: fixture.now.Add(-10 * time.Minute),
		Status:      "offline",
	}
	fixture.sessions.tenantSnapshots[fixture.tenantID] = AgentSessionSnapshot{
		SessionID:   "live-session",
		TenantID:    fixture.tenantID,
		TokenID:     liveTokenID,
		ConnectedAt: connectedAt,
		Status:      "online",
	}

	record, payload, workspace, err := fixture.ctrl.refreshAgentConnection(
		context.Background(),
		model.OnboardingWorkspace{TenantID: fixture.tenantID},
		model.TenantOnboardingState{
			TenantID:    fixture.tenantID,
			CurrentStep: OnboardingStepAgentInstall,
		},
		model.OnboardingPayload{
			AgentTokenID:    staleTokenID.String(),
			AgentTokenPlain: "stale-secret",
		},
	)
	if err != nil {
		t.Fatalf("refreshAgentConnection returned error: %v", err)
	}

	if record.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("currentStep = %d, want %d", record.CurrentStep, OnboardingStepDatabase)
	}
	if payload.AgentTokenID != liveTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", payload.AgentTokenID, liveTokenID.String())
	}
	if payload.AgentSessionID != "live-session" {
		t.Fatalf("AgentSessionID = %q, want %q", payload.AgentSessionID, "live-session")
	}
	if payload.AgentConnectedAt == nil || !payload.AgentConnectedAt.Equal(connectedAt.UTC()) {
		t.Fatalf("AgentConnectedAt = %v, want %v", payload.AgentConnectedAt, connectedAt.UTC())
	}
	if payload.AgentTokenPlain != "" {
		t.Fatalf("AgentTokenPlain = %q, want empty", payload.AgentTokenPlain)
	}
	if workspace.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("workspace.CurrentStep = %d, want %d", workspace.CurrentStep, OnboardingStepDatabase)
	}
	if fixture.states.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", fixture.states.upsertCalls)
	}
	if len(fixture.sessions.latestTokenCalls) != 1 {
		t.Fatalf("LatestSessionForToken calls = %d, want 1", len(fixture.sessions.latestTokenCalls))
	}
	if len(fixture.sessions.latestTenantCalls) != 1 {
		t.Fatalf("LatestSessionForTenant calls = %d, want 1", len(fixture.sessions.latestTenantCalls))
	}
}

func TestRefreshAgentConnectionPrefersStoredTokenSession(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{})
	storedTokenID := uuid.New()
	otherTokenID := uuid.New()
	storedConnectedAt := fixture.now.Add(-4 * time.Minute)

	fixture.sessions.tokenSnapshots[storedTokenID] = AgentSessionSnapshot{
		SessionID:   "stored-session",
		TenantID:    fixture.tenantID,
		TokenID:     storedTokenID,
		ConnectedAt: storedConnectedAt,
		Status:      "online",
	}
	fixture.sessions.tenantSnapshots[fixture.tenantID] = AgentSessionSnapshot{
		SessionID:   "other-session",
		TenantID:    fixture.tenantID,
		TokenID:     otherTokenID,
		ConnectedAt: fixture.now.Add(-1 * time.Minute),
		Status:      "online",
	}

	_, payload, _, err := fixture.ctrl.refreshAgentConnection(
		context.Background(),
		model.OnboardingWorkspace{TenantID: fixture.tenantID},
		model.TenantOnboardingState{
			TenantID:    fixture.tenantID,
			CurrentStep: OnboardingStepAgentInstall,
		},
		model.OnboardingPayload{
			AgentTokenID:    storedTokenID.String(),
			AgentTokenPlain: "stored-secret",
		},
	)
	if err != nil {
		t.Fatalf("refreshAgentConnection returned error: %v", err)
	}

	if payload.AgentTokenID != storedTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", payload.AgentTokenID, storedTokenID.String())
	}
	if payload.AgentSessionID != "stored-session" {
		t.Fatalf("AgentSessionID = %q, want %q", payload.AgentSessionID, "stored-session")
	}
	if payload.AgentConnectedAt == nil || !payload.AgentConnectedAt.Equal(storedConnectedAt.UTC()) {
		t.Fatalf("AgentConnectedAt = %v, want %v", payload.AgentConnectedAt, storedConnectedAt.UTC())
	}
	if len(fixture.sessions.latestTokenCalls) != 1 {
		t.Fatalf("LatestSessionForToken calls = %d, want 1", len(fixture.sessions.latestTokenCalls))
	}
	if len(fixture.sessions.latestTenantCalls) != 0 {
		t.Fatalf("LatestSessionForTenant calls = %d, want 0", len(fixture.sessions.latestTenantCalls))
	}
}

func TestRefreshAgentConnectionIgnoresOfflineTenantSession(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{})
	fixture.sessions.tenantSnapshots[fixture.tenantID] = AgentSessionSnapshot{
		SessionID:   "offline-session",
		TenantID:    fixture.tenantID,
		TokenID:     uuid.New(),
		ConnectedAt: fixture.now.Add(-3 * time.Minute),
		Status:      "offline",
	}

	record := model.TenantOnboardingState{
		TenantID:    fixture.tenantID,
		CurrentStep: OnboardingStepAgentInstall,
	}
	payload := model.OnboardingPayload{}
	workspace := model.OnboardingWorkspace{TenantID: fixture.tenantID}

	nextRecord, nextPayload, nextWorkspace, err := fixture.ctrl.refreshAgentConnection(
		context.Background(),
		workspace,
		record,
		payload,
	)
	if err != nil {
		t.Fatalf("refreshAgentConnection returned error: %v", err)
	}

	if nextRecord.CurrentStep != record.CurrentStep {
		t.Fatalf("currentStep = %d, want %d", nextRecord.CurrentStep, record.CurrentStep)
	}
	if nextPayload.AgentTokenID != "" || nextPayload.AgentSessionID != "" || nextPayload.AgentConnectedAt != nil {
		t.Fatalf("payload unexpectedly changed: %+v", nextPayload)
	}
	if nextWorkspace.CurrentStep != workspace.CurrentStep {
		t.Fatalf("workspace.CurrentStep = %d, want %d", nextWorkspace.CurrentStep, workspace.CurrentStep)
	}
	if fixture.states.upsertCalls != 0 {
		t.Fatalf("upsertCalls = %d, want 0", fixture.states.upsertCalls)
	}
}

func TestRefreshAgentConnectionClearsStaleConnectedFields(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepDatabase, model.OnboardingPayload{})
	storedTokenID := uuid.New()
	connectedAt := fixture.now.Add(-3 * time.Minute)

	fixture.sessions.tokenSnapshots[storedTokenID] = AgentSessionSnapshot{
		SessionID:   "old-session",
		TenantID:    fixture.tenantID,
		TokenID:     storedTokenID,
		ConnectedAt: connectedAt,
		Status:      "offline",
	}

	record := model.TenantOnboardingState{
		TenantID:    fixture.tenantID,
		CurrentStep: OnboardingStepDatabase,
	}
	payload := model.OnboardingPayload{
		AgentTokenID:     storedTokenID.String(),
		AgentWaitStarted: cloneTimePtr(&fixture.now),
		AgentSessionID:   "old-session",
		AgentConnectedAt: cloneTimePtr(&connectedAt),
	}
	workspace := model.OnboardingWorkspace{
		TenantID:    fixture.tenantID,
		CurrentStep: OnboardingStepDatabase,
		UpdatedAt:   fixture.now.Add(-time.Hour),
	}

	nextRecord, nextPayload, nextWorkspace, err := fixture.ctrl.refreshAgentConnection(
		context.Background(),
		workspace,
		record,
		payload,
	)
	if err != nil {
		t.Fatalf("refreshAgentConnection returned error: %v", err)
	}

	if nextRecord.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("currentStep = %d, want %d", nextRecord.CurrentStep, OnboardingStepDatabase)
	}
	if nextPayload.AgentTokenID != storedTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", nextPayload.AgentTokenID, storedTokenID.String())
	}
	if nextPayload.AgentWaitStarted == nil || !nextPayload.AgentWaitStarted.Equal(fixture.now.UTC()) {
		t.Fatalf("AgentWaitStarted = %v, want %v", nextPayload.AgentWaitStarted, fixture.now.UTC())
	}
	if nextPayload.AgentSessionID != "" {
		t.Fatalf("AgentSessionID = %q, want empty", nextPayload.AgentSessionID)
	}
	if nextPayload.AgentConnectedAt != nil {
		t.Fatalf("AgentConnectedAt = %v, want nil", nextPayload.AgentConnectedAt)
	}
	if nextWorkspace.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("workspace.CurrentStep = %d, want %d", nextWorkspace.CurrentStep, OnboardingStepDatabase)
	}
	if !nextWorkspace.UpdatedAt.After(workspace.UpdatedAt) {
		t.Fatalf("workspace.UpdatedAt = %v, want after %v", nextWorkspace.UpdatedAt, workspace.UpdatedAt)
	}
	if fixture.states.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", fixture.states.upsertCalls)
	}
	if len(fixture.sessions.latestTokenCalls) != 1 {
		t.Fatalf("LatestSessionForToken calls = %d, want 1", len(fixture.sessions.latestTokenCalls))
	}
	if len(fixture.sessions.latestTenantCalls) != 1 {
		t.Fatalf("LatestSessionForTenant calls = %d, want 1", len(fixture.sessions.latestTenantCalls))
	}
}

func TestEnsureInstallBundleDoesNotReissueWhenPlaintextMissing(t *testing.T) {
	existingTokenID := uuid.New()
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{
		WorkspaceName: "Ecotech",
		InstallSlug:   "ecotech",
		AgentTokenID:  existingTokenID.String(),
	})

	view, err := fixture.ctrl.EnsureInstallBundle(context.Background(), fixture.tenantID, fixture.userID)
	if err != nil {
		t.Fatalf("EnsureInstallBundle returned error: %v", err)
	}

	if len(fixture.tenants.issueCalls) != 0 {
		t.Fatalf("IssueAgentToken calls = %d, want 0", len(fixture.tenants.issueCalls))
	}
	if view.Payload.AgentTokenID != existingTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", view.Payload.AgentTokenID, existingTokenID.String())
	}
	if view.Payload.AgentTokenPlain != "" {
		t.Fatalf("AgentTokenPlain = %q, want empty", view.Payload.AgentTokenPlain)
	}
	if view.Payload.AgentWaitStarted == nil || !view.Payload.AgentWaitStarted.Equal(fixture.now.UTC()) {
		t.Fatalf("AgentWaitStarted = %v, want %v", view.Payload.AgentWaitStarted, fixture.now.UTC())
	}
	if view.DockerRunCommand != "" {
		t.Fatalf("DockerRunCommand = %q, want empty", view.DockerRunCommand)
	}
	if view.State.CurrentStep != OnboardingStepAgentInstall {
		t.Fatalf("currentStep = %d, want %d", view.State.CurrentStep, OnboardingStepAgentInstall)
	}
}

func TestEnsureInstallBundleDoesNotIssueWhenTenantAlreadyConnected(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{})
	liveTokenID := uuid.New()
	connectedAt := fixture.now.Add(-30 * time.Second)
	fixture.sessions.tenantSnapshots[fixture.tenantID] = AgentSessionSnapshot{
		SessionID:   "live-session",
		TenantID:    fixture.tenantID,
		TokenID:     liveTokenID,
		ConnectedAt: connectedAt,
		Status:      "online",
	}

	view, err := fixture.ctrl.EnsureInstallBundle(context.Background(), fixture.tenantID, fixture.userID)
	if err != nil {
		t.Fatalf("EnsureInstallBundle returned error: %v", err)
	}

	if len(fixture.tenants.issueCalls) != 0 {
		t.Fatalf("IssueAgentToken calls = %d, want 0", len(fixture.tenants.issueCalls))
	}
	if !view.AgentConnected {
		t.Fatal("AgentConnected = false, want true")
	}
	if view.State.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("currentStep = %d, want %d", view.State.CurrentStep, OnboardingStepDatabase)
	}
	if view.Payload.AgentTokenID != liveTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", view.Payload.AgentTokenID, liveTokenID.String())
	}
	if view.Payload.AgentTokenPlain != "" {
		t.Fatalf("AgentTokenPlain = %q, want empty", view.Payload.AgentTokenPlain)
	}
	if view.Payload.AgentWaitStarted == nil || !view.Payload.AgentWaitStarted.Equal(fixture.now.UTC()) {
		t.Fatalf("AgentWaitStarted = %v, want %v", view.Payload.AgentWaitStarted, fixture.now.UTC())
	}
	if view.Payload.InstallSlug != "ecotech" {
		t.Fatalf("InstallSlug = %q, want %q", view.Payload.InstallSlug, "ecotech")
	}
	if view.DockerRunCommand != "" {
		t.Fatalf("DockerRunCommand = %q, want empty", view.DockerRunCommand)
	}
}

func TestEnsureInstallBundleIssuesTokenWhenNoStoredTokenAndNoLiveSession(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{})
	issuedTokenID := uuid.New()
	fixture.tenants.nextToken = model.TenantToken{ID: issuedTokenID}
	fixture.tenants.nextPlaintext = "mssn_bootstrap"

	view, err := fixture.ctrl.EnsureInstallBundle(context.Background(), fixture.tenantID, fixture.userID)
	if err != nil {
		t.Fatalf("EnsureInstallBundle returned error: %v", err)
	}

	if len(fixture.tenants.issueCalls) != 1 {
		t.Fatalf("IssueAgentToken calls = %d, want 1", len(fixture.tenants.issueCalls))
	}
	if fixture.tenants.issueCalls[0].label != defaultOnboardingTokenLabel {
		t.Fatalf("IssueAgentToken label = %q, want %q", fixture.tenants.issueCalls[0].label, defaultOnboardingTokenLabel)
	}
	if view.Payload.AgentTokenID != issuedTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", view.Payload.AgentTokenID, issuedTokenID.String())
	}
	if view.Payload.AgentTokenPlain != "mssn_bootstrap" {
		t.Fatalf("AgentTokenPlain = %q, want %q", view.Payload.AgentTokenPlain, "mssn_bootstrap")
	}
	if !strings.Contains(view.DockerRunCommand, "mssn_bootstrap") {
		t.Fatalf("DockerRunCommand = %q, want token plaintext", view.DockerRunCommand)
	}
	if view.State.CurrentStep != OnboardingStepAgentInstall {
		t.Fatalf("currentStep = %d, want %d", view.State.CurrentStep, OnboardingStepAgentInstall)
	}
	if view.AgentConnected {
		t.Fatal("AgentConnected = true, want false")
	}
}

func TestConfigureDatabaseUsesReboundTokenAfterTenantFallback(t *testing.T) {
	staleTokenID := uuid.New()
	liveTokenID := uuid.New()
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{
		AgentTokenID:    staleTokenID.String(),
		AgentTokenPlain: "stale-secret",
	})
	fixture.sessions.tenantSnapshots[fixture.tenantID] = AgentSessionSnapshot{
		SessionID:   "live-session",
		TenantID:    fixture.tenantID,
		TokenID:     liveTokenID,
		ConnectedAt: fixture.now.Add(-1 * time.Minute),
		Status:      "online",
	}
	fixture.sessions.configureResult = AgentConfigureDatabaseResult{
		CompletedAt:  fixture.now.Add(2 * time.Minute),
		DatabaseUser: "okta_ai_ro",
		DatabaseName: "analytics",
	}

	if _, err := fixture.ctrl.EnsureInstallBundle(context.Background(), fixture.tenantID, fixture.userID); err != nil {
		t.Fatalf("EnsureInstallBundle returned error: %v", err)
	}

	view, err := fixture.ctrl.ConfigureDatabase(
		context.Background(),
		fixture.tenantID,
		fixture.userID,
		"db.internal",
		3306,
		"analytics",
		"mysql://readonly@db.internal/analytics",
		model.LocaleKorean,
	)
	if err != nil {
		t.Fatalf("ConfigureDatabase returned error: %v", err)
	}

	if len(fixture.sessions.configureCalls) != 1 {
		t.Fatalf("ConfigureDatabase calls = %d, want 1", len(fixture.sessions.configureCalls))
	}
	if fixture.sessions.configureCalls[0].tokenID != liveTokenID {
		t.Fatalf("ConfigureDatabase tokenID = %s, want %s", fixture.sessions.configureCalls[0].tokenID, liveTokenID)
	}
	if fixture.sessions.configureCalls[0].tokenID == staleTokenID {
		t.Fatalf("ConfigureDatabase tokenID = %s, want rebound token", fixture.sessions.configureCalls[0].tokenID)
	}
	if view.State.CurrentStep != OnboardingStepSchema {
		t.Fatalf("currentStep = %d, want %d", view.State.CurrentStep, OnboardingStepSchema)
	}
	if view.Payload.AgentTokenID != liveTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", view.Payload.AgentTokenID, liveTokenID.String())
	}
	if view.Payload.DBVerifiedAt == nil || !view.Payload.DBVerifiedAt.Equal(fixture.now.Add(2*time.Minute).UTC()) {
		t.Fatalf("DBVerifiedAt = %v, want %v", view.Payload.DBVerifiedAt, fixture.now.Add(2*time.Minute).UTC())
	}
}

func TestGetAgentConnectionStatusClearsDisconnectedStateWithoutStepRegression(t *testing.T) {
	fixture := newOnboardingControllerFixture(t, OnboardingStepAgentInstall, model.OnboardingPayload{})
	liveTokenID := uuid.New()
	connectedAt := fixture.now.Add(-30 * time.Second)

	fixture.sessions.tenantSnapshots[fixture.tenantID] = AgentSessionSnapshot{
		SessionID:   "live-session",
		TenantID:    fixture.tenantID,
		TokenID:     liveTokenID,
		ConnectedAt: connectedAt,
		Status:      "online",
	}

	connectedView, err := fixture.ctrl.EnsureInstallBundle(
		context.Background(),
		fixture.tenantID,
		fixture.userID,
	)
	if err != nil {
		t.Fatalf("EnsureInstallBundle returned error: %v", err)
	}
	if !connectedView.AgentConnected {
		t.Fatal("AgentConnected = false, want true")
	}
	if connectedView.State.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("currentStep = %d, want %d", connectedView.State.CurrentStep, OnboardingStepDatabase)
	}

	delete(fixture.sessions.tokenSnapshots, liveTokenID)
	delete(fixture.sessions.tenantSnapshots, fixture.tenantID)

	view, err := fixture.ctrl.GetAgentConnectionStatus(
		context.Background(),
		fixture.tenantID,
		fixture.userID,
	)
	if err != nil {
		t.Fatalf("GetAgentConnectionStatus returned error: %v", err)
	}

	if view.AgentConnected {
		t.Fatal("AgentConnected = true, want false")
	}
	if view.State.CurrentStep != OnboardingStepDatabase {
		t.Fatalf("currentStep = %d, want %d", view.State.CurrentStep, OnboardingStepDatabase)
	}
	if view.Payload.AgentTokenID != liveTokenID.String() {
		t.Fatalf("AgentTokenID = %q, want %q", view.Payload.AgentTokenID, liveTokenID.String())
	}
	if view.Payload.AgentSessionID != "" {
		t.Fatalf("AgentSessionID = %q, want empty", view.Payload.AgentSessionID)
	}
	if view.Payload.AgentConnectedAt != nil {
		t.Fatalf("AgentConnectedAt = %v, want nil", view.Payload.AgentConnectedAt)
	}
}

func cloneOnboardingStateRecord(record model.TenantOnboardingState) model.TenantOnboardingState {
	record.Payload = append([]byte(nil), record.Payload...)
	record.CompletedAt = cloneTimePtr(record.CompletedAt)
	return record
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func mustMarshalOnboardingPayload(t *testing.T, payload model.OnboardingPayload) []byte {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return body
}

func containsLine(body, want string) bool {
	for _, line := range strings.Split(body, "\n") {
		if line == want || strings.Contains(line, want) {
			return true
		}
	}
	return false
}
