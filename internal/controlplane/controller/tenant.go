package controller

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

// Token format: prefix + base64url-encoded 32 random bytes. Prefix makes
// leaked tokens trivially greppable in logs and source code.
const tokenPrefix = "mssn_"

var (
	ErrInvalidSlug = errors.New(
		"slug must be 3-63 chars, lowercase, alphanumeric or dash",
	)
	ErrInvalidName  = errors.New("name must be 1-200 chars")
	ErrInvalidLabel = errors.New("label must be 1-100 chars")

	slugRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])?$`)
)

type tenantStore interface {
	CreateWithOwner(
		ctx context.Context,
		slug, name, ownerClerkID string,
	) (model.Tenant, error)
	ListForUser(
		ctx context.Context,
		clerkUserID string,
	) ([]model.Tenant, error)
	GetMembership(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (model.TenantUser, error)
	UpdateName(
		ctx context.Context,
		tenantID uuid.UUID,
		name string,
	) (model.Tenant, error)
}

type tenantTokenStore interface {
	Create(
		ctx context.Context,
		tenantID uuid.UUID,
		label string,
		tokenHash []byte,
	) (model.TenantToken, error)
	ListByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
	) ([]model.TenantToken, error)
	Revoke(ctx context.Context, tenantID, tokenID uuid.UUID) error
}

type TenantController struct {
	tenants tenantStore
	tokens  tenantTokenStore
}

func NewTenantController(
	tenants tenantStore,
	tokens tenantTokenStore,
) *TenantController {
	return &TenantController{tenants: tenants, tokens: tokens}
}

func (c *TenantController) Create(
	ctx context.Context,
	ownerClerkID, slug, name string,
) (model.Tenant, error) {
	if !slugRE.MatchString(slug) {
		return model.Tenant{}, ErrInvalidSlug
	}
	if l := len(name); l < 1 || l > 200 {
		return model.Tenant{}, ErrInvalidName
	}
	return c.tenants.CreateWithOwner(ctx, slug, name, ownerClerkID)
}

func (c *TenantController) ListForUser(
	ctx context.Context,
	clerkUserID string,
) ([]model.Tenant, error) {
	return c.tenants.ListForUser(ctx, clerkUserID)
}

// EnsureMembership verifies the user belongs to the tenant. Returns the
// membership record or an error suitable for the handler to translate into
// a permission-denied response.
func (c *TenantController) EnsureMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (model.TenantUser, error) {
	return c.tenants.GetMembership(ctx, tenantID, clerkUserID)
}

// IssueAgentToken returns the persisted record AND the plaintext token. The
// plaintext is shown to the user exactly once.
func (c *TenantController) IssueAgentToken(
	ctx context.Context,
	tenantID uuid.UUID,
	label string,
) (model.TenantToken, string, error) {
	if l := len(label); l < 1 || l > 100 {
		return model.TenantToken{}, "", ErrInvalidLabel
	}
	plaintext, err := generateToken()
	if err != nil {
		return model.TenantToken{}, "", fmt.Errorf("generate token: %w", err)
	}
	hash := hashToken(plaintext)
	rec, err := c.tokens.Create(ctx, tenantID, label, hash)
	if err != nil {
		return model.TenantToken{}, "", err
	}
	return rec, plaintext, nil
}

func (c *TenantController) ListAgentTokens(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.TenantToken, error) {
	return c.tokens.ListByTenant(ctx, tenantID)
}

func (c *TenantController) RevokeAgentToken(
	ctx context.Context,
	tenantID, tokenID uuid.UUID,
) error {
	return c.tokens.Revoke(ctx, tenantID, tokenID)
}

func (c *TenantController) UpdateName(
	ctx context.Context,
	tenantID uuid.UUID,
	name string,
) (model.Tenant, error) {
	if l := len(name); l < 1 || l > 200 {
		return model.Tenant{}, ErrInvalidName
	}
	return c.tenants.UpdateName(ctx, tenantID, name)
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
