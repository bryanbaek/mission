package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRoleConstants(t *testing.T) {
	t.Parallel()

	if RoleOwner != "owner" {
		t.Fatalf("RoleOwner = %q, want owner", RoleOwner)
	}
	if RoleMember != "member" {
		t.Fatalf("RoleMember = %q, want member", RoleMember)
	}
}

func TestTenantModelsStoreFields(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	createdAt := time.Unix(1700000000, 0)
	lastUsedAt := createdAt.Add(time.Hour)
	revokedAt := createdAt.Add(2 * time.Hour)

	tenant := Tenant{
		ID:        tenantID,
		Slug:      "acme",
		Name:      "Acme",
		CreatedAt: createdAt,
	}
	if tenant.ID != tenantID || tenant.Slug != "acme" || tenant.Name != "Acme" || !tenant.CreatedAt.Equal(createdAt) {
		t.Fatalf("tenant = %+v, unexpected fields", tenant)
	}

	tenantUser := TenantUser{
		TenantID:    tenantID,
		ClerkUserID: "user_123",
		Role:        RoleOwner,
		CreatedAt:   createdAt,
	}
	if tenantUser.TenantID != tenantID || tenantUser.ClerkUserID != "user_123" || tenantUser.Role != RoleOwner || !tenantUser.CreatedAt.Equal(createdAt) {
		t.Fatalf("tenant user = %+v, unexpected fields", tenantUser)
	}

	token := TenantToken{
		ID:         uuid.New(),
		TenantID:   tenantID,
		Label:      "edge agent",
		CreatedAt:  createdAt,
		LastUsedAt: &lastUsedAt,
		RevokedAt:  &revokedAt,
	}
	if token.TenantID != tenantID || token.Label != "edge agent" || token.LastUsedAt == nil || token.RevokedAt == nil {
		t.Fatalf("token = %+v, unexpected fields", token)
	}
	if !token.LastUsedAt.Equal(lastUsedAt) || !token.RevokedAt.Equal(revokedAt) {
		t.Fatalf("token timestamps = %+v, unexpected values", token)
	}
}
