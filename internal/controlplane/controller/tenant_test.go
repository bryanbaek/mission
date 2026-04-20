package controller

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type fakeTenantStore struct {
	createCalled int
	createFn     func(ctx context.Context, slug, name, ownerClerkID string) (model.Tenant, error)
	listCalled   int
	listFn       func(ctx context.Context, clerkUserID string) ([]model.Tenant, error)
	updateFn     func(ctx context.Context, tenantID uuid.UUID, name string) (model.Tenant, error)
}

func (f *fakeTenantStore) CreateWithOwner(ctx context.Context, slug, name, ownerClerkID string) (model.Tenant, error) {
	f.createCalled++
	if f.createFn != nil {
		return f.createFn(ctx, slug, name, ownerClerkID)
	}
	return model.Tenant{}, nil
}

func (f *fakeTenantStore) ListForUser(ctx context.Context, clerkUserID string) ([]model.Tenant, error) {
	f.listCalled++
	if f.listFn != nil {
		return f.listFn(ctx, clerkUserID)
	}
	return nil, nil
}

func (f *fakeTenantStore) GetMembership(_ context.Context, _ uuid.UUID, _ string) (model.TenantUser, error) {
	return model.TenantUser{}, nil
}

func (f *fakeTenantStore) UpdateName(
	ctx context.Context,
	tenantID uuid.UUID,
	name string,
) (model.Tenant, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, tenantID, name)
	}
	return model.Tenant{}, nil
}

type fakeTenantTokenStore struct {
	createCalled int
	createFn     func(ctx context.Context, tenantID uuid.UUID, label string, tokenHash []byte) (model.TenantToken, error)
	listCalled   int
	listFn       func(ctx context.Context, tenantID uuid.UUID) ([]model.TenantToken, error)
	revokeCalled int
	revokeFn     func(ctx context.Context, tenantID, tokenID uuid.UUID) error
}

func (f *fakeTenantTokenStore) Create(ctx context.Context, tenantID uuid.UUID, label string, tokenHash []byte) (model.TenantToken, error) {
	f.createCalled++
	if f.createFn != nil {
		return f.createFn(ctx, tenantID, label, tokenHash)
	}
	return model.TenantToken{}, nil
}

func (f *fakeTenantTokenStore) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.TenantToken, error) {
	f.listCalled++
	if f.listFn != nil {
		return f.listFn(ctx, tenantID)
	}
	return nil, nil
}

func (f *fakeTenantTokenStore) Revoke(ctx context.Context, tenantID, tokenID uuid.UUID) error {
	f.revokeCalled++
	if f.revokeFn != nil {
		return f.revokeFn(ctx, tenantID, tokenID)
	}
	return nil
}

func TestTenantControllerCreateValidation(t *testing.T) {
	t.Parallel()

	tenants := &fakeTenantStore{}
	controller := NewTenantController(tenants, &fakeTenantTokenStore{})

	cases := []struct {
		name  string
		slug  string
		value string
		err   error
	}{
		{name: "invalid slug", slug: "Nope", value: "Tenant", err: ErrInvalidSlug},
		{name: "empty name", slug: "tenant-slug", value: "", err: ErrInvalidName},
		{name: "too long name", slug: "tenant-slug", value: strings.Repeat("x", 201), err: ErrInvalidName},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := controller.Create(context.Background(), "user_123", tc.slug, tc.value)
			if !errors.Is(err, tc.err) {
				t.Fatalf("Create error = %v, want %v", err, tc.err)
			}
		})
	}

	if tenants.createCalled != 0 {
		t.Fatalf("CreateWithOwner called %d times, want 0", tenants.createCalled)
	}
}

func TestTenantControllerCreateCallsRepository(t *testing.T) {
	t.Parallel()

	want := model.Tenant{
		ID:        uuid.New(),
		Slug:      "acme",
		Name:      "Acme",
		CreatedAt: time.Unix(1700000000, 0),
	}
	tenants := &fakeTenantStore{
		createFn: func(_ context.Context, slug, name, ownerClerkID string) (model.Tenant, error) {
			if slug != "acme" || name != "Acme" || ownerClerkID != "user_123" {
				t.Fatalf("unexpected create args: slug=%q name=%q owner=%q", slug, name, ownerClerkID)
			}
			return want, nil
		},
	}

	controller := NewTenantController(tenants, &fakeTenantTokenStore{})
	got, err := controller.Create(context.Background(), "user_123", "acme", "Acme")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Create returned %+v, want %+v", got, want)
	}
}

func TestTenantControllerCreatePropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("insert failed")
	controller := NewTenantController(&fakeTenantStore{
		createFn: func(context.Context, string, string, string) (model.Tenant, error) {
			return model.Tenant{}, wantErr
		},
	}, &fakeTenantTokenStore{})

	_, err := controller.Create(context.Background(), "user_123", "acme", "Acme")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Create error = %v, want %v", err, wantErr)
	}
}

