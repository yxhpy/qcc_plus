# 通知系统文档

## 概述

QCC Plus 支持为每个用户配置独立的通知渠道（如企业微信机器人），用户可以自定义想要接收的通知事件类型。

## 功能特性

- **多租户支持**：每个账号可以独立配置自己的通知渠道
- **多渠道扩展**：支持企业微信，架构支持后续扩展（邮件、钉钉、Slack 等）
- **事件订阅**：用户可多选想要接收的通知类型
- **异步发送**：通知不阻塞主业务流程
- **去重限流**：默认 5 分钟内相同事件只通知一次

## 支持的事件类型

### 节点相关 (node.*)

| 事件类型 | 说明 |
|---------|------|
| `node.status_changed` | 节点状态变化 |
| `node.switched` | 节点自动切换 |
| `node.failed` | 节点标记为失败 |
| `node.recovered` | 节点从失败恢复 |
| `node.added` | 节点添加 |
| `node.deleted` | 节点删除 |
| `node.updated` | 节点更新 |
| `node.enabled` | 节点启用 |
| `node.disabled` | 节点禁用 |
| `node.health_check_failed` | 节点探活失败 |

### 请求相关 (request.*)

| 事件类型 | 说明 |
|---------|------|
| `request.failed` | 请求失败（重试耗尽） |
| `request.upstream_error` | 上游返回错误状态码 |
| `request.proxy_error` | 代理错误 |

### 账号相关 (account.*)

| 事件类型 | 说明 |
|---------|------|
| `account.quota_warning` | 配额使用告警（预留） |
| `account.auth_failed` | 认证失败（预留） |

### 系统相关 (system.*)

| 事件类型 | 说明 |
|---------|------|
| `system.tunnel_started` | 隧道启动 |
| `system.tunnel_stopped` | 隧道停止 |
| `system.tunnel_error` | 隧道错误 |
| `system.error` | 系统错误 |

## API 接口

### 通知渠道管理

#### 列出渠道
```http
GET /api/notification/channels
```

响应：
```json
[
  {
    "id": "ch-xxx",
    "name": "我的企业微信",
    "channel_type": "wechat_work",
    "enabled": true,
    "created_at": "2025-11-24T00:00:00Z"
  }
]
```

#### 创建渠道
```http
POST /api/notification/channels
Content-Type: application/json

{
  "name": "我的企业微信机器人",
  "channel_type": "wechat_work",
  "config": {
    "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
  },
  "enabled": true
}
```

#### 更新渠道
```http
PUT /api/notification/channels/:id
Content-Type: application/json

{
  "name": "新名称",
  "enabled": false
}
```

#### 删除渠道
```http
DELETE /api/notification/channels/:id
```

### 通知订阅管理

#### 列出订阅
```http
GET /api/notification/subscriptions
GET /api/notification/subscriptions?channel_id=ch-xxx
```

响应：
```json
[
  {
    "id": "sub-xxx",
    "channel_id": "ch-xxx",
    "event_type": "node.failed",
    "enabled": true,
    "created_at": "2025-11-24T00:00:00Z"
  }
]
```

#### 创建订阅（批量）
```http
POST /api/notification/subscriptions
Content-Type: application/json

{
  "channel_id": "ch-xxx",
  "event_types": ["node.failed", "node.recovered", "node.switched"],
  "enabled": true
}
```

#### 更新订阅
```http
PUT /api/notification/subscriptions/:id
Content-Type: application/json

{
  "enabled": false
}
```

#### 删除订阅
```http
DELETE /api/notification/subscriptions/:id
```

### 其他接口

#### 获取事件类型列表
```http
GET /api/notification/event-types
```

响应：
```json
[
  {
    "type": "node.failed",
    "category": "node",
    "description": "节点标记为失败"
  }
]
```

#### 测试通知
```http
POST /api/notification/test
Content-Type: application/json

{
  "channel_id": "ch-xxx",
  "title": "测试通知",
  "content": "这是一条测试消息"
}
```

## 企业微信机器人配置

