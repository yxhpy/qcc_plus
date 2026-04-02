#!/usr/bin/env bash
# Idempotent deployment helper for test/prod environments.
set -Eeuo pipefail

make_deploy_id() {
  printf '%s-%06x\n' "$(date '+%Y%m%d%H%M%S')" "$(((((RANDOM << 8) ^ RANDOM)) & 0xffffff))"
}

DEPLOY_ID="${DEPLOY_ID:-$(make_deploy_id)}"
CURRENT_STEP="bootstrap"
CURRENT_HINT=""

timestamp() {
  date '+%Y-%m-%d %H:%M:%S'
}

log() {
  printf '[%s] [deploy:%s] %s\n' "$(timestamp)" "$DEPLOY_ID" "$*"
}

error() {
  printf '[%s] [deploy:%s] [error] %s\n' "$(timestamp)" "$DEPLOY_ID" "$*" >&2
}

hint() {
  printf '[%s] [deploy:%s] [hint] %s\n' "$(timestamp)" "$DEPLOY_ID" "$*" >&2
}

usage() {
  cat <<'EOF'
Usage: ./scripts/deploy-server.sh [--dry-run] <test|prod>

Options:
  --dry-run   Print resolved deployment context and exit without running git/npm/docker steps.
  -h, --help  Show this help message.

Examples:
  ./scripts/deploy-server.sh test
  ./scripts/deploy-server.sh prod
  ./scripts/deploy-server.sh --dry-run test
EOF
}

fail_usage() {
  error "$1"
  usage >&2
  exit 1
}

run_step() {
  CURRENT_STEP="$1"
  CURRENT_HINT="$2"
  shift 2
  log "$CURRENT_STEP"
  "$@"
}

require_command() {
  local command_name="$1"
  local failure_hint="$2"

  if ! command -v "$command_name" >/dev/null 2>&1; then
    error "required command not found: $command_name"
    hint "$failure_hint"
    exit 1
  fi
}

resolve_docker_compose() {
  if docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    DOCKER_COMPOSE="docker-compose"
  else
    error "docker compose is not available on this host"
    hint "请确认 Docker 已安装且 compose 插件可用，或安装 docker-compose。"
    exit 1
  fi
}

sync_branch() {
  git reset --hard HEAD
  git clean -fd
  git fetch --prune origin "$BRANCH"
  git checkout "$BRANCH"
  git pull --rebase origin "$BRANCH"
}

install_frontend_dependencies() {
  (
    cd frontend

    if [[ -d "node_modules" ]] && [[ -n "$(ls -A node_modules 2>/dev/null)" ]]; then
      log "node_modules exists, trying npm ci..."
      if npm ci --no-progress; then
        return 0
      fi

      log "npm ci failed, cleaning node_modules and npm cache..."
      cd ..
      rm -rf frontend/node_modules
      npm cache clean --force >/dev/null 2>&1 || true
      cd frontend
    fi

    log "running clean npm ci..."
    npm ci --no-progress
  )
}

wait_for_mysql() {
  local health=""

  for ((i=1; i<=30; i++)); do
    health="$(docker inspect "${PROJECT_NAME}_mysql" --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
    if [[ "$health" == "healthy" || "$health" == "running" ]]; then
      log "mysql status: $health"
      return 0
    fi

    log "waiting for mysql... (${i}/30)"
    sleep 2
  done

  error "mysql did not become healthy in time"
  hint "请检查 MySQL 容器日志、卷挂载和端口占用，例如: docker logs ${PROJECT_NAME}_mysql"
  docker logs --tail 120 "${PROJECT_NAME}_mysql" 2>/dev/null || true
  return 1
}

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

    log "health check attempt ${i}/${attempts} failed (status=${status:-000}), retrying in ${delay}s..."
    sleep "$delay"
  done

  error "service did not become healthy after ${attempts} attempts"
  hint "请检查代理容器日志、LISTEN_ADDR/端口映射和 .env 配置，例如: docker logs ${PROJECT_NAME}_proxy"
  docker logs --tail 150 "${PROJECT_NAME}_proxy" 2>/dev/null || true
  return 1
}

cleanup_images() {
  log "cleaning old containers/images"
  docker container prune -f --filter "label=com.docker.compose.project=${PROJECT_NAME}" >/dev/null 2>&1 || true

  log "pruning dangling images (build layers)"
  DANGLING_COUNT="$(docker images -f "dangling=true" -q | wc -l | tr -d ' ')"
  if [[ "$DANGLING_COUNT" -gt 0 ]]; then
    docker image prune -f >/dev/null 2>&1 || true
    log "removed ${DANGLING_COUNT} dangling images"
  fi

  OLD_IMAGES="$(docker images "$IMAGE_NAME" --format '{{.ID}} {{.Tag}}' | grep '<none>' | awk '{print $1}' || true)"
  if [[ -n "${OLD_IMAGES}" ]]; then
    echo "$OLD_IMAGES" | xargs -r docker rmi -f >/dev/null 2>&1 || true
  fi
}

validate_compose_config() {
  $DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" config >/dev/null
}

remove_fixed_name_containers() {
  docker rm -f "${PROJECT_NAME}_proxy" "${PROJECT_NAME}_mysql" >/dev/null 2>&1 || true
}

stop_existing_containers() {
  $DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down --remove-orphans
}

