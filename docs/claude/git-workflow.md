# Git 工作流规范

本文档定义项目的 Git 分支策略和版本控制规范。

> **定位**：本文档是 Git 工作流的详细说明，主文件见 @CLAUDE.md

## 分支策略（强制）

**强制规则**：所有开发工作必须在 `test` 分支进行，编写代码前必须确认当前分支。

| 分支 | 用途 | 说明 |
|------|------|------|
| `test` | 日常开发 | ✅ 在这里开发，推送后自动部署到测试环境（端口 8001） |
| `main` | 正式发布 | 合并测试通过的代码，用于打 tag 发布版本 |
| `prod` | 生产候选分支 | 作为生产部署默认 ref，需通过 GitHub Actions 手动触发部署到生产服务器（端口 8000） |

## 工作流程

```bash
# 1. 开发
git checkout test
# 编写代码
git push origin test

# 2. 发布
git checkout main
git merge test
git tag vX.Y.Z
git push origin vX.Y.Z

# 3. 更新生产候选分支
git checkout prod
git merge main
git push origin prod  # 仅更新候选分支，不自动部署

# 4. 生产部署
# GitHub Actions -> Deploy Prod -> Run workflow
# ref 填 prod 或具体 tag（推荐版本 tag）
```

## 编写代码前检查清单

```bash
# 确认在 test 分支
git branch --show-current

# 如不在 test 分支
git checkout test
```

## Commit 格式

使用 Conventional Commits 格式：`type: description`

| 类型 | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档更新 |
| `refactor` | 代码重构 |
| `test` | 测试相关 |
| `chore` | 构建/工具 |

示例：
```
feat: 添加健康检查 API 端点
fix: 修复节点切换延迟问题
docs: 更新 CLAUDE.md 文档结构
```

## 质量保证

### 测试要求

- 核心业务逻辑必须有单元测试
- 使用真实数据测试，避免过度 mock
- 测试边界条件和错误场景
- 使用 `go test -race` 检测竞态条件

### 代码审查

- 所有合并到 main 的代码必须经过审查
- 检查错误处理是否完善
- 检查是否有资源泄漏（goroutine、文件句柄）
- 检查并发安全性

## 相关文档

- @CLAUDE.md - 主记忆文件
- @docs/claude/release-policy.md - 版本发布规范
- @docs/claude/coding-standards.md - 编码规范
