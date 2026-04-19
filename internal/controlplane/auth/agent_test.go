package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeAgentTokenStore struct {
	lookupFn func(ctx context.Context, hash []byte) (model.TenantToken, error)
	touchFn  func(ctx context.Context, tokenID uuid.UUID) error
}

func (f *fakeAgentTokenStore) LookupActiveByHash(
	ctx context.Context,
	hash []byte,
) (model.TenantToken, error) {
	return f.lookupFn(ctx, hash)
}

func (f *fakeAgentTokenStore) TouchLastUsed(
	ctx context.Context,
	tokenID uuid.UUID,
) error {
	return f.touchFn(ctx, tokenID)
}

func TestWithAgentAndAgentFromContext(t *testing.T) {
	t.Parallel()

	want := Agent{
		TokenID:  uuid.New(),
		TenantID: uuid.New(),
		Label:    "edge-1",
	}

	ctx := WithAgent(context.Background(), want)
	got, ok := AgentFromContext(ctx)
	if !ok {
		t.Fatal("AgentFromContext did not find agent")
	}
	if got != want {
		t.Fatalf("agent = %+v, want %+v", got, want)
	}

	_, ok = AgentFromContext(context.Background())
	if ok {
		t.Fatal("AgentFromContext unexpectedly found agent")
	}
}

func TestRequireAgentTokenMissingBearer(t *testing.T) {
	t.Parallel()

	store := &fakeAgentTokenStore{
		lookupFn: func(context.Context, []byte) (model.TenantToken, error) {
			t.Fatal("LookupActiveByHash should not run")
			return model.TenantToken{}, nil
		},
		touchFn: func(context.Context, uuid.UUID) error {
			t.Fatal("TouchLastUsed should not run")
			return nil
		},
	}

	handler := RequireAgentToken(store)(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler should not run")
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAgentTokenHandlesLookupAndTouchErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		lookupErr  error
		touchErr   error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "invalid lookup",
			lookupErr:  repository.ErrNotFound,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid token",
		},
		{
			name:       "lookup failure",
			lookupErr:  errors.New("db down"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "token lookup failed",
		},
		{
			name:       "touch missing token",
			touchErr:   repository.ErrNotFound,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid token",
		},
		{
			name:       "touch failure",
			touchErr:   errors.New("touch failed"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "token touch failed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeAgentTokenStore{
				lookupFn: func(
					context.Context,
					[]byte,
				) (model.TenantToken, error) {
					if tc.lookupErr != nil {
						return model.TenantToken{}, tc.lookupErr
					}
					return model.TenantToken{ID: uuid.New()}, nil
				},
				touchFn: func(context.Context, uuid.UUID) error {
					return tc.touchErr
				},
			}

			handler := RequireAgentToken(store)(http.HandlerFunc(
				func(http.ResponseWriter, *http.Request) {
					t.Fatal("next handler should not run")
				},
			))

			req := httptest.NewRequest(http.MethodGet, "/agents", nil)
			req.Header.Set("Authorization", "Bearer good-token")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tc.wantBody {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantBody)
			}
		})
	}
}

func TestRequireAgentTokenInjectsAgent(t *testing.T) {
	t.Parallel()

	tokenID := uuid.New()
	tenantID := uuid.New()
	sum := sha256.Sum256([]byte("good-token"))
	store := &fakeAgentTokenStore{
		lookupFn: func(
			_ context.Context,
			hash []byte,
		) (model.TenantToken, error) {
			if !bytes.Equal(hash, sum[:]) {
				t.Fatalf("hash = %x, want %x", hash, sum)
			}
			return model.TenantToken{
				ID:       tokenID,
				TenantID: tenantID,
				Label:    "edge-1",
			}, nil
		},
		touchFn: func(_ context.Context, gotTokenID uuid.UUID) error {
			if gotTokenID != tokenID {
				t.Fatalf("tokenID = %s, want %s", gotTokenID, tokenID)
			}
			return nil
		},
	}

	handler := RequireAgentToken(store)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			agent, ok := AgentFromContext(r.Context())
			if !ok {
				t.Fatal("expected agent in context")
			}
			if agent.TokenID != tokenID {
				t.Fatalf("TokenID = %s, want %s", agent.TokenID, tokenID)
			}
			if agent.TenantID != tenantID {
				t.Fatalf("TenantID = %s, want %s", agent.TenantID, tenantID)
			}
			if agent.Label != "edge-1" {
				t.Fatalf("Label = %q, want edge-1", agent.Label)
			}
			w.WriteHeader(http.StatusNoContent)
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "") {
		t.Fatal("handler unexpectedly modified content type")
	}
}
