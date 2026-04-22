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
require_env ANTHROPIC_API_KEY
require_env OPENAI_API_KEY

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

echo "preflight passed"
echo "  service: ${SERVICE_URL}"
echo "  edge-agent: ${EDGE_AGENT_IMAGE_REPOSITORY}:${EDGE_AGENT_VERSION}"
echo "  migrations: at head (version ${HEAD_VERSION})"
