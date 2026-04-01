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
- 可选存储：MySQL（设置 `PROXY_MYSQL_DSN`）
- 默认账号策略：自动创建管理员账号；普通默认账号不再自动创建

## 代码与文档入口

- 项目主页：`README.md`
- 文档索引：`docs/README.md`
- API 索引：`docs/api/INDEX.md`
- 模块注册表：`docs/modules/REGISTRY.md`
- 前端说明：`frontend/README.md`

## 自动化资源

### Skills

- `qcc-dev`：编码规范与实现建议
- `qcc-git`：Git 流程与提交规范
- `qcc-release`：版本发布
- `qcc-debug`：问题排查
- `qcc-deploy`：部署相关
- `codex`：Codex CLI 集成

### Agents

- `.claude/agents/test-agent.md`
- `.claude/agents/doc-agent.md`

### Scripts

- `./.claude/scripts/search-feature.sh "关键词"`
- `./.claude/scripts/update-registry.sh`
- `./.claude/scripts/maintain.sh`
- `./scripts/build-frontend.sh`

## 核心规则

| 规则 | 说明 |
|------|------|
| 开发分支 | 默认在 `test` 分支开发 |
| 节点权重 | 数值越小优先级越高 |
| 时间格式 | 使用 `timeutil.FormatBeijingTime()` |
| 前端颜色 | 不写硬编码颜色，使用 CSS 变量 |
| 前端构建 | 使用 `scripts/build-frontend.sh` 同步到 `web/dist` |

## 当前实现口径

- 管理端前端路由定义在 `frontend/src/App.tsx`
- 后端路由注册在 `internal/proxy/handler.go`
- 默认健康检查方式是 `cli`
- 全量健康检查主变量是 `PROXY_HEALTH_CHECK_ALL_INTERVAL`
- 请求日志页面是 `RequestLogs.tsx`，使用量统计页面是 `Usage.tsx`

## 开发前检查

1. 先搜索现有实现：`./.claude/scripts/search-feature.sh "关键词"`
2. 阅读 `docs/modules/REGISTRY.md`
3. 阅读 `docs/claude/lessons-learned.md`
4. 如改动接口或结构，检查 `docs/api/INDEX.md` 与 `docs/README.md`

## 验证建议

- 编译服务端：`go build ./cmd/cccli`
- 编译前端：`bash scripts/build-frontend.sh`
- 后端测试：`go test ./...`
- 前端静态检查：`cd frontend && npm run build`

## 踩坑记录

遇到新的实现差异、部署问题或文档漂移时，记录到 `docs/claude/lessons-learned.md`。