### 1. 创建机器人

1. 在企业微信群聊中，点击右上角 `...` → `群机器人` → `添加`
2. 输入机器人名称，创建成功后会获得 Webhook 地址
3. Webhook 地址格式：`https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`

### 2. 在 QCC Plus 中配置

```bash
# 创建渠道
curl -X POST http://localhost:8000/api/notification/channels \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=xxx" \
  -d '{
    "name": "运维告警群",
    "channel_type": "wechat_work",
    "config": {"webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"},
    "enabled": true
  }'

# 订阅事件
curl -X POST http://localhost:8000/api/notification/subscriptions \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=xxx" \
  -d '{
    "channel_id": "ch-xxx",
    "event_types": ["node.failed", "node.recovered", "node.switched"],
    "enabled": true
  }'

# 测试通知
curl -X POST http://localhost:8000/api/notification/test \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=xxx" \
  -d '{
    "channel_id": "ch-xxx",
    "title": "测试通知",
    "content": "QCC Plus 通知系统配置成功！"
  }'
```

## 通知消息格式

企业微信通知使用 Markdown 格式：

```markdown
**节点故障告警**
> 事件类型：node.failed
> 时间：2025-11-24 08:00:00

**节点名称**: api-node-1
**错误信息**: connection timeout
**失败次数**: 3
```

## 数据库表结构

### notification_channels（通知渠道）

| 字段 | 类型 | 说明 |
|-----|------|-----|
| id | VARCHAR(64) | 主键 |
| account_id | VARCHAR(64) | 所属账号 |
| channel_type | VARCHAR(32) | 渠道类型 |
| name | VARCHAR(128) | 渠道名称 |
| config | JSON | 配置（webhook_url 等） |
| enabled | BOOLEAN | 是否启用 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### notification_subscriptions（通知订阅）

| 字段 | 类型 | 说明 |
|-----|------|-----|
| id | VARCHAR(64) | 主键 |
| account_id | VARCHAR(64) | 所属账号 |
| channel_id | VARCHAR(64) | 渠道 ID |
| event_type | VARCHAR(64) | 事件类型 |
| enabled | BOOLEAN | 是否启用 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### notification_history（通知历史）

| 字段 | 类型 | 说明 |
|-----|------|-----|
| id | VARCHAR(64) | 主键 |
| account_id | VARCHAR(64) | 账号 ID |
| channel_id | VARCHAR(64) | 渠道 ID |
| event_type | VARCHAR(64) | 事件类型 |
| title | VARCHAR(256) | 通知标题 |
| content | TEXT | 通知内容 |
| status | VARCHAR(16) | 状态（sent/failed） |
| error | TEXT | 错误信息 |
| sent_at | DATETIME | 发送时间 |
| created_at | DATETIME | 创建时间 |

## 架构说明

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  业务逻辑   │────▶│  通知管理器  │────▶│  通知渠道   │
│ (节点/请求) │     │  (异步队列)  │     │ (微信/邮件) │
└─────────────┘     └──────────────┘     └─────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  订阅检查    │
                    │  去重限流    │
                    └──────────────┘
```

## 扩展新渠道

要添加新的通知渠道（如钉钉、Slack），需要：

1. 在 `internal/notify/types.go` 添加渠道类型常量
2. 创建新文件（如 `dingtalk.go`）实现 `NotificationChannel` 接口
3. 在 `internal/notify/channel.go` 的 `BuildChannel` 函数中注册新渠道

```go
// 实现 NotificationChannel 接口
type NotificationChannel interface {
    Send(ctx context.Context, msg NotificationMessage) error
}
```

## 注意事项

1. **安全性**：Webhook URL 等敏感信息不会在 API 响应中返回
2. **去重**：默认 5 分钟内相同事件只通知一次，可通过配置调整
3. **限流**：队列满时会丢弃新事件并记录日志
4. **持久化**：通知功能依赖存储层，默认 SQLite 即可使用；多实例或集中部署可切换到 MySQL（`PROXY_MYSQL_DSN`）
