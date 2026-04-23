#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TIMESTAMP="${TIMESTAMP:-$(date +%F-%H%M%S)}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups/$TIMESTAMP}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-newapi}"
POSTGRES_DB="${POSTGRES_DB:-new-api}"
NGINX_CONF_PATH="${NGINX_CONF_PATH:-}"

mkdir -p "$BACKUP_DIR"

copy_if_exists() {
    local src="$1"
    local dst="$2"
    if [[ -e "$src" ]]; then
        cp -a "$src" "$dst"
    else
        echo "skip missing path: $src"
    fi
}

echo "[1/4] backing up local config"
copy_if_exists "$ROOT_DIR/.env" "$BACKUP_DIR/.env"
copy_if_exists "$ROOT_DIR/docker-compose.yml" "$BACKUP_DIR/docker-compose.yml"

if [[ -n "$NGINX_CONF_PATH" ]]; then
    copy_if_exists "$NGINX_CONF_PATH" "$BACKUP_DIR/$(basename "$NGINX_CONF_PATH")"
else
    echo "skip nginx config backup: set NGINX_CONF_PATH if needed"
fi

echo
echo "[2/4] dumping PostgreSQL database"
docker exec "$POSTGRES_CONTAINER" pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
    > "$BACKUP_DIR/postgres-${POSTGRES_DB}.sql"

echo
echo "[3/4] archiving data and logs"
if [[ -d "$ROOT_DIR/data" ]]; then
    tar -C "$ROOT_DIR" -czf "$BACKUP_DIR/data.tar.gz" data
else
    echo "skip missing path: $ROOT_DIR/data"
fi

if [[ -d "$ROOT_DIR/logs" ]]; then
    tar -C "$ROOT_DIR" -czf "$BACKUP_DIR/logs.tar.gz" logs
else
    echo "skip missing path: $ROOT_DIR/logs"
fi

echo
echo "[4/4] backup completed"
echo "$BACKUP_DIR"
