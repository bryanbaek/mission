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
