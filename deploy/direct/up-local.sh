#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

docker compose \
  -p new-api-direct-fixed \
  --env-file "${SCRIPT_DIR}/.env" \
  -f "${SCRIPT_DIR}/docker-compose.yml" \
  -f "${SCRIPT_DIR}/docker-compose.local-build.yml" \
  up -d --build "$@"
