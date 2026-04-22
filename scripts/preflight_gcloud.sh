#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ $# -gt 1 ]]; then
  echo "usage: scripts/preflight_gcloud.sh [env-file]" >&2
  exit 1
fi

ENV_FILE="${1:-${PREFLIGHT_ENV_FILE:-}}"
if [[ -n "${ENV_FILE}" ]]; then
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
fi

fail() {
  echo "preflight failed: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_env() {
  local name="$1"
  [[ -n "${!name:-}" ]] || fail "required environment variable is missing: ${name}"
}

provider_env_prefix() {
  printf '%s' "$1" | tr '[:lower:]-' '[:upper:]_'
}

configured_providers() {
  local provider prefix api_key_var api_key
  local out=()
  for provider in anthropic openai together mistral cerebras deepseek xai fireworks; do
    prefix="$(provider_env_prefix "${provider}")"
    api_key_var="${prefix}_API_KEY"
    api_key="${!api_key_var:-}"
    if [[ -n "${api_key}" ]]; then
      out+=("${provider}")
    fi
  done
  printf '%s\n' "${out[@]}"
}

resolve_provider_model() {
  local provider="$1"
  local configured_count="$2"
  local prefix preflight_var query_var semantic_var model
  prefix="$(provider_env_prefix "${provider}")"
  preflight_var="${prefix}_PREFLIGHT_MODEL"
  query_var="${prefix}_QUERY_MODEL"
  semantic_var="${prefix}_SEMANTIC_LAYER_MODEL"
  model="${!preflight_var:-${!query_var:-${!semantic_var:-}}}"
  if [[ -z "${model}" && "${configured_count}" -eq 1 ]]; then
    model="${QUERY_MODEL:-${SEMANTIC_LAYER_MODEL:-}}"
  fi
  printf '%s' "${model}"
}

run_provider_live_checks() {
  local providers=()
  local provider prefix api_key_var api_key model
  while IFS= read -r provider; do
    [[ -n "${provider}" ]] && providers+=("${provider}")
  done < <(configured_providers)

  if [[ "${#providers[@]}" -eq 0 ]]; then
    fail "configure at least one LLM provider API key (ANTHROPIC_API_KEY, OPENAI_API_KEY, TOGETHER_API_KEY, MISTRAL_API_KEY, CEREBRAS_API_KEY, DEEPSEEK_API_KEY, XAI_API_KEY, or FIREWORKS_API_KEY)"
  fi

  for provider in "${providers[@]}"; do
    prefix="$(provider_env_prefix "${provider}")"
    api_key_var="${prefix}_API_KEY"
    api_key="${!api_key_var:-}"
    model="$(resolve_provider_model "${provider}" "${#providers[@]}")"
    if [[ -z "${model}" ]]; then
      fail "provider ${provider} is configured but no ${prefix}_PREFLIGHT_MODEL, ${prefix}_QUERY_MODEL, or ${prefix}_SEMANTIC_LAYER_MODEL is set"
    fi

    go run ./cmd/preflight-helper llm-ping \
      --provider "${provider}" \
      --api-key "${api_key}" \
      --model "${model}" >/dev/null \
      || fail "${provider} live preflight request failed for model ${model}; verify key, model, and account credit"
  done
}

require_command gcloud
require_command jq
require_command docker
require_command go

require_env DATABASE_URL
require_env CLERK_SECRET_KEY
require_env SENTRY_DSN
require_env SENTRY_ENVIRONMENT
require_env SENTRY_RELEASE
require_env VITE_SENTRY_DSN
require_env VITE_SENTRY_ENVIRONMENT
require_env VITE_SENTRY_RELEASE

PUBLIC_CONTROL_PLANE_URL="${PUBLIC_CONTROL_PLANE_URL:-}"
if [[ -z "${PUBLIC_CONTROL_PLANE_URL}" ]]; then
  fail "PUBLIC_CONTROL_PLANE_URL must be set so onboarding renders a real externally reachable docker command"
fi
if [[ "${PUBLIC_CONTROL_PLANE_URL}" =~ localhost|127\.0\.0\.1|host\.docker\.internal ]]; then
  fail "PUBLIC_CONTROL_PLANE_URL must not point to localhost or a host-only development address"
fi

EDGE_AGENT_VERSION="${EDGE_AGENT_VERSION:-v0.1.0}"
EDGE_AGENT_IMAGE_REPOSITORY="${EDGE_AGENT_IMAGE_REPOSITORY:-}"
if [[ -z "${EDGE_AGENT_IMAGE_REPOSITORY}" ]]; then
  fail "EDGE_AGENT_IMAGE_REPOSITORY must be set (e.g. us-docker.pkg.dev/PROJECT/mission/edge-agent)"
fi

# Validate Artifact Registry format: REGION-docker.pkg.dev/PROJECT/REPO/IMAGE
if [[ ! "${EDGE_AGENT_IMAGE_REPOSITORY}" =~ ^[a-z0-9-]+(-docker\.pkg\.dev|\.pkg\.dev)/[^/]+/[^/]+/[^/]+$ ]]; then
  fail "EDGE_AGENT_IMAGE_REPOSITORY must be an Artifact Registry path (e.g. us-docker.pkg.dev/PROJECT/REPO/IMAGE), got: ${EDGE_AGENT_IMAGE_REPOSITORY}"
fi

docker version >/dev/null 2>&1 || fail "docker is installed but not usable from this shell"
gcloud auth print-access-token >/dev/null 2>&1 || fail "not authenticated with gcloud — run: gcloud auth login"

# Check that the pinned edge-agent tag exists in Artifact Registry.
EXISTING_TAG="$(
  gcloud artifacts docker tags list "${EDGE_AGENT_IMAGE_REPOSITORY}" \
    --filter="tag=${EDGE_AGENT_VERSION}" \
    --format="value(tag)" \
    --quiet 2>/dev/null || true
)"
if [[ -z "${EXISTING_TAG}" ]]; then
  fail "Artifact Registry image ${EDGE_AGENT_IMAGE_REPOSITORY} is missing pinned tag ${EDGE_AGENT_VERSION}"
