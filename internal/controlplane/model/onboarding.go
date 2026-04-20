package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type WorkspaceRole string

const (
	WorkspaceRoleOwner  WorkspaceRole = "owner"
	WorkspaceRoleMember WorkspaceRole = "member"
)

type TenantOnboardingState struct {
	TenantID    uuid.UUID
	CurrentStep int32
	Payload     json.RawMessage
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OnboardingPayload struct {
	PrimaryLanguage   string     `json:"primary_language,omitempty"`
	WorkspaceName     string     `json:"workspace_name,omitempty"`
	InstallSlug       string     `json:"install_slug,omitempty"`
	AgentTokenID      string     `json:"agent_token_id,omitempty"`
	AgentTokenPlain   string     `json:"agent_token_plaintext,omitempty"`
	AgentWaitStarted  *time.Time `json:"agent_wait_started_at,omitempty"`
	AgentSessionID    string     `json:"agent_session_id,omitempty"`
	AgentConnectedAt  *time.Time `json:"agent_connected_at,omitempty"`
	DatabaseHost      string     `json:"db_host,omitempty"`
	DatabasePort      int32      `json:"db_port,omitempty"`
	DatabaseName      string     `json:"db_name,omitempty"`
	DatabaseUsername  string     `json:"db_username,omitempty"`
	GeneratedPassword string     `json:"generated_password,omitempty"`
	DBVerifiedAt      *time.Time `json:"db_verified_at,omitempty"`
	DBErrorCode       string     `json:"db_error_code,omitempty"`
	DBErrorMessageKO  string     `json:"db_error_message_ko,omitempty"`
	SchemaVersionID   string     `json:"schema_version_id,omitempty"`
	SchemaTableCount  int32      `json:"schema_table_count,omitempty"`
	SchemaColumnCount int32      `json:"schema_column_count,omitempty"`
	SchemaFKCount     int32      `json:"schema_fk_count,omitempty"`
	SemanticLayerID   string     `json:"semantic_layer_id,omitempty"`
	SemanticApproved  *time.Time `json:"semantic_approved_at,omitempty"`
}

type OnboardingWorkspace struct {
	TenantID           uuid.UUID
	Slug               string
	Name               string
	Role               WorkspaceRole
	OnboardingComplete bool
	CurrentStep        int32
	UpdatedAt          time.Time
}

type TenantInvite struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Email           string
	CreatedByUserID string
	CreatedAt       time.Time
}
