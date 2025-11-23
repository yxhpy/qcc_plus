# 多租户账号隔离架构设计

## 概述

qcc_plus 多租户系统支持按账号隔离配置和节点，实现以下功能：
- 每个账号拥有独立的节点池和配置
- 管理员可以管理所有账号
- 通过 proxy_api_key 进行请求路由
- 完全的数据隔离和权限控制

## 数据模型

### 1. accounts 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(64) PK | 账号唯一标识 |
| name | VARCHAR(255) | 账号名称 |
| proxy_api_key | VARCHAR(255) UNIQUE | 用于路由识别的 API Key |
| is_admin | BOOLEAN | 是否为管理员账号 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

### 2. nodes 表（新增字段）

| 字段 | 类型 | 说明 |
|------|------|------|
| account_id | VARCHAR(64) FK | 所属账号 ID |
| ... | ... | 其他字段保持不变 |

### 3. config 表（支持账号级别配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| account_id | VARCHAR(64) | 账号 ID（空表示全局配置） |
| ... | ... | 其他配置字段 |

## 路由逻辑

```
┌─────────────┐
│ 客户端请求  │
└──────┬──────┘
       │ x-api-key: <proxy_api_key>
       ▼
┌─────────────────┐
│ 提取 proxy_key  │
└──────┬──────────┘
       │
       ▼
┌──────────────────┐
│ 查找 accounts 表 │
└──────┬───────────┘
       │
       ▼
┌───────────────────┐
│ 获取账号的节点池  │
└──────┬────────────┘
       │
       ▼
┌──────────────────┐
│ 根据权重选择节点 │
└──────┬───────────┘
       │
       ▼
┌─────────────┐
│ 转发到上游  │
└─────────────┘
```

## 权限模型

### 管理员账号 (is_admin=true)

- 可以查看和管理所有账号
- 可以创建、修改、删除任何账号
- 可以查看和管理所有账号的节点
- 可以切换查看不同账号的数据

### 普通账号 (is_admin=false)

- 只能查看和管理自己的账号信息
- 只能管理自己账号下的节点
- 不能访问其他账号的数据

## API 端点

### 账号管理

```
POST   /admin/api/accounts           # 创建账号（需已登录且为管理员）
GET    /admin/api/accounts           # 列出账号（管理员可见全部，普通账号仅见自己）
PUT    /admin/api/accounts?id=xxx    # 更新账号
DELETE /admin/api/accounts?id=xxx    # 删除账号
```

### 节点管理（支持账号过滤）

```
GET    /admin/api/nodes?account_id=xxx  # 获取指定账号的节点
POST   /admin/api/nodes                 # 在当前账号下创建节点
PUT    /admin/api/nodes?id=xxx          # 更新节点
DELETE /admin/api/nodes?id=xxx          # 删除节点
```

### 配置管理（账号级别）

```
GET    /admin/api/config?account_id=xxx  # 获取指定账号配置
PUT    /admin/api/config?account_id=xxx  # 更新账号配置
```

> 管理 API 认证：先通过 `/login` 表单登录（`username`/`password`），获得 `session_token` Cookie 后再访问以上接口；不再使用 `x-admin-key` 头。

## 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| ADMIN_API_KEY | 管理员访问密钥 | - |
| DEFAULT_ACCOUNT_NAME | 默认账号名称 | default |
| DEFAULT_PROXY_API_KEY | 默认代理 API Key（仅内存模式自动创建时使用） | - |

## 使用示例

### 1. 启动代理服务器

```bash
# 设置管理员密钥
export ADMIN_API_KEY=your-admin-secret

# 设置默认账号配置
export DEFAULT_ACCOUNT_NAME=my-company
export DEFAULT_PROXY_API_KEY=proxy-key-123

# 启动服务
go run ./cmd/cccli proxy
```

### 2. 创建新账号（先登录获取 Cookie）

```bash
auth_cookie=cookies.txt
curl -c "$auth_cookie" -X POST -d "username=admin&password=admin123" http://localhost:8000/login

curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "team-a",
    "proxy_api_key": "team-a-proxy-key",
    "is_admin": false
  }'
```

