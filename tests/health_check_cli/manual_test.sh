#!/bin/bash
# 手动测试 CLI 健康检查功能

set -e

echo "=== 健康检查 CLI 方式手动测试 ==="
echo

# 1. 确保 Docker 镜像已构建
echo "[1/5] 检查 Claude Code CLI Docker 镜像..."
if ! docker images | grep -q claude-code-cli-verify; then
    echo "构建 Claude Code CLI Docker 镜像..."
    cd verify/claude_code_cli
    docker build -f Dockerfile.verify_pass -t claude-code-cli-verify .
    cd ../..
fi
echo "✓ Docker 镜像已就绪"
echo

# 2. 启动代理服务器（使用临时配置）
echo "[2/5] 启动代理服务器..."
export UPSTREAM_BASE_URL="https://www.88code.org/api"
export UPSTREAM_API_KEY="88_820f837e55c1d16735a79fa4c7cfdfeeab135f965e65b1b930e8f1543998caae"
export ANTHROPIC_AUTH_TOKEN="88_820f837e55c1d16735a79fa4c7cfdfeeab135f965e65b1b930e8f1543998caae"
export LISTEN_ADDR=":8888"
export PROXY_HEALTH_INTERVAL_SEC=10
go run ./cmd/cccli proxy > /tmp/qcc_test.log 2>&1 &
SERVER_PID=$!
echo "✓ 服务器已启动 (PID: $SERVER_PID)"
echo

# 等待服务器启动
echo "[3/5] 等待服务器就绪..."
sleep 3
echo "✓ 服务器就绪"
echo

# 3. 登录获取 session token
echo "[4/5] 登录管理界面..."
LOGIN_RESP=$(curl -s -X POST http://localhost:8888/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')

if echo "$LOGIN_RESP" | grep -q "session_token"; then
    SESSION_TOKEN=$(echo "$LOGIN_RESP" | grep -o '"session_token":"[^"]*"' | cut -d'"' -f4)
    echo "✓ 登录成功，获得 session token"
else
    echo "✗ 登录失败: $LOGIN_RESP"
    kill $SERVER_PID
    exit 1
fi
echo

# 4. 创建使用 CLI 健康检查的节点
echo "[5/5] 创建 CLI 健康检查节点..."
CREATE_RESP=$(curl -s -X POST http://localhost:8888/admin/api/nodes \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=$SESSION_TOKEN" \
  -d '{
    "name": "cli_test_node",
    "base_url": "https://www.88code.org/api",
    "api_key": "88_820f837e55c1d16735a79fa4c7cfdfeeab135f965e65b1b930e8f1543998caae",
    "health_check_method": "cli",
    "weight": 1
  }')

if echo "$CREATE_RESP" | grep -q '"id"'; then
    NODE_ID=$(echo "$CREATE_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "✓ 节点创建成功 (ID: $NODE_ID)"
else
    echo "✗ 节点创建失败: $CREATE_RESP"
    kill $SERVER_PID
    exit 1
fi
echo

# 5. 等待健康检查执行
echo "等待 15 秒让健康检查执行..."
sleep 15

# 6. 查看节点状态
echo
echo "=== 查看节点状态 ==="
curl -s -X GET http://localhost:8888/admin/api/nodes \
  -H "Cookie: session_token=$SESSION_TOKEN" | jq '.nodes[] | select(.name=="cli_test_node") | {name, health_check_method, failed, last_health_check_at, last_ping_ms, last_ping_error}'

echo
echo "=== 查看服务器日志（最后20行）==="
tail -20 /tmp/qcc_test.log

# 清理
echo
echo "=== 清理 ==="
kill $SERVER_PID
echo "✓ 服务器已停止"
echo
echo "测试完成！"
