DROP INDEX IF EXISTS tenant_canonical_query_examples_notes_trgm_idx;
DROP INDEX IF EXISTS tenant_canonical_query_examples_question_trgm_idx;
DROP INDEX IF EXISTS tenant_canonical_query_examples_schema_active_created_idx;
DROP INDEX IF EXISTS tenant_canonical_query_examples_active_created_idx;

DROP TABLE IF EXISTS tenant_canonical_query_examples;

DROP INDEX IF EXISTS tenant_query_feedback_user_updated_idx;
DROP TABLE IF EXISTS tenant_query_feedback;

DROP INDEX IF EXISTS tenant_query_runs_review_queue_idx;
DROP INDEX IF EXISTS tenant_query_runs_tenant_user_created_idx;
DROP INDEX IF EXISTS tenant_query_runs_tenant_created_idx;
DROP TABLE IF EXISTS tenant_query_runs;

DROP TYPE IF EXISTS query_feedback_rating;
DROP TYPE IF EXISTS query_prompt_context_source;
DROP TYPE IF EXISTS query_run_status;

DROP EXTENSION IF EXISTS pg_trgm;

DROP INDEX IF EXISTS idx_starter_questions_set_ordinal;
DROP INDEX IF EXISTS idx_starter_questions_tenant_active;
DROP TABLE IF EXISTS tenant_starter_questions;

DROP TABLE IF EXISTS tenant_invites;
DROP TABLE IF EXISTS tenant_onboarding_state;
DROP TABLE IF EXISTS tenant_semantic_layers;
DROP TYPE IF EXISTS semantic_layer_status;
DROP TABLE IF EXISTS tenant_schemas;
DROP TABLE IF EXISTS tenant_tokens;
DROP TABLE IF EXISTS tenant_users;
DROP TABLE IF EXISTS tenants;
