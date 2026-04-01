# 模块注册表

最后校准：2026-04-02

本文档记录 qcc_plus 当前主要模块、职责边界和关键文件，方便开发前快速定位现有实现，避免重复造轮子。

## 使用建议

1. 先运行 `./.claude/scripts/search-feature.sh "关键词"` 搜索现有实现。
2. 再查看本文档确认模块边界和落点。
3. 涉及公开接口或行为变化时，同步更新 [API 索引](../api/INDEX.md) 和相关专题文档。

## Go 后端模块

### `internal/client/`

- 职责：Claude 客户端兼容逻辑、请求构造、流式响应处理、CLI 运行入口辅助。
- 关键文件：
  - `client.go`
  - `config.go`
  - `http.go`
  - `stream.go`
  - `system.go`
- 测试：覆盖较完整，包含单测和集成测试。
- 相关文档：
  - [API 索引](../api/INDEX.md)

### `internal/proxy/`

- 职责：HTTP 入口、`/v1/messages` 代理、多租户账号路由、节点管理、重试、健康检查、监控、WebSocket、Tunnel API、通知 API、定价与使用量 API。
- 关键文件：
  - `handler.go`
  - `builder.go`
  - `server.go`
  - `health.go`
  - `health_scheduler.go`
  - `handler.go`
  - `api_*.go`
- 子域能力：
  - 认证与会话
  - 账号与节点管理
  - 负载均衡与慢节点降级
  - 指标聚合、请求日志、模型恢复
  - 监控分享与 WebSocket
- 相关文档：
  - [多租户架构](../multi-tenant-architecture.md)
  - [健康检查机制](../health_check_mechanism.md)
  - [监控数据持久化](../monitoring-data-persistence.md)
  - [API 索引](../api/INDEX.md)

### `internal/store/`

- 职责：MySQL / SQLite 持久化层。
- 涵盖数据：
  - 账号、节点、配置
  - 健康检查历史、指标、监控分享
  - 通知渠道与订阅
  - Tunnel 配置
  - 模型定价、使用量、请求尝试日志
  - Session 与系统设置
- 关键文件：
  - `mysql.go`
  - `sqlite.go`
  - `account.go`
  - `node.go`
  - `metrics.go`
  - `notification.go`
  - `pricing.go`
  - `session.go`
  - `settings.go`
- 说明：
  - 默认使用 SQLite。
  - 设置 `PROXY_MYSQL_DSN` 后切换到 MySQL。

### `internal/notify/`

- 职责：通知渠道、订阅规则、事件投递、测试发送。
- 关键文件：
  - `manager.go`
  - `channel.go`
  - `wechat.go`
  - `store.go`
  - `migration.go`
- 相关文档：
  - [通知系统](../notification-system.md)

### `internal/tunnel/`

- 职责：Cloudflare Tunnel 配置、启动、停止、域名映射。
- 关键文件：
  - `manager.go`
  - `cloudflare.go`
  - `types.go`
- 相关文档：
  - [Cloudflare Tunnel](../cloudflare-tunnel.md)

### `internal/timeutil/`

- 职责：统一北京时间格式化与解析。
- 关键文件：
  - `format.go`
- 约束：
  - 文档与界面涉及时间展示时，优先使用统一工具函数。

### `internal/version/`

- 职责：构建信息、版本号、提交号、构建时间输出。
- 关键文件：
  - `version.go`

## 前端模块

### `frontend/`

- 技术栈：React 19、TypeScript、Vite、Chart.js、React Router。
- 关键目录：
  - `src/pages/`
  - `src/components/`
  - `src/contexts/`
  - `src/hooks/`
  - `src/services/`
  - `src/themes/`
- 当前页面：
  - `Login`
  - `Dashboard`
  - `Accounts`
  - `Nodes`
  - `Monitor`
  - `MonitorShares`
  - `SharedMonitor`
  - `Settings`
  - `SystemSettings`
  - `TunnelSettings`
  - `Notifications`
  - `ClaudeConfig`
  - `Pricing`
  - `Usage`
  - `RequestLogs`
  - `ModelRecovery`
  - `ChangelogPage`
- 相关文档：
  - [前端技术栈](../frontend-tech-stack.md)
  - [前端 README](../../frontend/README.md)

### `web/`

- 职责：Go embed 前端产物目录。
- 关键文件：
  - `embed.go`
  - `dist/`
- 说明：
  - 必须通过 `scripts/build-frontend.sh` 将 `frontend/dist` 复制到 `web/dist`。

## CLI 与分发

### `cmd/cccli/`

- 职责：
  - `proxy` 子命令启动服务端
  - 其他参数走客户端模式
- 关键文件：
  - `main.go`

### `cccli/`

- 职责：CLI 相关嵌入资源。
- 关键文件：
  - `assets.go`
  - `system0_cli.txt`
  - `system1_cli.txt`
  - `tools_cli.json`

### `npm-packages/`

- 职责：`@qccplus/cli` 及多平台二进制 npm 包分发。
- 关键目录：
  - `@qccplus/cli/`
  - `@qccplus/darwin-arm64/`
  - `@qccplus/darwin-x64/`
  - `@qccplus/linux-arm64/`
  - `@qccplus/linux-x64/`
  - `@qccplus/win32-x64/`

## 脚本与自动化

### `scripts/`

- `build-frontend.sh`：构建前端并同步到 `web/dist`
- `build-npm-packages.sh`：构建 npm 分发包
- `deploy-test-local.sh`：本地测试部署
- `publish-docker.sh`：发布 Docker 镜像
- `update-dockerhub-info*.sh`：更新 Docker Hub 描述

### `.claude/`

- `agents/`：项目内专用代理定义
- `skills/`：项目内技能
- `scripts/search-feature.sh`：搜索现有功能
- `scripts/update-registry.sh`：更新模块注册表骨架
- `scripts/maintain.sh`：项目维护脚本

## 相关文档

- [API 索引](../api/INDEX.md)
- [文档索引](../README.md)
- [多租户架构](../multi-tenant-architecture.md)
- [前端技术栈](../frontend-tech-stack.md)
