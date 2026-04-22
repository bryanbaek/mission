ALTER TABLE tenant_query_runs
    ADD COLUMN reviewed_at TIMESTAMPTZ NULL,
    ADD COLUMN reviewed_by_user_id TEXT NULL;

CREATE INDEX tenant_query_runs_review_queue_idx
    ON tenant_query_runs (tenant_id, reviewed_at, created_at DESC);
