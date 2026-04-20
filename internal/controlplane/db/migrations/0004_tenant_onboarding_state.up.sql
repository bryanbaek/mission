CREATE TABLE tenant_onboarding_state (
    tenant_id    UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    current_step INTEGER NOT NULL CHECK (current_step BETWEEN 1 AND 7),
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    completed_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tenant_onboarding_state_incomplete_idx
    ON tenant_onboarding_state (completed_at, updated_at DESC);
