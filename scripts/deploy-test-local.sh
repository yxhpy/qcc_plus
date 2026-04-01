#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="docker-compose.test.yml"
IMAGE_NAME="qcc_plus-proxy:test"
PROXY_CONTAINER="qcc_plus-test-proxy-1"
MYSQL_CONTAINER="qcc_plus-test-mysql-1"
PROXY_PORT="${PROXY_PORT:-8001}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:${PROXY_PORT}/version}"

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

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

log "building frontend bundle into web/dist"
bash scripts/build-frontend.sh

log "building docker image $IMAGE_NAME"
docker build \
  --build-arg VERSION="$VERSION" \
  --build-arg GIT_COMMIT="$GIT_COMMIT" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  -t "$IMAGE_NAME" .

# docker-compose.test.yml uses fixed container_name values.
# Remove stale containers first, otherwise compose may fail with name conflicts.
log "removing stale test containers if they exist"
docker rm -f "$PROXY_CONTAINER" "$MYSQL_CONTAINER" >/dev/null 2>&1 || true

log "starting test environment from $COMPOSE_FILE"
docker compose -f "$COMPOSE_FILE" up -d

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
log "test deploy complete"
log "image=$image_id version=$VERSION commit=$GIT_COMMIT build_date=$BUILD_DATE"
log "login: http://127.0.0.1:${PROXY_PORT}/login"
