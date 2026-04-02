#!/usr/bin/env bash
# 测试 Docker 数据持久化
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="${1:-docker-compose.yml}"
COMPOSE_PROJECT="qcc_persistence_test"
MYSQL_CONTAINER="${COMPOSE_PROJECT}_mysql"
PROXY_CONTAINER="${COMPOSE_PROJECT}_proxy"
PROXY_PORT="8002"
ADMIN_KEY="test-admin-key"

log "测试配置: compose_file=$COMPOSE_FILE project=$COMPOSE_PROJECT port=$PROXY_PORT"

# 清理之前的测试
log "清理之前的测试环境"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" down 2>/dev/null || true
docker rm -f "$MYSQL_CONTAINER" "$PROXY_CONTAINER" 2>/dev/null || true

# 创建测试用的 .env 文件
cat > .env.test-persistence <<EOF
MYSQL_ROOT_PASSWORD=test123
MYSQL_DATABASE=qcc_test_persist
MYSQL_USER=qcc_test
MYSQL_PASSWORD=test123
MYSQL_PORT=3309
PROXY_PORT=$PROXY_PORT
UPSTREAM_BASE_URL=https://api.anthropic.com
UPSTREAM_API_KEY=test-key
ADMIN_API_KEY=$ADMIN_KEY
PROXY_MYSQL_DSN=qcc_test:test123@tcp(mysql:3306)/qcc_test_persist?parseTime=true
EOF

# 首次部署
log "首次部署服务"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file .env.test-persistence up -d --build

# 等待服务就绪
log "等待服务就绪"
for ((i=1; i<=60; i++)); do
  if curl -s "http://localhost:$PROXY_PORT/version" >/dev/null 2>&1; then
    log "服务已就绪"
    break
  fi
  if [[ "$i" -eq 60 ]]; then
    log "错误: 服务未能在 60 秒内就绪"
    docker logs "$PROXY_CONTAINER" --tail 50
    exit 1
  fi
  sleep 1
done

# 写入测试数据
log "写入测试数据 - 创建测试账号"
TEST_ACCOUNT_NAME="test-persist-account-$(date +%s)"
CREATE_RESPONSE=$(curl -s -X POST "http://localhost:$PROXY_PORT/admin/api/accounts" \
  -H "x-api-key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$TEST_ACCOUNT_NAME\",\"proxy_api_key\":\"test-key-123\",\"is_admin\":false}")

if echo "$CREATE_RESPONSE" | grep -q "error"; then
  log "错误: 创建账号失败 - $CREATE_RESPONSE"
  exit 1
fi

log "测试账号已创建: $TEST_ACCOUNT_NAME"

# 验证数据存在
log "验证数据存在"
ACCOUNTS_RESPONSE=$(curl -s "http://localhost:$PROXY_PORT/admin/api/accounts" \
  -H "x-api-key: $ADMIN_KEY")

if ! echo "$ACCOUNTS_RESPONSE" | grep -q "$TEST_ACCOUNT_NAME"; then
  log "错误: 无法找到刚创建的账号"
  exit 1
fi

log "数据验证成功"

# 重新部署
log "停止服务（保留数据卷）"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file .env.test-persistence down

log "重新部署服务"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file .env.test-persistence up -d

# 等待服务再次就绪
log "等待服务重新就绪"
for ((i=1; i<=60; i++)); do
  if curl -s "http://localhost:$PROXY_PORT/version" >/dev/null 2>&1; then
    log "服务已重新就绪"
    break
  fi
  if [[ "$i" -eq 60 ]]; then
    log "错误: 服务未能在 60 秒内重新就绪"
    docker logs "$PROXY_CONTAINER" --tail 50
    exit 1
  fi
  sleep 1
done

# 验证数据持久化
log "验证数据持久化"
ACCOUNTS_AFTER=$(curl -s "http://localhost:$PROXY_PORT/admin/api/accounts" \
  -H "x-api-key: $ADMIN_KEY")

if ! echo "$ACCOUNTS_AFTER" | grep -q "$TEST_ACCOUNT_NAME"; then
  log "错误: 重新部署后数据丢失！"
  log "响应: $ACCOUNTS_AFTER"
  exit 1
fi

log "✓ 数据持久化测试通过！"
log "测试账号 $TEST_ACCOUNT_NAME 在重新部署后仍然存在"

# 清理测试环境
log "清理测试环境"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file .env.test-persistence down
rm -f .env.test-persistence

log "✓ 数据持久化测试完成"
