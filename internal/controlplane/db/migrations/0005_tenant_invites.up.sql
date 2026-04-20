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
