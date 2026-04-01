# 多租户快速开始

最后校准：2026-04-02

## 当前默认行为

- 多租户模式默认开启，无需额外开关。
- 服务默认使用 SQLite，不是内存模式。
- 默认会自动创建管理员账号 `admin/admin123`。
- 默认普通账号不会自动创建。
- 要使用代理，必须先创建普通账号并为其配置节点。

## 方式一：npm CLI

```bash
npm install -g @qccplus/cli
qccplus config init
qccplus config set upstream.api_key sk-ant-your-key
qccplus start
```

访问：`http://localhost:8000/admin`

## 方式二：Docker

```bash
docker compose up -d
```

## 方式三：源码运行

```bash
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy
```

## 首次配置流程

### 1. 登录

```bash
auth_cookie=cookies.txt

curl -c "$auth_cookie" -X POST \
  -d "username=admin&password=admin123" \
  http://localhost:8000/login
```

### 2. 创建账号

```bash
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "name":"team-alpha",
    "proxy_api_key":"alpha-key",
    "is_admin":false
  }'
```

### 3. 添加节点

```bash
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name":"alpha-node-1",
    "base_url":"https://api.anthropic.com",
    "api_key":"sk-ant-your-key",
    "weight":1
  }'
```

### 4. 通过代理访问 Claude

```bash
curl http://localhost:8000/v1/messages \
  -H "x-api-key: alpha-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-5-20250929",
    "messages":[{"role":"user","content":"Hello"}],
    "max_tokens":128
  }'
```

## 常见操作

### 查看账号

```bash
curl -b "$auth_cookie" http://localhost:8000/admin/api/accounts
```

### 查看节点

```bash
curl -b "$auth_cookie" http://localhost:8000/admin/api/nodes
```

### 激活节点

```bash
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes/activate \
  -H "Content-Type: application/json" \
  -d '{"id":"<node-id>"}'
```

### 禁用节点

```bash
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes/disable \
  -H "Content-Type: application/json" \
  -d '{"id":"<node-id>"}'
```

## 持久化选择

### 默认 SQLite

- 默认路径：`~/.qccplus/qccplus.db`
- 可通过 `PROXY_SQLITE_PATH` 指定新路径

### 切换到 MySQL

```bash
export PROXY_MYSQL_DSN='user:pass@tcp(localhost:3306)/qcc_plus?parseTime=true'
qccplus restart
```

## 建议

1. 生产环境立即修改默认管理员密码与相关密钥。
2. 每个账号至少配置两个节点以提高可用性。
3. 用权重控制优先级，数值越小优先级越高。
4. 涉及分享监控、通知、定价和使用量时，优先使用持久化存储。

## 下一步

- [多租户架构](./multi-tenant-architecture.md)
- [API 索引](./api/INDEX.md)
- [健康检查机制](./health_check_mechanism.md)
