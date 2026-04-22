DROP INDEX IF EXISTS tenant_query_runs_review_queue_idx;

ALTER TABLE tenant_query_runs
    DROP COLUMN IF EXISTS reviewed_by_user_id,
    DROP COLUMN IF EXISTS reviewed_at;
