# 项目记忆文件

## 元信息

- 当前版本：`v1.9.4`
- GitHub：https://github.com/yxhpy/qcc_plus
- Docker Hub：https://hub.docker.com/r/yxhpy520/qcc_plus
- npm：https://www.npmjs.com/package/@qccplus/cli

## 项目概述

qcc_plus 是一个面向 Claude Code CLI 的多租户代理服务。

- 技术栈：Go 1.21+、SQLite / MySQL、React 19、TypeScript、Vite
- 默认存储：SQLite（`~/.qccplus/qccplus.db`）
- 可选持久化：MySQL（设置 `PROXY_MYSQL_DSN`）
- 默认账号策略：自动创建管理员账号 `admin/admin123`；普通默认账号不再自动创建

## 文档入口

- `README.md`
- `docs/README.md`
- `docs/api/INDEX.md`
- `docs/modules/REGISTRY.md`
- `frontend/README.md`

## 项目内自动化

### Skills

- `qcc-dev`
- `qcc-git`
- `qcc-release`
- `qcc-debug`
- `qcc-deploy`
- `codex`

### Agents

- `.claude/agents/test-agent.md`
- `.claude/agents/doc-agent.md`

### Scripts

- `./.claude/scripts/search-feature.sh`
- `./.claude/scripts/update-registry.sh`
- `./.claude/scripts/maintain.sh`

## 核心规则

| 规则 | 说明 |
|------|------|
| 开发分支 | 默认在 `test` 分支开发 |
| 节点权重 | 数值越小优先级越高 |
| 时间格式 | 使用 `timeutil.FormatBeijingTime()` |
| 前端样式 | 禁止硬编码颜色，统一使用 CSS 变量 |
| 前端构建 | 通过 `scripts/build-frontend.sh` 生成并复制到 `web/dist` |

## 当前事实源

- 后端入口：`cmd/cccli/main.go`
- 服务端路由：`internal/proxy/handler.go`
- 健康检查配置：`internal/proxy/envvars.go`
- 前端路由：`frontend/src/App.tsx`
- 版本信息：`internal/version/version.go` 与 `CHANGELOG.md`

## 开发前建议

1. 搜索现有实现：`./.claude/scripts/search-feature.sh "关键词"`
2. 查看模块注册表：`docs/modules/REGISTRY.md`
3. 查看踩坑记录：`docs/claude/lessons-learned.md`
4. 如变更接口、路由、命令或默认值，同步更新相关文档

## 验证建议

- `go build ./cmd/cccli`
- `bash scripts/build-frontend.sh`
- `go test ./...`
- `cd frontend && npm run build`

## 维护要求

- 文档必须以代码为准。
- 发现文档漂移时，优先修正 `README.md`、`docs/README.md`、`docs/api/INDEX.md`、`docs/modules/REGISTRY.md`。
- 新的坑点记录到 `docs/claude/lessons-learned.md`。
