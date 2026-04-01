# API 索引

最后校准：2026-04-02

本文档提供 qcc_plus 当前 HTTP 接口和核心 Go 包入口的快速索引，详细行为以 `internal/proxy/`、`internal/store/`、`internal/notify/`、`internal/tunnel/` 实现为准。

## HTTP 路由

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/login` | 表单登录，成功后写入 `session_token` Cookie |
| `POST` | `/logout` | 注销登录 |
| `GET` | `/version` | 返回版本信息 |

### 代理入口

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/v1/messages` | Claude 兼容消息代理入口，按 `x-api-key` 路由账号 |

### 管理接口

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` `POST` `PUT` `DELETE` | `/admin/api/accounts` | 账号管理 |
| `GET` `POST` `PUT` `DELETE` | `/admin/api/nodes` | 节点管理 |
| `POST` | `/admin/api/nodes/activate` | 激活节点 |
| `POST` | `/admin/api/nodes/disable` | 禁用节点 |
| `POST` | `/admin/api/nodes/enable` | 启用节点 |
| `GET` `PUT` | `/admin/api/config` | 账号配置 |
| `GET` `PUT` | `/admin/api/tunnel` | Tunnel 配置 |
| `POST` | `/admin/api/tunnel/start` | 启动 Tunnel |
| `POST` | `/admin/api/tunnel/stop` | 停止 Tunnel |
| `GET` | `/admin/api/tunnel/zones` | 获取 Cloudflare Zones |

### 监控与实时数据

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/monitor/dashboard` | 监控大屏数据 |
| `GET` | `/api/monitor/ws` | 监控 WebSocket |
| `POST` `GET` | `/api/monitor/shares` | 创建/查询分享链接 |
| `DELETE` | `/api/monitor/shares/:id` | 撤销分享链接 |
| `GET` | `/api/monitor/share/:token` | 通过 token 访问分享数据 |

### 指标与健康历史

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/nodes/:id/metrics` | 节点指标 |
| `GET` | `/api/nodes/:id/health-history` | 节点健康历史 |
| `GET` | `/api/accounts/:id/metrics` | 账号指标 |
| `POST` | `/api/metrics/aggregate` | 手动触发指标聚合 |
| `POST` | `/api/metrics/cleanup` | 手动触发指标清理 |

### 设置、环境变量与 Claude 配置

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/settings/version` | 设置版本号 |
| `GET` | `/api/settings` | 查询设置 |
| `POST` | `/api/settings/batch` | 批量更新设置 |
| `GET` `PUT` `DELETE` | `/api/settings/:key` | 单项设置操作 |
| `GET` | `/api/envvars` | 环境变量定义与当前值 |
| `GET` | `/api/envvars/categories` | 环境变量分类 |
| `GET` | `/api/claude-config/template` | 生成 Claude Code 配置模板 |
| `GET` | `/api/claude-config/download/:id` | 下载配置文件 |

### 定价、使用量与请求日志

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` `POST` `DELETE` | `/api/pricing` | 模型定价管理 |
| `POST` | `/api/pricing/sync` | 同步官方定价 |
| `GET` | `/api/usage/logs` | 请求日志与尝试详情 |
| `GET` | `/api/usage/summary` | 使用量汇总 |
| `POST` | `/api/usage/cleanup` | 清理历史使用日志 |

### 通知系统

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` `POST` | `/api/notification/channels` | 通知渠道列表与创建 |
| `PUT` `DELETE` | `/api/notification/channels/:id` | 更新/删除渠道 |
| `GET` `POST` | `/api/notification/subscriptions` | 订阅列表与创建 |
| `PUT` `DELETE` | `/api/notification/subscriptions/:id` | 更新/删除订阅 |
| `GET` | `/api/notification/event-types` | 事件类型枚举 |
| `POST` | `/api/notification/test` | 测试通知 |

### 模型恢复

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` `POST` | `/api/model-recovery` | 查询/更新模型恢复状态 |
| `POST` | `/api/model-recovery/dismiss` | 忽略恢复提醒 |

## 认证规则

- 管理接口和大多数 `/api/*` 端点要求登录态。
- 登录通过 `POST /login` 写入 `session_token` Cookie。
- 分享监控相关接口支持通过分享 token 访问公开数据。
- `/v1/messages` 使用 `x-api-key` 做账号路由，不依赖后台登录态。

## 主要实现文件

### HTTP 路由注册

- `internal/proxy/handler.go`
- `internal/proxy/api_*.go`
- `internal/proxy/settings_handler.go`

### 指标与监控

- `internal/proxy/api_monitor.go`
- `internal/proxy/api_monitor_share.go`
- `internal/proxy/api_metrics.go`
- `internal/proxy/api_health_history.go`
- `internal/proxy/api_ws.go`

### 账号、节点、认证

- `internal/proxy/api_accounts.go`
- `internal/proxy/api_nodes.go`
- `internal/proxy/api_actions.go`
- `internal/proxy/session.go`
- `internal/proxy/session_auth.go`

## Go 包入口索引

### `internal/client`

- `LoadConfig(args []string) (*Config, error)`
- `Run(cfg *Config) error`
- `NewClient(cfg *Config) *Client`

职责：Claude CLI 客户端兼容能力、请求构造、SSE 流处理。

### `internal/proxy`

- `NewBuilder() *Builder`
- `(*Builder).Build() (*Server, error)`
- `(*Server).Start() error`

职责：代理服务、管理 API、健康检查、重试、负载均衡、监控。

### `internal/store`

- `Open(dsn string) (*Store, error)`
- `OpenSQLite(dsn string) (*Store, error)`

职责：MySQL / SQLite 存储，涵盖账号、节点、设置、监控、通知、Tunnel、定价、使用量。

### `internal/notify`

- `NewManager(...)`
- `BuildChannel(...)`

职责：通知渠道构建、订阅过滤、事件投递。

### `internal/tunnel`

- `NewManager(...)`

职责：Cloudflare Tunnel 生命周期管理。

### `internal/version`

- `GetVersionInfo() Info`
- `GetFormattedBuildDate() string`

职责：构建版本信息输出。

## 相关文档

- [模块注册表](../modules/REGISTRY.md)
- [多租户架构](../multi-tenant-architecture.md)
- [健康检查机制](../health_check_mechanism.md)
- [前端技术栈](../frontend-tech-stack.md)
- [文档索引](../README.md)
