# 多租户架构

最后校准：2026-04-02

## 目标

qcc_plus 按账号隔离节点池、配置和监控数据。客户端请求通过 `x-api-key` 找到目标账号，再从该账号自己的节点集合中选择可用节点。

## 核心对象

### 账号

- 标识：`account.id`
- 路由键：`proxy_api_key`
- 权限：`is_admin`
- 登录口令：`name + password`

### 节点

- 归属：`account_id`
- 上游信息：`base_url`、`api_key`
- 调度属性：`weight`、`disabled`、健康状态、熔断状态

### 配置与监控

- 账号级配置存储在 `config/settings`
- 监控与统计按账号、节点分别聚合
- 分享监控通过 token 暴露只读视图

## 路由流程

```text
客户端请求
  -> 读取 x-api-key
  -> 匹配账号
  -> 读取该账号节点池
  -> 根据权重、健康状态、熔断状态选择节点
  -> 转发到上游 /v1/messages
```

如果请求中的 `x-api-key` 没有匹配到账号，则会尝试使用 `defaultAccount`；但当前实现不会自动创建默认普通账号，因此生产上应显式创建业务账号。

## 权限模型

### 管理员

- 可查看和管理全部账号
- 可管理全部节点
- 可访问系统级配置、定价、Tunnel 等页面

### 普通账号

- 只能访问自己的节点和统计数据
- 不能管理其他账号
- 不能访问管理员专属页面

## 登录与认证

- 管理后台通过 `POST /login` 登录
- 成功后写入 `session_token` Cookie
- `/admin/api/*` 和大多数 `/api/*` 端点依赖该 Cookie
- `/v1/messages` 只依赖 `x-api-key`

## 当前默认账号策略

- 自动创建管理员账号 `admin/admin123`
- 默认普通账号不会自动创建
- `DEFAULT_ACCOUNT_NAME` 和 `DEFAULT_PROXY_API_KEY` 仍然保留，主要用于默认账号口径和兼容逻辑，不等于“自动创建默认账号”

## 存储模式

### SQLite 默认

- 默认启用 SQLite
- 默认路径：`~/.qccplus/qccplus.db`
- 可通过 `PROXY_SQLITE_PATH` 覆盖

### MySQL 可选

- 设置 `PROXY_MYSQL_DSN` 后切换到 MySQL
- 适合多实例或集中持久化部署

## 管理接口

### 账号

- `GET|POST|PUT|DELETE /admin/api/accounts`

### 节点

- `GET|POST|PUT|DELETE /admin/api/nodes`
- `POST /admin/api/nodes/activate`
- `POST /admin/api/nodes/disable`
- `POST /admin/api/nodes/enable`

### 账号相关数据

- `GET /api/accounts/:id/metrics`
- `GET /api/nodes/:id/metrics`
- `GET /api/nodes/:id/health-history`
- `GET /api/usage/logs`
- `GET /api/usage/summary`

## 隔离边界

1. 节点按 `account_id` 归属。
2. 指标和使用量查询会按账号限制。
3. 普通账号登录后只能获取自己的数据。
4. 分享监控只暴露被授权的只读数据。

## 相关实现

- `internal/proxy/handler.go`
- `internal/proxy/api_accounts.go`
- `internal/proxy/api_nodes.go`
- `internal/proxy/api_actions.go`
- `internal/proxy/server.go`
- `internal/store/account.go`
- `internal/store/node.go`

## 相关文档

- [快速开始](./quick-start-multi-tenant.md)
- [API 索引](./api/INDEX.md)
- [健康检查机制](./health_check_mechanism.md)
