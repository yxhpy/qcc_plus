#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="docker-compose.prod.yml"
COMPOSE_PROJECT="qcc_prod"
COMPOSE_PROXY_SERVICE="proxy"
PROXY_CONTAINER="qcc_prod_proxy"
MYSQL_CONTAINER="qcc_prod_mysql"
PROXY_PORT="${PROXY_PORT:-8000}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:${PROXY_PORT}/version}"
REQUIRE_EXISTING_PROD_DB="${REQUIRE_EXISTING_PROD_DB:-1}"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "[error] compose file $COMPOSE_FILE not found" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "[error] docker is not installed" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "[error] docker compose is not available" >&2
  exit 1
fi

COMPOSE_RENDERED="$(docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" config)"
PROD_DB_VOLUME_KEY="$(docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" config --volumes | head -n 1 | tr -d '\r')"
PROD_DB_VOLUME="$(printf '%s\n' "$COMPOSE_RENDERED" | awk '
  /^volumes:$/ {in_volumes=1; next}
  in_volumes && /^    name:/ {print $2; exit}
  in_volumes && /^[^[:space:]]/ {exit}
')"
PROD_DB_DSN="$(printf '%s\n' "$COMPOSE_RENDERED" | awk '$1 == "PROXY_MYSQL_DSN:" {sub(/^PROXY_MYSQL_DSN:[[:space:]]*/, ""); print; exit}')"
PROD_DB_NAME="$(printf '%s' "$PROD_DB_DSN" | sed -E 's#.*\)/([^?]+)\?.*#\1#')"

if [[ -z "$PROD_DB_VOLUME" && -n "$PROD_DB_VOLUME_KEY" ]]; then
  PROD_DB_VOLUME="${COMPOSE_PROJECT}_${PROD_DB_VOLUME_KEY}"
fi

if [[ -z "$PROD_DB_VOLUME" ]]; then
  echo "[error] unable to resolve production mysql volume from $COMPOSE_FILE" >&2
  exit 1
fi

if [[ "$REQUIRE_EXISTING_PROD_DB" == "1" ]] && ! docker volume inspect "$PROD_DB_VOLUME" >/dev/null 2>&1; then
  echo "[error] required production mysql volume '$PROD_DB_VOLUME' does not exist" >&2
  echo "[error] refusing to initialize an empty production database" >&2
  echo "[hint] if you intentionally want a new empty production database, rerun with REQUIRE_EXISTING_PROD_DB=0" >&2
  exit 1
fi

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
ENVIRONMENT="${ENVIRONMENT:-prod}"
export VERSION GIT_COMMIT BUILD_DATE ENVIRONMENT

log "resolved production mysql volume: $PROD_DB_VOLUME"
if [[ -n "$PROD_DB_NAME" ]]; then
  log "resolved production database name: $PROD_DB_NAME"
fi

log "building frontend bundle into web/dist"
bash scripts/build-frontend.sh

log "building proxy image via docker compose"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" build "$COMPOSE_PROXY_SERVICE"

log "starting mysql service from $COMPOSE_FILE"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" up -d mysql

log "waiting for mysql health"
for ((i=1; i<=30; i++)); do
  health="$(docker inspect "$MYSQL_CONTAINER" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
  if [[ "$health" == "healthy" || "$health" == "running" ]]; then
    log "mysql status: $health"
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    echo "[error] mysql did not become healthy in time" >&2
    docker logs --tail 100 "$MYSQL_CONTAINER" || true
    exit 1
  fi
  sleep 2
done

log "starting proxy service"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" up -d --no-deps "$COMPOSE_PROXY_SERVICE"

log "waiting for proxy health at $HEALTH_URL"
for ((i=1; i<=30; i++)); do
  status="$(curl -s -o /dev/null -w '%{http_code}' "$HEALTH_URL" || true)"
  if [[ "$status" == "200" ]]; then
    log "proxy health check succeeded"
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    echo "[error] proxy did not become healthy in time (status=$status)" >&2
    docker logs --tail 150 "$PROXY_CONTAINER" || true
    exit 1
  fi
  sleep 2
done

image_id="$(docker inspect "$PROXY_CONTAINER" --format '{{.Image}}')"
log "production deploy complete"
log "image=$image_id version=$VERSION commit=$GIT_COMMIT build_date=$BUILD_DATE"
log "login: http://127.0.0.1:${PROXY_PORT}/login"
