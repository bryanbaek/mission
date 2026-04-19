package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

// Agent represents an authenticated edge-agent session scoped to a tenant
// token.
type Agent struct {
	TokenID  uuid.UUID
	TenantID uuid.UUID
	Label    string
}

type AgentTokenStore interface {
	LookupActiveByHash(ctx context.Context, hash []byte) (model.TenantToken, error)
	TouchLastUsed(ctx context.Context, tokenID uuid.UUID) error
}

// WithAgent stores a verified agent identity in context.
func WithAgent(ctx context.Context, agent Agent) context.Context {
	return context.WithValue(ctx, agentKey, agent)
}

// AgentFromContext returns the authenticated agent, or false if missing.
func AgentFromContext(ctx context.Context) (Agent, bool) {
	agent, ok := ctx.Value(agentKey).(Agent)
	return agent, ok
}

// RequireAgentToken verifies an edge-agent bearer token against the
// tenant_tokens table and injects the verified tenant-scoped agent identity
// into request context.
func RequireAgentToken(store AgentTokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerTokenFromHeader(r.Header.Get("Authorization"))
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			sum := sha256.Sum256([]byte(token))
			rec, err := store.LookupActiveByHash(r.Context(), sum[:])
			switch {
			case errors.Is(err, repository.ErrNotFound):
				writeJSONError(w, http.StatusUnauthorized, "invalid token")
				return
			case err != nil:
				writeJSONError(w, http.StatusInternalServerError, "token lookup failed")
				return
			}

			if err := store.TouchLastUsed(r.Context(), rec.ID); err != nil {
				switch {
				case errors.Is(err, repository.ErrNotFound):
					writeJSONError(w, http.StatusUnauthorized, "invalid token")
				default:
					writeJSONError(w, http.StatusInternalServerError, "token touch failed")
				}
				return
			}

			ctx := WithAgent(r.Context(), Agent{
				TokenID:  rec.ID,
				TenantID: rec.TenantID,
				Label:    rec.Label,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
