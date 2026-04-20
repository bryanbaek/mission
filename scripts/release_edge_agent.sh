#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ $# -gt 1 ]]; then
  echo "usage: scripts/release_edge_agent.sh [env-file]" >&2
  exit 1
fi

ENV_FILE="${1:-${PREFLIGHT_ENV_FILE:-}}"
if [[ -n "${ENV_FILE}" ]]; then
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
fi

EDGE_AGENT_VERSION="${EDGE_AGENT_VERSION:-v0.1.0}"
EDGE_AGENT_IMAGE_REPOSITORY="${EDGE_AGENT_IMAGE_REPOSITORY:-registry.digitalocean.com/mission/edge-agent}"
EDGE_AGENT_IMAGE="${EDGE_AGENT_IMAGE_REPOSITORY}:${EDGE_AGENT_VERSION}"

command -v docker >/dev/null 2>&1 || {
  echo "release failed: docker is required" >&2
  exit 1
}

cd "${ROOT_DIR}"
docker build -f Dockerfile.edge-agent -t "${EDGE_AGENT_IMAGE}" .
docker push "${EDGE_AGENT_IMAGE}"

echo "edge-agent release pushed: ${EDGE_AGENT_IMAGE}"
