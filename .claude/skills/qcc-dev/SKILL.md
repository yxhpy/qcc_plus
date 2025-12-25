---
name: qcc-dev
description: Use for coding standards, project conventions, and development best practices in qcc_plus project
---

# QCC Plus 开发规范

## 项目概述
- **项目名称**: qcc_plus
- **技术栈**: Go 1.21 + MySQL + React 18 + TypeScript + Vite
- **核心功能**: Claude Code CLI 代理服务器，多租户账号隔离，自动故障切换

## Go 语言规范

### 通用规则
- 遵循 Go 官方代码风格（gofmt）
- 使用有意义的变量和函数命名
- 函数保持单一职责，避免过长
- 正确处理错误，不要忽略 error 返回值
- 使用 context 进行超时和取消控制
- 避免全局变量，使用依赖注入

### 项目特异规则

| 规则 | 说明 |
|------|------|
| Builder 模式 | 代理服务器使用 Builder 模式构建，参考 `internal/proxy/` |
| 环境变量配置 | 所有配置通过环境变量注入，参考 `.env.example` |
| MySQL 持久化 | 节点配置持久化到 MySQL，设置 `PROXY_MYSQL_DSN` 启用 |
| SSE 流处理 | SSE 流读取逻辑在 `internal/client/` 中实现 |
| 请求指纹复刻 | 保持与官方 CLI 一致的请求头和参数 |
| 节点权重与切换 | 权重值越小优先级越高（1 > 2 > 3）；事件驱动切换 |
| 时间格式统一 | 使用 `timeutil.FormatBeijingTime()`，格式 `2006年01月02日 15时04分05秒` |

### 错误处理
- 所有外部调用（HTTP、数据库）必须有超时控制
- 错误信息要有足够上下文，便于排查
- 使用 `errors.Wrap/Wrapf` 包装底层错误
- 在边界层（HTTP handler）统一处理和记录错误

## 前端规范

### 禁止硬编码颜色
**重要**: 所有颜色必须使用 `index.css` 中定义的 CSS 变量：
- `var(--bg)` - 背景色
- `var(--text)` - 文本色
- `var(--border)` - 边框色
- `var(--primary)` - 主色调
- `var(--success)` - 成功状态
- `var(--danger)` - 危险状态

确保深色/浅色主题兼容。

### UI 高信息密度
- 单行紧凑显示
- 字体 12-14px
- padding/gap 6-10px
- 组件样式优先使用内联 `style` 或 CSS Modules

## 安全规范
- API Token 等敏感信息通过环境变量注入，禁止硬编码
- 所有外部输入必须验证和过滤
- 使用 HTTPS 进行外部通信
- 日志中禁止输出敏感信息（token、密码）
- 数据库查询使用参数化，防止 SQL 注入

## 避免过度工程

### 不要做的事
- 不添加未请求的功能或"改进"
- 不为单次操作创建抽象
- 不为假设的未来需求设计
- 不添加未更改代码的注释/文档
- 不添加不可能发生场景的错误处理

### 正确做法
- 只做直接请求或明显必要的更改
- 保持解决方案简单聚焦
- 三行相似代码胜过过早抽象
- 如果未使用就完全删除
