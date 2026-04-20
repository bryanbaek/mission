package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeOnboardingStateStore struct{}

func (fakeOnboardingStateStore) GetByTenant(context.Context, uuid.UUID) (model.TenantOnboardingState, error) {
	return model.TenantOnboardingState{}, nil
}

func (fakeOnboardingStateStore) Upsert(
	context.Context,
	uuid.UUID,
	int32,
	[]byte,
	*time.Time,
) (model.TenantOnboardingState, error) {
	return model.TenantOnboardingState{}, nil
}

func (fakeOnboardingStateStore) ListWorkspacesForUser(context.Context, string) ([]model.OnboardingWorkspace, error) {
	return nil, nil
}

type fakeOnboardingInviteStore struct{}

func (fakeOnboardingInviteStore) ListByTenant(context.Context, uuid.UUID) ([]model.TenantInvite, error) {
	return nil, nil
}

func (fakeOnboardingInviteStore) CreateMany(context.Context, uuid.UUID, []string, string) ([]model.TenantInvite, error) {
	return nil, nil
}

type fakeOnboardingTenantController struct{}

func (fakeOnboardingTenantController) EnsureMembership(context.Context, uuid.UUID, string) (model.TenantUser, error) {
	return model.TenantUser{}, nil
}

func (fakeOnboardingTenantController) IssueAgentToken(context.Context, uuid.UUID, string) (model.TenantToken, string, error) {
	return model.TenantToken{}, "", nil
}

func (fakeOnboardingTenantController) UpdateName(context.Context, uuid.UUID, string) (model.Tenant, error) {
	return model.Tenant{}, nil
}

type fakeOnboardingAgentSessions struct{}

func (fakeOnboardingAgentSessions) LatestSessionForToken(uuid.UUID) (AgentSessionSnapshot, bool) {
	return AgentSessionSnapshot{}, false
}

func (fakeOnboardingAgentSessions) ConfigureDatabase(context.Context, uuid.UUID, string) (AgentConfigureDatabaseResult, error) {
	return AgentConfigureDatabaseResult{}, nil
}

type fakeOnboardingSchemaCapturer struct{}

func (fakeOnboardingSchemaCapturer) Capture(context.Context, uuid.UUID) (SchemaCaptureResult, error) {
	return SchemaCaptureResult{}, nil
}

type fakeOnboardingSemanticStore struct{}

func (fakeOnboardingSemanticStore) GetByID(context.Context, uuid.UUID, uuid.UUID) (model.TenantSemanticLayer, error) {
	return model.TenantSemanticLayer{}, nil
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
		fakeOnboardingStateStore{},
		fakeOnboardingInviteStore{},
		fakeOnboardingTenantController{},
		fakeOnboardingAgentSessions{},
		fakeOnboardingSchemaCapturer{},
		fakeOnboardingSemanticStore{},
		OnboardingControllerConfig{},
	)

	if ctrl.edgeAgentImage != defaultEdgeAgentImage {
		t.Fatalf("edgeAgentImage = %q, want %q", ctrl.edgeAgentImage, defaultEdgeAgentImage)
	}
	if ctrl.edgeAgentVersion != defaultEdgeAgentVersion {
		t.Fatalf("edgeAgentVersion = %q, want %q", ctrl.edgeAgentVersion, defaultEdgeAgentVersion)
	}
}

func containsLine(body, want string) bool {
	for _, line := range strings.Split(body, "\n") {
		if line == want || strings.Contains(line, want) {
			return true
		}
	}
	return false
}
