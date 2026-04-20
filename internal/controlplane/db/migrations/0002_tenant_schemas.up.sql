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
