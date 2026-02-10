# 项目记忆文件

## 元信息
- **当前版本**: v1.11.0
- **GitHub**: https://github.com/yxhpy/qcc_plus
- **Docker Hub**: https://hub.docker.com/r/yxhpy520/qcc_plus
- **npm**: https://www.npmjs.com/package/@qccplus/cli

## 项目概述

**qcc_plus** - Claude Code CLI 代理服务器

- **技术栈**: Go 1.21 + MySQL + React 18 + TypeScript + Vite
- **核心功能**: 多租户账号隔离、自动故障切换、React SPA 管理界面

## Skills 索引

详细知识已转换为 Skills，Claude 会根据任务自动调用：

| Skill | 说明 | 触发场景 |
|-------|------|----------|
| `qcc-dev` | 编码规范、Go/前端开发规范 | 编写代码、代码审查 |
| `qcc-git` | Git 分支策略、Commit 规范 | Git 操作、提交代码 |
| `qcc-release` | 版本发布、GoReleaser | 发布新版本 |
| `qcc-debug` | 调试排查、问题诊断 | 遇到错误、排查问题 |
| `qcc-deploy` | 部署操作、服务器连接 | 部署、查看日志 |
| `codex` | Codex CLI 集成 | 代码分析、重构 |

## 快速启动

```bash
# npm 安装（推荐）
npm install -g @qccplus/cli
qccplus start

# 或 Docker 部署
docker compose up -d
```

**默认凭证（仅内存模式）**: `admin`/`admin123`，管理界面 http://localhost:8000/admin

## 核心规则速查

| 规则 | 说明 |
|------|------|
| **开发分支** | 必须在 `test` 分支开发 |
| **发布** | `git tag vX.Y.Z && git push origin vX.Y.Z` |
| **节点权重** | 值越小优先级越高（1 > 2 > 3） |
| **时间格式** | `timeutil.FormatBeijingTime()` |
| **前端颜色** | 禁止硬编码，使用 CSS 变量 |

## 📝 测试要求

- **当前覆盖率**: client 96.7%, notify 96.2%, proxy 65.0%, store 72.7%, timeutil 93.3%, tunnel 92.9%, version 100%
- **目标覆盖率**: 100%
- **测试框架**: go test
- **运行测试**: `go test -v -cover ./...`
- **覆盖率报告**: `go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out`
- **部署方式**: 本地 Docker 部署（`docker compose up -d --build`，不部署到远程服务器）

## 🚀 开发工作流

1. 输入 **"开始"** 启动开发
2. AI 自动读取 TODO.md
3. 搜索现有功能（防止重复）
4. 执行任务
5. 提交代码并本地部署
6. 更新文档

**注意**: 修改完成后不执行测试，直接提交和部署。

## 踩坑记录

遇到问题时立即记录到 @docs/claude/lessons-learned.md

### 前端部署经验

1. **前端构建流程**: 前端源码在 `frontend/`，构建产物需要复制到 `web/dist/`（Go embed 嵌入目录）。必须运行 `bash scripts/build-frontend.sh` 而不是直接 `npm run build`，否则 Docker 镜像中不会包含最新前端。
2. **Go embed 机制**: `web/embed.go` 通过 `//go:embed dist/*` 将 `web/dist/` 嵌入到 Go 二进制中。前端文件不会单独出现在容器文件系统中，而是编译进了二进制。
3. **避免重复写入**: 同一个数据写入操作不要在多个地方调用（如 `metrics.go` 和 `handler.go` 都写 usage log 导致重复记录）。统一到一个入口。
4. **页面对应关系**: 请求日志页面是 `RequestLogs.tsx`（`/admin/request-logs`），不是 `Usage.tsx`（`/admin/usage`）。修改功能前先确认用户实际使用的是哪个页面。
5. **`.dockerignore`**: `frontend/dist` 被排除但 `web/dist` 没有被排除，这是正确的，因为 Go embed 需要 `web/dist`。
6. **流式请求缓冲陷阱**: `retryBufferWriter` 的 `Flush()` 在未 flush 时是 no-op，会导致 SSE 流式数据被无限缓冲，客户端卡住。流式请求必须设置 `SetStreaming(true)`，确保数据到达即透传。

## 🔍 开发前必读

### 检查清单

1. **搜索现有功能**：
   ```bash
   ./.claude/scripts/search-feature.sh "功能关键词"
   ```

2. **查看模块注册表**：
   - 阅读 `docs/modules/REGISTRY.md` ⭐
   - 了解现有模块和功能

3. **查看已知问题**：
   - 阅读 `docs/claude/lessons-learned.md`
   - 避免重复踩坑

4. **启动开发**：
   - 输入 **"开始"** 让 AI 自动执行 TODO.md 中的任务

## 🤖 自动化能力

### Skills（自动触发）
- **qcc-dev**: 编码规范、Go/前端开发规范
- **qcc-git**: Git 分支策略、Commit 规范
- **qcc-release**: 版本发布、GoReleaser
- **qcc-debug**: 调试排查、问题诊断
- **qcc-deploy**: 部署操作、服务器连接
- **codex**: Codex CLI 集成

### Agents（专用代理）
- **test-agent**: 测试相关任务（`.claude/agents/test-agent.md`）
- **doc-agent**: 文档相关任务（`.claude/agents/doc-agent.md`）

### Scripts（辅助工具）
- `search-feature.sh`: 搜索现有功能
- `update-registry.sh`: 更新模块注册表
- `maintain.sh`: 项目维护

## 📚 文档导航

### 核心文档
- @README.md - 项目主页
- @CHANGELOG.md - 版本历史
- @TODO.md - 任务列表 ⭐
- @docs/README.md - 完整文档索引

### 模块和 API
- @docs/modules/REGISTRY.md - 模块注册表 ⭐
- @docs/api/INDEX.md - API 索引

### 详细文档
- @docs/claude/lessons-learned.md - 踩坑记录
- @docs/claude/deployment-private.md - 🔒 私有部署配置
