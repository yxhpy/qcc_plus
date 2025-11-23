#!/usr/bin/env bash
# Idempotent deployment helper for test/prod environments.
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

APP_ENV="${1:-}"
if [[ "$APP_ENV" != "test" && "$APP_ENV" != "prod" ]]; then
  echo "Usage: $0 [test|prod]" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

case "$APP_ENV" in
  test)
    BRANCH="test"
    COMPOSE_FILE="docker-compose.test.yml"
    PROJECT_NAME="qcc_test"
    PROXY_PORT=8001
    ;;
  prod)
    BRANCH="prod"
    COMPOSE_FILE="docker-compose.prod.yml"
    PROJECT_NAME="qcc_prod"
    PROXY_PORT=8000
    ;;
esac

IMAGE_NAME="${PROJECT_NAME}-proxy"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:${PROXY_PORT}/}"
DOCKER_CONFIG="${DOCKER_CONFIG:-$ROOT_DIR/.docker}"
mkdir -p "$DOCKER_CONFIG"

if docker compose version >/dev/null 2>&1; then
  DOCKER_COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  DOCKER_COMPOSE="docker-compose"
else
  echo "[error] docker compose is not available on this host" >&2
  exit 1
fi

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "[error] compose file $COMPOSE_FILE not found" >&2
  exit 1
fi

PREVIOUS_PROXY_IMAGE="$(docker images -q "$IMAGE_NAME" | head -n 1 || true)"

rollback() {
  local exit_code=$?
  log "deploy failed (exit ${exit_code}), attempting rollback..."
  if [[ -n "${PREVIOUS_PROXY_IMAGE:-}" ]]; then
    docker tag "$PREVIOUS_PROXY_IMAGE" "${IMAGE_NAME}:latest" >/dev/null 2>&1 || true
    $DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --no-build --remove-orphans >/dev/null 2>&1 || true
    log "rollback triggered using previous image: $PREVIOUS_PROXY_IMAGE"
  else
    log "no previous proxy image captured; rollback skipped"
  fi
}
trap rollback ERR

log "syncing branch $BRANCH"
git fetch --prune origin "$BRANCH"
git checkout "$BRANCH"
git pull --rebase origin "$BRANCH"

log "installing frontend dependencies (npm ci)"
(
  cd frontend

  # 如果 node_modules 存在但有问题，完全清理
  if [ -d "node_modules" ] && ! npm ci --no-progress 2>/dev/null; then
    log "npm ci failed, cleaning node_modules and retrying..."
    rm -rf node_modules
    npm cache clean --force 2>/dev/null || true
  fi

  # 执行干净安装
  npm ci --no-progress
)

log "building frontend bundle"
bash scripts/build-frontend.sh

log "validating docker compose config ($COMPOSE_FILE)"
$DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" config >/dev/null

log "building and deploying containers"
$DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --build --remove-orphans

wait_for_service() {
  local url="$1"
  local attempts="${2:-12}"
  local delay="${3:-5}"
  local i status
  for ((i=1; i<=attempts; i++)); do
    status="$(curl -s -o /dev/null -w "%{http_code}" "$url" || true)"
    if [[ "$status" =~ ^[23] ]]; then
      log "health check succeeded with status $status"
      return 0
    fi
    log "health check attempt $i/${attempts} failed (status=$status), retrying in ${delay}s..."
    sleep "$delay"
  done
  log "service did not become healthy after ${attempts} attempts"
  return 1
}

log "waiting for service health at ${HEALTH_URL}"
wait_for_service "$HEALTH_URL"

log "cleaning old containers/images"
docker container prune -f --filter "label=com.docker.compose.project=${PROJECT_NAME}" >/dev/null 2>&1 || true
OLD_IMAGES="$(docker images "$IMAGE_NAME" --format '{{.ID}} {{.Tag}}' | grep '<none>' | awk '{print $1}' || true)"
if [[ -n "${OLD_IMAGES}" ]]; then
  echo "$OLD_IMAGES" | xargs -r docker rmi -f >/dev/null 2>&1 || true
fi

trap - ERR
log "deploy completed for $APP_ENV (branch: $BRANCH, port: $PROXY_PORT)"
