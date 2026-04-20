#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "${ROOT_DIR}/.env" ]]; then
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.env"
else
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/env.example"
fi

fail() {
  echo "isolation test failed: $*" >&2
  exit 1
}

wait_for_service() {
  local service="$1"
  local deadline=$((SECONDS + 90))
  local container_id=""

  container_id="$(docker compose -f "${ROOT_DIR}/docker-compose.yaml" ps -q "${service}")"
  [[ -n "${container_id}" ]] || fail "docker compose did not create a container for ${service}"

  while (( SECONDS < deadline )); do
    local status
    status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}")"
    if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
      return 0
    fi
    sleep 2
  done

  fail "${service} did not become ready within 90 seconds"
}

docker compose -f "${ROOT_DIR}/docker-compose.yaml" up -d postgres mysql >/dev/null
wait_for_service postgres
wait_for_service mysql

cd "${ROOT_DIR}"
go test ./internal/controlplane/handler -run '^TestTenantIsolationSmokeIntegration$' -count=1
