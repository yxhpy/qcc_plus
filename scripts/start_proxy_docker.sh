#!/usr/bin/env bash
# 启动本地 docker compose（mysql + proxy），加载环境变量，确保 ASCII 与可靠性。
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# 将 Docker 配置写到本项目目录，避免无权限写入用户主目录
export DOCKER_CONFIG="${DOCKER_CONFIG:-$ROOT_DIR/.docker}"
mkdir -p "$DOCKER_CONFIG"

# 自动编译前端（确保 web/dist 是最新的）
echo "[info] 检查并编译前端..."
if [ -f "scripts/build-frontend.sh" ]; then
  bash scripts/build-frontend.sh
else
  echo "[warn] 未找到 scripts/build-frontend.sh，跳过前端编译"
fi

ENV_FILE="${ENV_FILE:-.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "[info] 未找到 ${ENV_FILE}，改用 .env.example"
  ENV_FILE=".env.example"
fi

echo "[info] 使用环境文件: ${ENV_FILE}"
line=""
while IFS= read -r line || [[ -n "${line:-}" ]]; do
  [[ -z "${line:-}" || "${line}" =~ ^# ]] && continue
  export "$line"
done < "$ENV_FILE"

if docker compose version >/dev/null 2>&1; then
  DOCKER_COMPOSE="docker compose"
else
  DOCKER_COMPOSE="docker-compose"
fi

echo "[info] 构建并启动 docker compose（mysql + proxy） -> ${DOCKER_COMPOSE} up -d --build"
${DOCKER_COMPOSE} up -d --build

echo "[info] 当前容器状态"
${DOCKER_COMPOSE} ps

echo "[info] 管理页: http://localhost:8000/admin"
