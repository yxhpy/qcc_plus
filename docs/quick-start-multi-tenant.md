# 多租户模式快速开始指南

## 概述

- 系统默认启用多租户，无需额外开关。
- 默认登录账号（仅内存模式自动创建）：管理员 `admin/admin123`，普通账号 `default/default123`。
- 配置了 `PROXY_MYSQL_DSN`（持久化模式）时不会自动创建默认账号，请登录后手动创建第一个账号和节点。
- 以上默认凭证仅供本地试用，生产环境请尽快修改密码与密钥。
- 登录方式：`POST /login`（表单 `username`、`password`），成功后获得 `session_token` Cookie，用于访问 `/admin` 与 `/admin/api/*`。

## 快速开始

### 方式一：开箱即用（本地验证）

```bash
# 1) 启动代理（默认多租户 + 默认凭证）
LISTEN_ADDR=:8000 \
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy

# 日志会打印（内存模式）：
# - 管理员登录：username=admin password=admin123
# - 默认账号：username=default password=default123
# 提示：配置 PROXY_MYSQL_DSN 时不会自动创建默认账号，需要登录后手动创建。

# 2) 访问管理界面（登录页）
open "http://localhost:8000/admin"

# 3) 使用默认账号调用代理（仅在存在默认账号且 proxy_api_key 为 default-proxy-key 时）
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 256
  }'
```

### 方式二：生产化启动（推荐，先改掉默认密钥）

```bash
# 修改默认凭证
export ADMIN_API_KEY=my-secure-admin-key
export DEFAULT_ACCOUNT_NAME=team-alpha
export DEFAULT_PROXY_API_KEY=alpha-proxy-key-123

# 配置上游与持久化
export UPSTREAM_BASE_URL=https://api.anthropic.com
export UPSTREAM_API_KEY=sk-ant-alpha-upstream-key
export UPSTREAM_NAME=alpha-node-1
export PROXY_MYSQL_DSN=user:pass@tcp(localhost:3306)/qcc_plus?parseTime=true

# 启动
go run ./cmd/cccli proxy

# 提示：启用 PROXY_MYSQL_DSN 时不会自动创建默认账号，登录后请在管理界面创建账号与节点。
```

> 提醒：若首次启动时仍使用默认凭证，请在验证通过后立刻更新 `ADMIN_API_KEY` 与 `DEFAULT_PROXY_API_KEY` 并重启。

## 账号管理操作

> 先登录并保存 Cookie（管理员）：
> ```bash
> auth_cookie=cookies.txt
> curl -c "$auth_cookie" -X POST -d "username=admin&password=admin123" http://localhost:8000/login
> ```

### 列出所有账号

```bash
curl -b "$auth_cookie" http://localhost:8000/admin/api/accounts
```

### 查看账号的节点

```bash
curl -b "$auth_cookie" "http://localhost:8000/admin/api/nodes?account_id=<account-id>"
```

### 更新账号信息

```bash
curl -b "$auth_cookie" -X PUT "http://localhost:8000/admin/api/accounts?id=<account-id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "new-name",
    "proxy_api_key": "new-proxy-key"
  }'
```

### 删除账号

```bash
curl -b "$auth_cookie" -X DELETE "http://localhost:8000/admin/api/accounts?id=<account-id>"
```

## 节点管理操作

> 使用登录获得的 Cookie；如需操作指定账号，管理员可通过 `account_id` 查询参数指定目标账号。

### 添加节点（到指定账号）

```bash
curl -b "$auth_cookie" -X POST "http://localhost:8000/admin/api/nodes?account_id=<account-id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "new-node",
    "base_url": "https://api.anthropic.com",
    "api_key": "sk-ant-xxx",
    "weight": 1
  }'
```

### 更新节点

```bash
curl -b "$auth_cookie" -X PUT "http://localhost:8000/admin/api/nodes?id=<node-id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "updated-node",
    "base_url": "https://api.anthropic.com",
    "api_key": "sk-ant-xxx",
    "weight": 2
  }'
```

### 删除节点

```bash
curl -b "$auth_cookie" -X DELETE "http://localhost:8000/admin/api/nodes?id=<node-id>"
```

### 激活节点

```bash
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes/activate \
  -H "Content-Type: application/json" \
  -d '{"id": "<node-id>"}'
```

### 禁用/启用节点

```bash
# 禁用
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes/disable \
  -H "Content-Type: application/json" \
  -d '{"id": "<node-id>"}'

# 启用
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes/enable \
  -H "Content-Type: application/json" \
  -d '{"id": "<node-id>"}'
```

## Docker 部署

### docker-compose.yml

```yaml
version: '3.8'

services:
  proxy:
    build: .
    ports:
      - "8000:8000"
    environment:
      - LISTEN_ADDR=:8000
      - ADMIN_API_KEY=your-admin-key
      - DEFAULT_ACCOUNT_NAME=default
      - DEFAULT_PROXY_API_KEY=default-proxy-key
      - UPSTREAM_BASE_URL=https://api.anthropic.com
      - UPSTREAM_API_KEY=sk-ant-your-key
      - PROXY_MYSQL_DSN=qcc:example@tcp(mysql:3306)/qcc_proxy?parseTime=true
      - PROXY_RETRY_MAX=3
      - PROXY_FAIL_THRESHOLD=3
      - PROXY_HEALTH_INTERVAL_SEC=30
    depends_on:
      - mysql

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=example
      - MYSQL_DATABASE=qcc_proxy
      - MYSQL_USER=qcc
      - MYSQL_PASSWORD=example
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3307:3306"

volumes:
  mysql_data:
```

### 启动

```bash
docker-compose up -d
```

## 常见问题

### 1. 老版本单租户如何升级？

现在已默认启用多租户。升级后如果启用持久化（设置 `PROXY_MYSQL_DSN`），系统只会创建管理员账号，不再自动创建默认账号；请登录后手动创建账号与节点，并将历史节点迁移到目标账号。建议同时配置新的 `ADMIN_API_KEY` 与 `DEFAULT_PROXY_API_KEY` 后再重启。

### 2. 如何重置管理员密钥？

重新设置 `ADMIN_API_KEY` 环境变量并重启服务即可。

### 3. proxy_api_key 和 upstream api_key 的区别？

- **proxy_api_key**：用于路由识别，决定请求发送到哪个账号的节点池
- **upstream api_key**：存储在节点配置中，用于调用上游 API

### 4. 如何查看当前使用的是哪个节点？

响应头中包含 `X-Proxy-Node` 字段，显示当前使用的节点名称。

### 5. 账号之间的数据是否隔离？

是的，完全隔离：
- 每个账号有独立的节点池
- 配置按账号隔离
- 数据库中通过 account_id 严格隔离

## 最佳实践

1. **proxy_api_key 管理**
   - 使用强随机字符串
   - 定期轮换
   - 不要在日志或前端暴露

2. **权限控制**
   - 生产环境必须设置 ADMIN_API_KEY
   - 管理员密钥要妥善保管
   - 普通账号只给必要权限

3. **节点配置**
   - 为关键业务配置多个节点实现高可用
   - 使用 weight 控制节点优先级
   - 定期检查节点健康状态

4. **监控**
   - 通过管理界面查看各账号的请求统计
   - 监控节点失败率和延迟
   - 设置告警

## 下一步

- 查看 [完整架构文档](./multi-tenant-architecture.md)
- 查看 [API 参考](./api-reference.md)（待创建）
- 查看 [故障排查指南](./troubleshooting.md)（待创建）
