# 版本发布规范

本文档定义 GitHub Release 和 Docker Hub 发布流程。

> **定位**：本文档是版本发布的详细说明，主文件见 @CLAUDE.md

## 语义化版本

格式：`vX.Y.Z`

| 版本号 | 说明 | 示例 |
|--------|------|------|
| X (主版本) | 不兼容的 API 变更 | v2.0.0 |
| Y (次版本) | 向后兼容的功能新增 | v1.1.0 |
| Z (修订号) | 向后兼容的问题修正 | v1.1.1 |

## GoReleaser 自动化发布（推荐）

本项目已集成 GoReleaser，发布新版本只需：

```bash
git tag v1.2.0
git push origin v1.2.0
```

GitHub Actions 自动完成：
- 构建多平台 Go 二进制（Linux/macOS/Windows，amd64/arm64）
- 注入版本信息（version、git commit、build date）
- 构建并推送 Docker 镜像（amd64 + arm64 multi-arch）
- 生成分类 CHANGELOG
- 创建 GitHub Release 并上传构建产物
- 更新 Docker Hub 仓库信息

`release.yml` 保持 tag 推送自动触发，同时提供 `workflow_dispatch` 作为补触发入口；手动触发时仍应选择已存在的版本 tag。

## 正式环境部署策略

- `test` 分支推送后会自动部署测试环境。
- 正式环境不再由 `push prod` 自动触发部署。
- 正式部署入口统一为 GitHub Actions `deploy-prod.yml` 的 `workflow_dispatch`。
- `deploy-prod.yml` 的 `ref` 支持 branch/tag，正式发布建议直接填写版本 tag，便于追踪和回滚。

## Commit Message 规范

使用 Conventional Commits 格式：`type: description`

| 类型 | 说明 | 版本影响 |
|------|------|----------|
| `feat` | 新功能 | minor 升级 |
| `fix` | Bug 修复 | patch 升级 |
| `feat!` / `fix!` | 重大变更 | major 升级 |
| `docs` | 文档更新 | 不触发 |
| `refactor` | 代码重构 | 不触发 |
| `test` | 测试相关 | 不触发 |
| `chore` | 构建/工具 | 不包含在 CHANGELOG |

示例：
```
feat: 添加健康检查 API 端点
fix: 修复 Docker 容器健康检查超时
feat!: 重构 API 接口，移除 v1 兼容性
```

## GitHub Secrets 配置

| Secret | 说明 |
|--------|------|
| `DOCKER_USERNAME` | Docker Hub 用户名（yxhpy520） |
| `DOCKER_TOKEN` | Docker Hub Personal Access Token |

在 GitHub 仓库设置 → Secrets and variables → Actions 中配置。

## 本地测试

```bash
# 检查配置
goreleaser check

# 构建测试（快照模式）
goreleaser build --snapshot --clean

# 完整发布测试（不推送）
goreleaser release --snapshot --clean --skip=publish
```

## 发布后更新

1. 更新 CLAUDE.md 中的"当前版本"字段
2. 更新 CHANGELOG.md
3. 验证 Docker 镜像：`docker pull yxhpy520/qcc_plus:vX.Y.Z`
4. 验证版本信息：`curl http://localhost:8000/version`
5. 在 GitHub Actions 手动执行 `Deploy Prod`，`ref` 推荐填写本次发布 tag

## 重要提醒

- Docker Hub 用户名是 `yxhpy520`（不是 yxhpy）
- `latest` 标签始终指向最新稳定版本
- 发布前必须确保代码已通过所有测试
- 版本信息通过构建时 ldflags 注入，无需手动修改代码
- 开发流程结束后不会自动发布生产，必须显式执行 `workflow_dispatch`

## 相关文档

- @CLAUDE.md - 主记忆文件
- @docs/claude/git-workflow.md - Git 工作流
- @docs/release-workflow.md - 发布流程详解（手动模式）
- @docs/goreleaser-guide.md - GoReleaser 指南
