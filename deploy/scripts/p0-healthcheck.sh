#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/.env}"
APP_BIND_IP="${APP_BIND_IP:-127.0.0.1}"
APP_PORT="${APP_PORT:-}"

if [[ -z "$APP_PORT" && -f "$ENV_FILE" ]]; then
    APP_PORT="$(awk -F= '/^APP_PORT=/{print $2; exit}' "$ENV_FILE")"
fi

APP_PORT="${APP_PORT:-3000}"
STATUS_URL="${STATUS_URL:-http://${APP_BIND_IP}:${APP_PORT}/api/status}"

echo "[1/4] docker compose ps"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps

echo
echo "[2/4] health endpoint"
curl --fail --silent --show-error "$STATUS_URL"
echo

echo
echo "[3/4] recent new-api logs"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" logs --tail=100 new-api

echo
echo "[4/4] checklist reminder"
cat <<'EOF'
- Browser: open the site and confirm the login page loads.
- Admin: sign in and verify settings can be saved.
- API: run one non-stream request and one stream request.
- Register: verify Turnstile and public registration work as expected.
EOF