handle_error() {
  local exit_code="$1"

  error "deploy failed during step '${CURRENT_STEP}' (exit=${exit_code})"
  if [[ -n "$CURRENT_HINT" ]]; then
    hint "$CURRENT_HINT"
  fi

  if [[ -n "${PREVIOUS_PROXY_IMAGE:-}" && -n "${DOCKER_COMPOSE:-}" && -n "${PROJECT_NAME:-}" && -n "${COMPOSE_FILE:-}" ]]; then
    log "attempting rollback with previous image ${PREVIOUS_PROXY_IMAGE}"
    if docker tag "$PREVIOUS_PROXY_IMAGE" "${IMAGE_NAME}:latest" >/dev/null 2>&1 \
      && $DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --no-build --remove-orphans >/dev/null 2>&1; then
      log "rollback triggered using previous image: $PREVIOUS_PROXY_IMAGE"
    else
      hint "自动回滚也失败了，请手动检查 docker compose 状态和镜像标签。"
    fi
  else
    log "rollback skipped because no previous proxy image was captured"
  fi

  exit "$exit_code"
}

DRY_RUN=0
APP_ENV=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    test|prod)
      if [[ -n "$APP_ENV" ]]; then
        fail_usage "duplicate environment argument: $1"
      fi
      APP_ENV="$1"
      ;;
    *)
      fail_usage "invalid environment or option: $1"
      ;;
  esac
  shift
done

if [[ -z "$APP_ENV" ]]; then
  fail_usage "missing environment argument"
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
export DOCKER_CONFIG
mkdir -p "$DOCKER_CONFIG"

log "deploy_id=$DEPLOY_ID"
log "environment=$APP_ENV branch=$BRANCH port=$PROXY_PORT compose_file=$COMPOSE_FILE project=$PROJECT_NAME"
log "root_dir=$ROOT_DIR health_url=$HEALTH_URL"

if [[ "$DRY_RUN" == "1" ]]; then
  log "dry run enabled; skipping git sync, build, and deploy steps"
  exit 0
fi

require_command "git" "请确认服务器已安装 Git，并且当前目录是 qcc_plus 仓库。"
require_command "npm" "请确认服务器已安装 Node.js 20+ 和 npm，并能在 frontend/ 目录执行 npm ci。"
require_command "curl" "请确认服务器已安装 curl，用于部署后的本机健康检查。"
require_command "docker" "请确认服务器已安装 Docker，并且当前用户有权限访问 Docker daemon。"
resolve_docker_compose

if [[ ! -f "$COMPOSE_FILE" ]]; then
  error "compose file not found: $COMPOSE_FILE"
  hint "请确认部署目录完整，且当前分支包含对应的 docker-compose.*.yml 文件。"
  exit 1
fi

PREVIOUS_PROXY_IMAGE="$(docker images -q "$IMAGE_NAME" | sed -n '1p' || true)"
trap 'handle_error $?' ERR

run_step \
  "syncing branch $BRANCH" \
  "检查远端分支、Deploy Key/SSH 权限，以及仓库目录是否允许执行 git reset、git clean 和 git pull。" \
  sync_branch

run_step \
  "installing frontend dependencies (npm ci)" \
  "检查 Node.js/npm 版本、frontend/package-lock.json 是否一致，以及服务器是否能访问 npm registry。" \
  install_frontend_dependencies

run_step \
  "building frontend bundle" \
  "检查 frontend 构建日志、磁盘空间，以及 scripts/build-frontend.sh 是否能把产物同步到 web/dist。" \
  bash scripts/build-frontend.sh

export VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')"
export GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
export BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
log "building with VERSION=$VERSION GIT_COMMIT=$GIT_COMMIT BUILD_DATE=$BUILD_DATE"

run_step \
  "validating docker compose config ($COMPOSE_FILE)" \
  "检查 compose 文件语法、.env 配置和 Docker Compose 插件状态。" \
  validate_compose_config

run_step \
  "cleaning up old containers with fixed names" \
  "如果这里失败，请检查是否存在手动创建的同名容器，或当前用户是否具备 Docker 操作权限。" \
  remove_fixed_name_containers

run_step \
  "stopping existing containers (volumes preserved)" \
  "如果 down 失败，请检查 Docker daemon 状态，以及 compose project 是否被其他进程占用。" \
  stop_existing_containers

run_step \
  "building and deploying containers" \
  "检查 Docker build 输出、基础镜像拉取权限、.env 配置和宿主机磁盘空间。" \
  $DOCKER_COMPOSE -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --build --force-recreate

run_step \
  "waiting for mysql to be healthy" \
  "请重点查看 mysql 容器日志、卷权限、数据库初始化脚本和端口冲突。" \
  wait_for_mysql

run_step \
  "waiting for service health at ${HEALTH_URL}" \
  "请检查代理容器日志、监听端口映射、上游配置，以及服务器本机到 ${HEALTH_URL} 的连通性。" \
  wait_for_service "$HEALTH_URL"

run_step \
  "cleaning old containers/images" \
  "如果清理阶段异常，可手动执行 docker image prune/docker container prune；这不会影响已完成的部署结果。" \
  cleanup_images

trap - ERR
log "deploy completed for $APP_ENV (branch=$BRANCH, port=$PROXY_PORT)"
