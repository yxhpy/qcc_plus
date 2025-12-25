# 项目记忆文件

## 元信息
- **当前版本**: v1.9.4
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

## 踩坑记录

遇到问题时立即记录到 @docs/claude/lessons-learned.md

## 文档索引

### Skills（自动调用）
- `.claude/skills/qcc-dev/SKILL.md` - 编码规范
- `.claude/skills/qcc-git/SKILL.md` - Git 工作流
- `.claude/skills/qcc-release/SKILL.md` - 版本发布
- `.claude/skills/qcc-debug/SKILL.md` - 调试排查
- `.claude/skills/qcc-deploy/SKILL.md` - 部署操作

### 详细文档
- @README.md - 项目主页
- @CHANGELOG.md - 版本历史
- @docs/README.md - 完整文档索引
- @docs/claude/lessons-learned.md - 踩坑记录
- @docs/claude/deployment-private.md - 🔒 私有部署配置
