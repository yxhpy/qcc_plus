# API 索引

> 自动生成的 API 参考文档 | 最后更新: 2026-02-02

## 说明

本文档提供项目中主要 API 的快速参考。详细信息请查看源代码和相关文档。

## 目录

- [Client API](#client-api) - Claude API 客户端
- [Proxy API](#proxy-api) - 代理服务器
- [Store API](#store-api) - 数据存储
- [Notify API](#notify-api) - 通知系统
- [Tunnel API](#tunnel-api) - 隧道管理
- [管理 API](#管理-api) - Web 管理接口

---

## Client API

### `NewClient(config *Config) *Client`

创建新的 Claude API 客户端。

**参数**:
- `config` (*Config): 客户端配置

**返回**: *Client - 客户端实例

**示例**:
```go
client := client.NewClient(&client.Config{
    BaseURL: "https://api.anthropic.com",
    APIKey:  "sk-ant-xxx",
})
```

**位置**: internal/client/client.go

### `SendMessage(ctx context.Context, req *MessageRequest) (*MessageResponse, error)`

发送消息到 Claude API。

**参数**:
- `ctx` (context.Context): 上下文
- `req` (*MessageRequest): 消息请求

**返回**:
- *MessageResponse - 响应
- error - 错误

**位置**: internal/client/client.go

### `StreamMessage(ctx context.Context, req *MessageRequest) (<-chan Event, error)`

流式发送消息到 Claude API。

**参数**:
- `ctx` (context.Context): 上下文
- `req` (*MessageRequest): 消息请求

**返回**:
- <-chan Event - 事件流
- error - 错误

**位置**: internal/client/client.go

---

## Proxy API

### `NewProxy(store *store.Store) *Proxy`

创建新的代理服务器。

**参数**:
- `store` (*store.Store): 数据存储

**返回**: *Proxy - 代理实例

**位置**: internal/proxy/handler.go

### `HandleRequest(w http.ResponseWriter, r *http.Request)`

处理代理请求。

**参数**:
- `w` (http.ResponseWriter): 响应写入器
- `r` (*http.Request): HTTP 请求

**位置**: internal/proxy/handler.go

### `SelectNode(accountID string, excludeIDs []string) (*Node, error)`

选择可用节点。

**参数**:
- `accountID` (string): 账号 ID
- `excludeIDs` ([]string): 排除的节点 ID

**返回**:
- *Node - 选中的节点
- error - 错误

**位置**: internal/proxy/node_manager.go

### `CheckHealth(node *Node) error`

检查节点健康状态。

**参数**:
- `node` (*Node): 节点信息

**返回**: error - 错误（nil 表示健康）

**位置**: internal/proxy/health.go

---

## Store API

### `NewStore(dsn string) (*Store, error)`

创建新的数据存储。

**参数**:
- `dsn` (string): 数据库连接字符串

**返回**:
- *Store - 存储实例
- error - 错误

**位置**: internal/store/store.go

### `GetAccount(id string) (*Account, error)`

获取账号信息。

**参数**:
- `id` (string): 账号 ID

**返回**:
- *Account - 账号信息
- error - 错误

**位置**: internal/store/account.go

### `UpsertNode(node *Node) error`

更新或插入节点。

**参数**:
- `node` (*Node): 节点信息

**返回**: error - 错误

**位置**: internal/store/node.go

### `RecordMetrics(metrics *Metrics) error`

记录指标数据。

**参数**:
- `metrics` (*Metrics): 指标数据

**返回**: error - 错误

**位置**: internal/store/metrics.go

---

## Notify API

### `NewManager(store *store.Store) *Manager`

创建通知管理器。

**参数**:
- `store` (*store.Store): 数据存储

**返回**: *Manager - 管理器实例

**位置**: internal/notify/manager.go

### `SendNotification(notification *Notification) error`

发送通知。

**参数**:
- `notification` (*Notification): 通知内容

**返回**: error - 错误

**位置**: internal/notify/manager.go

### `GetNotifications(accountID string, limit int) ([]*Notification, error)`

获取通知列表。

**参数**:
- `accountID` (string): 账号 ID
- `limit` (int): 数量限制

**返回**:
- []*Notification - 通知列表
- error - 错误

**位置**: internal/notify/store.go

---

## Tunnel API

### `NewManager(config *Config) *Manager`

创建隧道管理器。

**参数**:
- `config` (*Config): 隧道配置

**返回**: *Manager - 管理器实例

**位置**: internal/tunnel/manager.go

### `CreateTunnel(subdomain string) (*Tunnel, error)`

创建 Cloudflare Tunnel。

**参数**:
- `subdomain` (string): 子域名

**返回**:
- *Tunnel - 隧道信息
- error - 错误

**位置**: internal/tunnel/manager.go

### `DeleteTunnel(tunnelID string) error`

删除隧道。

**参数**:
- `tunnelID` (string): 隧道 ID

**返回**: error - 错误

**位置**: internal/tunnel/manager.go

---

## 管理 API

### POST /login

用户登录。

**请求体**:
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**响应**:
```json
{
  "session_token": "xxx",
  "expires_at": "2026-02-03T23:25:00+08:00"
}
```

### GET /admin/api/accounts

获取账号列表（需要登录）。

**响应**:
```json
{
  "accounts": [
    {
      "id": "xxx",
      "name": "default",
      "proxy_api_key": "xxx",
      "is_admin": false
    }
  ]
}
```

### POST /admin/api/accounts

创建账号（需要管理员权限）。

**请求体**:
```json
{
  "name": "team-alpha",
  "proxy_api_key": "alpha-key",
  "is_admin": false
}
```

### GET /admin/api/nodes

获取节点列表（需要登录）。

**响应**:
```json
{
  "nodes": [
    {
      "id": "xxx",
      "name": "node-1",
      "base_url": "https://api.anthropic.com",
      "weight": 1,
      "active": true,
      "failed": false
    }
  ]
}
```

### POST /admin/api/nodes

创建节点（需要登录）。

**请求体**:
```json
{
  "name": "node-1",
  "base_url": "https://api.anthropic.com",
  "api_key": "sk-ant-xxx",
  "weight": 1
}
```

### POST /v1/messages

代理 Claude API 请求。

**请求头**:
- `x-api-key`: 账号的 proxy_api_key

**请求体**: 标准 Claude API 格式

**响应**: 标准 Claude API 响应

---

## 相关文档

- [模块注册表](../modules/REGISTRY.md) - 模块索引
- [多租户架构](../multi-tenant-architecture.md) - 系统架构
- [前端技术栈](../frontend-tech-stack.md) - 前端 API
- [文档索引](../README.md) - 所有文档

## 维护

- **更新频率**: API 变更后及时更新
- **更新方法**: 手动维护或使用脚本生成
- **代码示例**: 确保示例代码可运行