func TestTenantControllerListForUserPassThrough(t *testing.T) {
	t.Parallel()

	want := []model.Tenant{{ID: uuid.New(), Slug: "acme", Name: "Acme"}}
	controller := NewTenantController(&fakeTenantStore{
		listFn: func(_ context.Context, clerkUserID string) ([]model.Tenant, error) {
			if clerkUserID != "user_123" {
				t.Fatalf("clerkUserID = %q, want user_123", clerkUserID)
			}
			return want, nil
		},
	}, &fakeTenantTokenStore{})

	got, err := controller.ListForUser(context.Background(), "user_123")
	if err != nil {
		t.Fatalf("ListForUser returned error: %v", err)
	}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ListForUser returned %+v, want %+v", got, want)
	}
}

func TestTenantControllerIssueAgentTokenValidation(t *testing.T) {
	t.Parallel()

	tokens := &fakeTenantTokenStore{}
	controller := NewTenantController(&fakeTenantStore{}, tokens)

	_, _, err := controller.IssueAgentToken(context.Background(), uuid.New(), "")
	if !errors.Is(err, ErrInvalidLabel) {
		t.Fatalf("IssueAgentToken error = %v, want %v", err, ErrInvalidLabel)
	}
	if tokens.createCalled != 0 {
		t.Fatalf("Create called %d times, want 0", tokens.createCalled)
	}
}

func TestTenantControllerIssueAgentTokenReturnsPlaintextAndHash(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	wantRecord := model.TenantToken{
		ID:       uuid.New(),
		TenantID: tenantID,
		Label:    "edge agent",
	}

	var capturedHash []byte
	controller := NewTenantController(&fakeTenantStore{}, &fakeTenantTokenStore{
		createFn: func(_ context.Context, gotTenantID uuid.UUID, label string, tokenHash []byte) (model.TenantToken, error) {
			if gotTenantID != tenantID {
				t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
			}
			if label != "edge agent" {
				t.Fatalf("label = %q, want edge agent", label)
			}
			capturedHash = append([]byte(nil), tokenHash...)
			return wantRecord, nil
		},
	})

	record, plaintext, err := controller.IssueAgentToken(context.Background(), tenantID, "edge agent")
	if err != nil {
		t.Fatalf("IssueAgentToken returned error: %v", err)
	}
	if record != wantRecord {
		t.Fatalf("record = %+v, want %+v", record, wantRecord)
	}
	if !strings.HasPrefix(plaintext, tokenPrefix) {
		t.Fatalf("plaintext = %q, want prefix %q", plaintext, tokenPrefix)
	}
	if !bytes.Equal(capturedHash, hashToken(plaintext)) {
		t.Fatal("Create received hash that does not match plaintext token")
	}
}

func TestTenantControllerIssueAgentTokenPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("store failed")
	controller := NewTenantController(&fakeTenantStore{}, &fakeTenantTokenStore{
		createFn: func(context.Context, uuid.UUID, string, []byte) (model.TenantToken, error) {
			return model.TenantToken{}, wantErr
		},
	})

	_, _, err := controller.IssueAgentToken(context.Background(), uuid.New(), "edge agent")
	if !errors.Is(err, wantErr) {
		t.Fatalf("IssueAgentToken error = %v, want %v", err, wantErr)
	}
}

func TestTenantControllerListAndRevokeAgentTokensPassThrough(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	tokenID := uuid.New()
	wantTokens := []model.TenantToken{{ID: tokenID, TenantID: tenantID, Label: "edge"}}
	wantErr := errors.New("revoke failed")

	tokens := &fakeTenantTokenStore{
		listFn: func(_ context.Context, gotTenantID uuid.UUID) ([]model.TenantToken, error) {
			if gotTenantID != tenantID {
				t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
			}
			return wantTokens, nil
		},
		revokeFn: func(_ context.Context, gotTenantID, gotTokenID uuid.UUID) error {
			if gotTenantID != tenantID || gotTokenID != tokenID {
				t.Fatalf("unexpected revoke args: tenant=%s token=%s", gotTenantID, gotTokenID)
			}
			return wantErr
		},
	}
	controller := NewTenantController(&fakeTenantStore{}, tokens)

	gotTokens, err := controller.ListAgentTokens(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListAgentTokens returned error: %v", err)
	}
	if len(gotTokens) != len(wantTokens) || gotTokens[0] != wantTokens[0] {
		t.Fatalf("ListAgentTokens returned %+v, want %+v", gotTokens, wantTokens)
	}

	err = controller.RevokeAgentToken(context.Background(), tenantID, tokenID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RevokeAgentToken error = %v, want %v", err, wantErr)
	}
}
