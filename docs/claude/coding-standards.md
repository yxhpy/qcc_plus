# 编码规范

本文档定义项目的编码规范和最佳实践。

> **定位**：本文档是编码规范的详细说明，主文件见 @CLAUDE.md

## Go 语言通用规范

- 遵循 Go 官方代码风格（gofmt）
- 使用有意义的变量和函数命名
- 函数保持单一职责，避免过长
- 正确处理错误，不要忽略 error 返回值
- 使用 context 进行超时和取消控制
- 避免全局变量，使用依赖注入

## 项目特异规则

| 规则 | 说明 |
|------|------|
| Builder 模式 | 代理服务器使用 Builder 模式构建，参考 `internal/proxy/` |
| 环境变量配置 | 所有配置通过环境变量注入，参考 `.env.example` |
| MySQL 持久化 | 节点配置持久化到 MySQL，设置 `PROXY_MYSQL_DSN` 启用 |
| SSE 流处理 | SSE 流读取逻辑在 `internal/client/` 中实现 |
| 请求指纹复刻 | 保持与官方 CLI 一致的请求头和参数 |
| 节点权重与切换 | 权重值越小优先级越高（1 > 2 > 3）；事件驱动切换 |
| 时间格式统一 | 使用 `timeutil.FormatBeijingTime()`，格式 `2006年01月02日 15时04分05秒` |
| UI 高信息密度 | 单行紧凑显示；字体 12-14px；padding/gap 6-10px |

## 错误处理

- 所有外部调用（HTTP、数据库）必须有超时控制
- 错误信息要有足够上下文，便于排查
- 使用 `errors.Wrap/Wrapf` 包装底层错误
- 在边界层（HTTP handler）统一处理和记录错误

## 前端规范

- **禁止硬编码颜色**：所有颜色必须使用 `index.css` 中定义的 CSS 变量（如 `var(--bg)`、`var(--text)`、`var(--border)`、`var(--primary)` 等），确保深色/浅色主题兼容
- 组件样式优先使用内联 `style` 或 CSS Modules
- 遵循高信息密度原则：字体 12-14px，padding/gap 6-10px

## 安全规范

- API Token 等敏感信息通过环境变量注入，禁止硬编码
- 所有外部输入必须验证和过滤
- 使用 HTTPS 进行外部通信
- 日志中禁止输出敏感信息（token、密码）
- 数据库查询使用参数化，防止 SQL 注入

## 相关文档

- @CLAUDE.md - 主记忆文件
- @docs/claude/git-workflow.md - Git 工作流（含质量保证）
- @docs/claude/debug-playbook.md - 调试排查手册
