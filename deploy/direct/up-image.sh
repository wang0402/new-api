#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-new-api-direct-fixed}"

docker compose \
  -p "${PROJECT_NAME}" \
  --env-file "${SCRIPT_DIR}/.env" \
  -f "${SCRIPT_DIR}/docker-compose.yml" \
  -f "${SCRIPT_DIR}/docker-compose.prebuilt-image.yml" \
  pull new-api

docker compose \
  -p "${PROJECT_NAME}" \
  --env-file "${SCRIPT_DIR}/.env" \
  -f "${SCRIPT_DIR}/docker-compose.yml" \
  -f "${SCRIPT_DIR}/docker-compose.prebuilt-image.yml" \
  up -d "$@"
