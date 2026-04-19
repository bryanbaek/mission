package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

type Tenant struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
}

type TenantUser struct {
	TenantID    uuid.UUID
	ClerkUserID string
	Role        Role
	CreatedAt   time.Time
}

// TenantToken is the persisted record of an edge-agent token. The plaintext
// token itself is only ever returned at issuance time; we store its hash.
type TenantToken struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	Label      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}
