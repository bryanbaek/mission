CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE tenants (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tenant_users (
    tenant_id     UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    clerk_user_id TEXT        NOT NULL,
    role          TEXT        NOT NULL CHECK (role IN ('owner', 'member')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, clerk_user_id)
);
CREATE INDEX tenant_users_clerk_user_id_idx ON tenant_users (clerk_user_id);

CREATE TABLE tenant_tokens (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    label        TEXT        NOT NULL,
    token_hash   BYTEA       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX tenant_tokens_active_hash_idx
    ON tenant_tokens (token_hash) WHERE revoked_at IS NULL;
CREATE INDEX tenant_tokens_tenant_id_idx ON tenant_tokens (tenant_id);

CREATE TABLE tenant_schemas (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    captured_at TIMESTAMPTZ NOT NULL,
    schema_hash TEXT        NOT NULL,
    blob        JSONB       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tenant_schemas_tenant_id_captured_at_idx
    ON tenant_schemas (tenant_id, captured_at DESC, created_at DESC);

CREATE TYPE semantic_layer_status AS ENUM ('draft', 'approved', 'archived');

CREATE TABLE tenant_semantic_layers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    schema_version_id   UUID NOT NULL REFERENCES tenant_schemas(id) ON DELETE CASCADE,
    status              semantic_layer_status NOT NULL,
    content             JSONB NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at         TIMESTAMPTZ NULL,
    approved_by_user_id TEXT NULL
);

CREATE UNIQUE INDEX tenant_semantic_layers_one_draft_idx
    ON tenant_semantic_layers (tenant_id, schema_version_id)
    WHERE status = 'draft';

CREATE UNIQUE INDEX tenant_semantic_layers_one_approved_idx
    ON tenant_semantic_layers (tenant_id, schema_version_id)
    WHERE status = 'approved';

CREATE INDEX tenant_semantic_layers_tenant_created_idx
    ON tenant_semantic_layers (tenant_id, created_at DESC);

CREATE INDEX tenant_semantic_layers_schema_status_created_idx
    ON tenant_semantic_layers (schema_version_id, status, created_at DESC);

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

CREATE TABLE tenant_invites (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email              TEXT NOT NULL,
    created_by_user_id TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX tenant_invites_tenant_email_idx
    ON tenant_invites (tenant_id, lower(email));

CREATE INDEX tenant_invites_tenant_created_idx
    ON tenant_invites (tenant_id, created_at DESC);
