#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ $# -gt 1 ]]; then
  echo "usage: scripts/preflight.sh [env-file]" >&2
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

curl_json() {
  local url="$1"
  curl -fsS \
    -H "Authorization: Bearer ${DO_API_TOKEN}" \
    -H "Content-Type: application/json" \
    "$url"
}

require_command curl
require_command jq
require_command docker
require_command go
require_command rg

DO_API_TOKEN="${DO_API_TOKEN:-${DIGITALOCEAN_ACCESS_TOKEN:-}}"
[[ -n "${DO_API_TOKEN}" ]] || fail "set DO_API_TOKEN or DIGITALOCEAN_ACCESS_TOKEN"
require_env DO_APP_ID
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
EDGE_AGENT_IMAGE_REPOSITORY="${EDGE_AGENT_IMAGE_REPOSITORY:-registry.digitalocean.com/mission/edge-agent}"
ANTHROPIC_PREFLIGHT_MODEL="${ANTHROPIC_PREFLIGHT_MODEL:-claude-3-5-haiku-latest}"
OPENAI_PREFLIGHT_MODEL="${OPENAI_PREFLIGHT_MODEL:-gpt-4.1-nano}"
CLERK_PUBLISHABLE_KEY_EFFECTIVE="${VITE_CLERK_PUBLISHABLE_KEY:-${CLERK_PUBLISHABLE_KEY:-}}"

docker version >/dev/null 2>&1 || fail "docker is installed but not usable from this shell"

APP_JSON="$(curl_json "https://api.digitalocean.com/v2/apps/${DO_APP_ID}")" || fail "unable to fetch DigitalOcean App Platform app ${DO_APP_ID}"
APP_SPEC_JSON="$(printf '%s' "${APP_JSON}" | jq -c '.app.active_deployment.spec // .app.spec')"
[[ "${APP_SPEC_JSON}" != "null" ]] || fail "DigitalOcean app response did not include an app spec"

SOURCE_BASED_COMPONENTS="$(
  printf '%s' "${APP_SPEC_JSON}" | jq -r '
    [((.services // []) + (.workers // []) + (.jobs // []) + (.static_sites // []))[]
      | select(.github or .gitlab or .git or .bitbucket or (.image | not))
      | .name] | join(", ")
  '
)"
if [[ -n "${SOURCE_BASED_COMPONENTS}" ]]; then
  fail "production app must be image-based from DOCR; source-based or non-image components found: ${SOURCE_BASED_COMPONENTS}"
fi

LATEST_TAG_COMPONENTS="$(
  printf '%s' "${APP_SPEC_JSON}" | jq -r '
    [((.services // []) + (.workers // []) + (.jobs // []) + (.static_sites // []))[]
      | select((.image.tag // "") == "" or (.image.tag // "") == "latest")
      | .name] | join(", ")
  '
)"
if [[ -n "${LATEST_TAG_COMPONENTS}" ]]; then
  fail "App Platform components must use pinned image tags, not latest: ${LATEST_TAG_COMPONENTS}"
fi

AUTODEPLOY_COMPONENTS="$(
  printf '%s' "${APP_SPEC_JSON}" | jq -r '
    [((.services // []) + (.workers // []) + (.jobs // []) + (.static_sites // []))[]
      | select(.image.deploy_on_push.enabled == true)
      | .name] | join(", ")
  '
)"
if [[ -n "${AUTODEPLOY_COMPONENTS}" ]]; then
  fail "App Platform image autodeploy must be disabled in production: ${AUTODEPLOY_COMPONENTS}"
fi

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

REPO_WITHOUT_HOST="${EDGE_AGENT_IMAGE_REPOSITORY#registry.digitalocean.com/}"
if [[ "${REPO_WITHOUT_HOST}" == "${EDGE_AGENT_IMAGE_REPOSITORY}" || "${REPO_WITHOUT_HOST}" != */* ]]; then
  fail "EDGE_AGENT_IMAGE_REPOSITORY must look like registry.digitalocean.com/<registry>/<repository>"
fi
REGISTRY_NAME="${REPO_WITHOUT_HOST%%/*}"
REPOSITORY_NAME="${REPO_WITHOUT_HOST#*/}"
ENCODED_REPOSITORY_NAME="$(jq -nr --arg value "${REPOSITORY_NAME}" '$value|@uri')"
TAGS_JSON="$(curl_json "https://api.digitalocean.com/v2/registries/${REGISTRY_NAME}/repositories/${ENCODED_REPOSITORY_NAME}/tags")" || fail "unable to list DOCR tags for ${EDGE_AGENT_IMAGE_REPOSITORY}"
printf '%s' "${TAGS_JSON}" | jq -e --arg tag "${EDGE_AGENT_VERSION}" '.tags[]? | select(.tag == $tag)' >/dev/null \
  || fail "DOCR repository ${EDGE_AGENT_IMAGE_REPOSITORY} is missing pinned tag ${EDGE_AGENT_VERSION}"

[[ "${CLERK_SECRET_KEY}" == sk_live_* ]] || fail "CLERK_SECRET_KEY must be a live production key, not a test key"
[[ -n "${CLERK_PUBLISHABLE_KEY_EFFECTIVE}" ]] || fail "set VITE_CLERK_PUBLISHABLE_KEY or CLERK_PUBLISHABLE_KEY"
[[ "${CLERK_PUBLISHABLE_KEY_EFFECTIVE}" == pk_live_* ]] || fail "Clerk publishable key must be a live production key, not a test key"

go run ./cmd/preflight-helper anthropic-ping \
  --api-key "${ANTHROPIC_API_KEY}" \
  --model "${ANTHROPIC_PREFLIGHT_MODEL}" >/dev/null \
  || fail "Anthropic live preflight request failed for model ${ANTHROPIC_PREFLIGHT_MODEL}; verify key and account credit"

go run ./cmd/preflight-helper openai-ping \
  --api-key "${OPENAI_API_KEY}" \
  --model "${OPENAI_PREFLIGHT_MODEL}" >/dev/null \
  || fail "OpenAI live preflight request failed for model ${OPENAI_PREFLIGHT_MODEL}; verify key and account credit"

cd "${ROOT_DIR}"
rg -q 'sentry\.Init' cmd/control-plane/main.go || fail "backend Sentry init hook is missing"
rg -q 'Sentry\.init' web/src/main.tsx || fail "frontend Sentry init hook is missing"

echo "preflight ok: production app spec is pinned, migrations are at head, DOCR tag ${EDGE_AGENT_VERSION} exists, provider live checks passed, Clerk keys are live, and Sentry is wired"
