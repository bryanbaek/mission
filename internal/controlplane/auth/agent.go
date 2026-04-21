package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
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
				slog.Warn("agent auth: missing bearer token", "path", r.URL.Path)
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			sum := sha256.Sum256([]byte(token))
			rec, err := store.LookupActiveByHash(r.Context(), sum[:])
			switch {
			case errors.Is(err, repository.ErrNotFound):
				slog.Warn("agent auth: token not found", "path", r.URL.Path)
				writeJSONError(w, http.StatusUnauthorized, "invalid token")
				return
			case err != nil:
				slog.Error("agent auth: token lookup failed", "path", r.URL.Path, "err", err)
				writeJSONError(w, http.StatusInternalServerError, "token lookup failed")
				return
			}

			if err := store.TouchLastUsed(r.Context(), rec.ID); err != nil {
				switch {
				case errors.Is(err, repository.ErrNotFound):
					slog.Warn("agent auth: token revoked during touch", "path", r.URL.Path)
					writeJSONError(w, http.StatusUnauthorized, "invalid token")
				default:
					slog.Error("agent auth: touch failed", "path", r.URL.Path, "err", err)
					writeJSONError(w, http.StatusInternalServerError, "token touch failed")
				}
				return
			}

			slog.Info("agent auth: ok", "path", r.URL.Path, "token_id", rec.ID, "tenant_id", rec.TenantID)
			ctx := WithAgent(r.Context(), Agent{
				TokenID:  rec.ID,
				TenantID: rec.TenantID,
				Label:    rec.Label,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