### 3. 为账号添加节点

```bash
curl -b "$auth_cookie" -X POST "http://localhost:8000/admin/api/nodes?account_id=<team-a-id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "team-a-node-1",
    "base_url": "https://api.anthropic.com",
    "api_key": "sk-ant-...",
    "weight": 1
  }'
```

### 4. 客户端使用代理

```bash
# 使用 team-a 的代理 key
curl http://localhost:8000/v1/messages \
  -H "x-api-key: team-a-proxy-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 1024
  }'
```

## 向后兼容性

如果未配置多账号系统且未启用持久化（内存模式）：
1. 系统自动创建名为 "default" 的账号
2. 所有现有节点归属到 default 账号
3. 系统行为与单租户模式一致

启用持久化（设置 `PROXY_MYSQL_DSN`）时不会自动创建默认账号，需要登录后手动创建。

## 安全考虑

1. **API Key 隔离**：
   - proxy_api_key 用于路由，存储在 accounts 表
   - upstream api_key 用于调用上游，存储在 nodes 表
   - 两者完全隔离，互不影响

2. **权限验证**：
   - 管理界面与管理 API 通过登录会话（`session_token` Cookie）鉴权
   - 管理员登录后可管理所有账号；普通账号登录后仅能访问自身资源

3. **数据隔离**：
   - 所有数据库查询都基于 account_id 过滤
   - 防止跨账号数据泄露

4. **日志脱敏**：
   - 日志中不输出完整 API Key
   - 敏感操作记录审计日志

## 迁移指南

### 从单租户升级到多租户

1. **备份数据库**：
   ```bash
   mysqldump -u user -p database > backup.sql
   ```

2. **更新代码**：
   ```bash
   git pull origin main
   go build ./cmd/cccli
   ```

3. **运行迁移**：
   系统会自动执行数据库迁移：
   - 创建 accounts 表
   - 在 nodes 表添加 account_id
   - 创建默认账号
   - 将现有节点关联到默认账号

4. **配置环境变量**：
   ```bash
   export ADMIN_API_KEY=your-secure-admin-key
   export DEFAULT_PROXY_API_KEY=your-proxy-key
   ```

5. **验证迁移**：
   - 访问 /admin 页面
   - 如果未配置 default 账号，请在登录后手动创建目标账号
   - 验证现有节点是否正常工作

## 故障排查

### 路由失败

**问题**：请求返回 "account not found"

**解决**：
1. 检查请求头中的 x-api-key 是否正确
2. 验证 accounts 表中是否存在对应的 proxy_api_key
3. 检查 proxy_api_key 是否已禁用

### 权限拒绝

**问题**：管理 API 返回 403 Forbidden / 401 Unauthorized

**解决**：
1. 确认已通过 `/login` 登录并携带 `session_token` Cookie（使用 `-c/-b` 保存与发送）
2. 检查登录账号是否为管理员（访问多账号资源时需管理员）
3. 确认目标账号存在且未被删除

### 节点无法访问

**问题**：特定账号的节点无法访问

**解决**：
1. 检查节点的 account_id 是否正确
2. 验证账号的 proxy_api_key 是否正确
3. 检查节点是否被禁用或失败

## 性能优化

1. **缓存账号查询**：
   - 在内存中缓存 proxy_api_key -> account_id 的映射
   - 减少数据库查询次数

2. **连接池管理**：
   - 每个账号维护独立的节点连接池
   - 避免跨账号连接竞争

3. **监控指标**：
   - 按账号统计请求量和错误率
   - 监控账号级别的资源使用情况

## 未来扩展

1. **配额管理**：
   - 为每个账号设置请求配额
   - 支持基于使用量的计费

2. **负载均衡策略**：
   - 支持账号级别的自定义负载均衡算法
   - 加权轮询、最少连接等策略

3. **审计日志**：
   - 记录所有账号操作
   - 支持日志查询和导出

4. **SSO 集成**：
   - 支持企业 SSO 登录
   - OAuth 2.0 / OIDC 集成