fi

# Check database migration status using the preflight helper.
MIGRATION_STATUS_JSON="$(
  cd "${ROOT_DIR}" &&
    go run ./cmd/preflight-helper migrations-status --database-url "${DATABASE_URL}"
)"
DIRTY="$(printf '%s' "${MIGRATION_STATUS_JSON}" | jq -r '.dirty')"
AT_HEAD="$(printf '%s' "${MIGRATION_STATUS_JSON}" | jq -r '.at_head')"
CURRENT_VERSION="$(printf '%s' "${MIGRATION_STATUS_JSON}" | jq -r '.current_version')"
HEAD_VERSION="$(printf '%s' "${MIGRATION_STATUS_JSON}" | jq -r '.head_version')"
if [[ "${DIRTY}" != "false" ]]; then
  fail "database migrations are dirty at version ${CURRENT_VERSION}"
fi
if [[ "${AT_HEAD}" != "true" ]]; then
  fail "database migrations are not at head (current=${CURRENT_VERSION}, head=${HEAD_VERSION})"
fi

# Verify the Cloud Run service exists and is serving.
CLOUD_RUN_SERVICE="${CLOUD_RUN_SERVICE:-control-plane}"
CLOUD_RUN_REGION="${CLOUD_RUN_REGION:-us-central1}"
SERVICE_URL="$(
  gcloud run services describe "${CLOUD_RUN_SERVICE}" \
    --region "${CLOUD_RUN_REGION}" \
    --format="value(status.url)" \
    --quiet 2>/dev/null || true
)"
if [[ -z "${SERVICE_URL}" ]]; then
  fail "Cloud Run service '${CLOUD_RUN_SERVICE}' not found in region '${CLOUD_RUN_REGION}' — deploy it first"
fi

run_provider_live_checks

echo "preflight passed"
echo "  service: ${SERVICE_URL}"
echo "  edge-agent: ${EDGE_AGENT_IMAGE_REPOSITORY}:${EDGE_AGENT_VERSION}"
echo "  migrations: at head (version ${HEAD_VERSION})"
