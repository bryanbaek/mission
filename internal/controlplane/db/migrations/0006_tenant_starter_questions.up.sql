CREATE TABLE tenant_starter_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    semantic_layer_id UUID NOT NULL REFERENCES tenant_semantic_layers(id) ON DELETE CASCADE,
    set_id UUID NOT NULL,
    ordinal INT NOT NULL,
    text TEXT NOT NULL,
    category TEXT NOT NULL,
    primary_table TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_starter_questions_tenant_active
    ON tenant_starter_questions (tenant_id, is_active, created_at DESC);

CREATE UNIQUE INDEX idx_starter_questions_set_ordinal
    ON tenant_starter_questions (set_id, ordinal);
