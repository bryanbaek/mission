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
