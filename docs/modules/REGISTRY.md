# 模块注册表

> 基于现有代码生成 | 最后更新: 2026-02-10
> 运行 `./.claude/scripts/update-registry.sh` 更新

## 说明

本注册表记录项目中所有模块的信息，防止重复造轮子。

**使用方法**:
1. 开发前先搜索: `./.claude/scripts/search-feature.sh "功能关键词"`
2. 查看本注册表了解现有模块
3. 复用现有功能而非重新实现

## 核心模块

### client
- **路径**: `internal/client/`
- **功能**: Claude API 客户端，处理请求构造、预热、SSE 流式响应
- **主要文件**:
  - `client.go` - 核心客户端实现
  - `config.go` - 配置管理
  - `http.go` - HTTP 请求处理
  - `sse.go` - SSE 流式响应处理
- **主要 API**:
  - `NewClient()` - 创建客户端
  - `SendMessage()` - 发送消息
  - `StreamMessage()` - 流式发送消息
- **测试**: ✅ 有测试（client_test.go, integration_test.go）
- **覆盖率**: 96.7%
- **文档**: [Claude API 客户端](../frontend-tech-stack.md)

### proxy
- **路径**: `internal/proxy/`
- **功能**: 反向代理服务器，多租户管理、节点管理、健康检查、熔断器
- **主要文件**:
  - `handler.go` - 请求处理（流式/非流式超时策略）
  - `reverse_proxy.go` - 反向代理核心（SSE 流修复、空闲超时注入）
  - `idle_timeout.go` - SSE 流空闲超时检测（v1.11.0 新增）
  - `retry.go` - 重试配置（含 StreamIdleTimeout）
  - `node_manager.go` - 节点管理
  - `health.go` - 健康检查
  - `circuit_breaker.go` - 熔断器
  - `account_manager.go` - 账号管理
  - `api_*.go` - 管理 API
- **主要 API**:
  - `NewProxy()` - 创建代理
  - `HandleRequest()` - 处理请求
  - `SelectNode()` - 选择节点
  - `CheckHealth()` - 健康检查
  - `newIdleTimeoutReader()` - 创建 SSE 流空闲超时 Reader
- **测试**: ✅ 有测试（含 idle_timeout_test.go）
- **文档**:
  - [多租户架构](../multi-tenant-architecture.md)
  - [健康检查机制](../health_check_mechanism.md)
  - [监控数据持久化](../monitoring-data-persistence.md)

### store
- **路径**: `internal/store/`
- **功能**: 数据持久化层，MySQL/SQLite 存储，账号、节点、配置、指标
- **主要文件**:
  - `account.go` - 账号存储
  - `node.go` - 节点存储
  - `config.go` - 配置存储
  - `metrics.go` - 指标存储
  - `health_check.go` - 健康检查记录
  - `migration.go` - 数据库迁移
  - `monitor_share.go` - 监控分享
  - `notification.go` - 通知存储
  - `session.go` - 会话存储
  - `settings.go` - 设置存储
  - `usage.go` - 使用量统计
- **主要 API**:
  - `NewStore()` - 创建存储
  - `GetAccount()` - 获取账号
  - `UpsertNode()` - 更新节点
  - `RecordMetrics()` - 记录指标
- **测试**: ✅ 有测试
- **文档**: [数据持久化](../multi-tenant-architecture.md#数据模型)

### notify
- **路径**: `internal/notify/`
- **功能**: 通知系统，节点故障和恢复通知
- **主要文件**:
  - `manager.go` - 通知管理器
  - `channel.go` - 通知渠道
  - `types.go` - 类型定义
  - `store.go` - 通知存储
  - `wechat.go` - 微信通知
  - `migration.go` - 数据库迁移
- **主要 API**:
  - `NewManager()` - 创建管理器
  - `SendNotification()` - 发送通知
  - `GetNotifications()` - 获取通知列表
- **测试**: ✅ 有测试
- **文档**: [通知系统](../notification-system.md)

### tunnel
- **路径**: `internal/tunnel/`
- **功能**: Cloudflare Tunnel 集成，内网穿透
- **主要文件**:
  - `manager.go` - 隧道管理器
  - `cloudflare.go` - Cloudflare API
  - `types.go` - 类型定义
- **主要 API**:
  - `NewManager()` - 创建管理器
  - `CreateTunnel()` - 创建隧道
  - `DeleteTunnel()` - 删除隧道
- **测试**: ✅ 有测试
- **文档**: [Cloudflare Tunnel 集成](../cloudflare-tunnel.md)

### timeutil
- **路径**: `internal/timeutil/`
- **功能**: 时间工具，统一北京时间格式化
- **主要文件**:
  - `format.go` - 时间格式化
- **主要 API**:
  - `FormatBeijingTime()` - 格式化为北京时间
  - `ParseBeijingTime()` - 解析北京时间
- **测试**: ✅ 有测试
- **文档**: [时间统一](../time-unification-summary.md)

### version
- **路径**: `internal/version/`
- **功能**: 版本信息管理
- **主要文件**:
  - `version.go` - 版本信息
- **主要 API**:
  - `GetVersion()` - 获取版本信息
  - `GetBuildInfo()` - 获取构建信息
- **测试**: ✅ 有测试
- **文档**: [版本管理](../notification-system.md#版本系统)

## 前端模块

### frontend
- **路径**: `frontend/`
- **功能**: React Web 管理界面
- **技术栈**: React 18 + TypeScript + Vite + Chart.js
- **主要页面**:
  - `Dashboard.tsx` - 仪表盘
  - `Accounts.tsx` - 账号管理
  - `Nodes.tsx` - 节点管理
  - `Monitor.tsx` - 监控大屏
  - `Usage.tsx` - 使用量统计
  - `Notifications.tsx` - 通知管理
  - `SystemSettings.tsx` - 系统设置
  - `ClaudeConfig.tsx` - Claude 配置
- **文档**: [前端技术栈](../frontend-tech-stack.md)

## CLI 工具

### cccli
- **路径**: `cccli/`, `cmd/cccli/`
- **功能**: Claude Code CLI 工具，系统 prompt 和工具定义
- **主要文件**:
  - `main.go` - 程序入口
  - `system0_cli.txt` - 系统 prompt
  - `system1_cli.txt` - 工具定义
  - `assets.go` - 资源嵌入
- **文档**: [CLI 健康检查](../cli_health_check_implementation.md)

## 测试覆盖率

| 模块 | 当前覆盖率 | 目标覆盖率 | 优先级 |
|------|-----------|-----------|--------|
| client | 18.5% | 100% | 高 |
| proxy | 未知 | 100% | 高 |
| store | 0% | 100% | 高 |
| notify | 0% | 80% | 中 |
| tunnel | 0% | 80% | 中 |
| timeutil | 未知 | 100% | 低 |
| version | 未知 | 100% | 低 |

## 相关文档

- [文档索引](../README.md) - 所有文档导航
- [API 索引](../api/INDEX.md) - API 参考
- [多租户架构](../multi-tenant-architecture.md) - 系统架构
- [健康检查机制](../health_check_mechanism.md) - 健康检查
- [前端技术栈](../frontend-tech-stack.md) - 前端开发

## 维护

- **更新频率**: 代码变更后及时更新
- **更新方法**: 运行 `./.claude/scripts/update-registry.sh`
- **手动维护**: 补充功能描述和 API 信息
