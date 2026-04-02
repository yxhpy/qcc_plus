# 发布流程最佳实践

## 概述

本文档定义了 qcc_plus 项目从开发到正式发布的完整流程，确保每个发布版本都经过充分测试和验证。

---

## 🎯 发布策略

采用**分支 + Tag 混合策略**，分为三个阶段：

```
开发测试 (test) → 预发布验证 (beta/rc) → 正式发布 (release)
```

---

## 📋 完整发布流程

### 阶段 1: 开发和测试环境验证

**目标**: 在测试环境验证新功能

**流程**:
```bash
# 1. 开发新功能
git checkout test
# ... 进行开发 ...

# 2. 提交代码
git add .
git commit -m "feat: 添加新功能"

# 3. 推送到 test 分支
git push origin test
```

**自动化行为**:
- ✅ GitHub Actions (`deploy-test.yml`) 自动触发
- ✅ 部署到测试服务器 (端口 8001)
- ✅ Docker 镜像仅在测试服务器本地构建（**不推送到 Docker Hub**）
- ✅ 自动健康检查

**验证步骤**:
```bash
# 访问测试环境
curl http://your-test-server:8001/

# 功能测试
# ... 执行测试用例 ...
```

**特点**:
- 🔒 **隔离性**: 测试环境独立，不影响生产
- 🚫 **不公开**: Docker 镜像不推送到公共仓库
- ⚡ **快速迭代**: 推送代码后 1-2 分钟部署完成

---

### 阶段 2: Pre-release 公开测试（可选）

**目标**: 发布 beta/rc 版本供用户提前测试

**适用场景**:
- 重大功能更新
- 架构重构
- 需要社区反馈的功能

**流程**:
```bash
# 1. 确保 test 分支测试通过
# 2. 创建 beta 或 rc tag
git tag v1.3.0-beta.1
# 或
git tag v1.3.0-rc.1

# 3. 推送 tag
git push origin v1.3.0-beta.1
```

**自动化行为**:
- ✅ GoReleaser 自动触发 (`release.yml`)
- ✅ 编译多平台二进制
- ✅ 构建多架构 Docker 镜像
- ✅ **推送到 Docker Hub**（打 `v1.3.0-beta.1` 标签）
- ✅ **不会更新 `latest` 标签**（因为是 pre-release）
- ✅ 创建 GitHub Pre-release（标记为 ⚠️ Pre-release）
- ✅ 自动生成 CHANGELOG

**用户使用**:
```bash
# 用户可以选择性安装测试版本
docker pull yxhpy520/qcc_plus:v1.3.0-beta.1

# 普通用户拉取 latest 不会受影响
docker pull yxhpy520/qcc_plus:latest  # 仍然是上一个稳定版本
```

**验证步骤**:
```bash
# 部署到 staging 环境测试
docker run -d -p 8002:8000 yxhpy520/qcc_plus:v1.3.0-beta.1

# 收集用户反馈
# 修复发现的问题
# 继续迭代 beta.2, beta.3 ...
```

**Pre-release 版本号规范**:
- `v1.3.0-alpha.1` - 内部测试，功能不完整
- `v1.3.0-beta.1` - 公开测试，功能基本完整
- `v1.3.0-rc.1` - Release Candidate，准备发布的候选版本

---

### 阶段 3: 正式发布

**目标**: 发布稳定的生产版本

**前置条件**:
- ✅ 测试环境验证通过
- ✅ （可选）Pre-release 测试通过
- ✅ 所有已知 bug 已修复
- ✅ 文档已更新

**流程**:
```bash
# 1. 确保所有更改已合并到 main/prod
git checkout main  # 或 prod
git merge test
git push origin main

# 2. 更新 CHANGELOG.md（可选，GoReleaser 会自动生成）
# 编辑 CHANGELOG.md，将 [Unreleased] 内容移至新版本

# 3. 提交 CHANGELOG 更新
git add CHANGELOG.md
git commit -m "docs: 准备发布 v1.3.0"
git push origin main

# 4. 创建正式 tag
git tag v1.3.0

# 5. 推送 tag 触发发布
git push origin v1.3.0
```

**自动化行为**:
- ✅ GoReleaser 自动触发
- ✅ 编译多平台二进制（5 个平台）
- ✅ 构建多架构 Docker 镜像（amd64 + arm64）
- ✅ **推送到 Docker Hub**:
  - `yxhpy520/qcc_plus:v1.3.0`
  - `yxhpy520/qcc_plus:latest` ⭐ **更新 latest**
  - `yxhpy520/qcc_plus:v1.3.0-amd64`
  - `yxhpy520/qcc_plus:v1.3.0-arm64`
  - `yxhpy520/qcc_plus:latest-amd64`
  - `yxhpy520/qcc_plus:latest-arm64`
- ✅ 创建 GitHub Release（**正式版本**）
- ✅ 上传所有构建产物（二进制 + checksums）
- ✅ 自动生成并发布 CHANGELOG
- ✅ 更新 Docker Hub 仓库信息

**验证步骤**:
```bash
# 1. 验证 GitHub Release
gh release view v1.3.0

# 2. 验证 Docker 镜像
docker pull yxhpy520/qcc_plus:v1.3.0
docker pull yxhpy520/qcc_plus:latest

# 3. 验证版本信息
docker run --rm yxhpy520/qcc_plus:v1.3.0 --version

# 4. 部署到生产环境
# 方式 1: 推送 prod 分支，触发 deploy-prod.yml
git checkout prod
git merge main
git push origin prod  # 触发 deploy-prod.yml

# 方式 2: 在生产服务器统一走脚本入口
ssh deploy@your-prod-server 'cd /opt/qcc_plus && chmod +x scripts/deploy-server.sh && ./scripts/deploy-server.sh prod'
```

---

## 🔄 版本号规范（语义化版本）

遵循 [Semantic Versioning 2.0.0](https://semver.org/)：

```
vMAJOR.MINOR.PATCH[-PRERELEASE]

示例:
v1.0.0          # 正式版本
v1.0.1          # Bug 修复
v1.1.0          # 新功能（向后兼容）
v2.0.0          # 重大变更（可能不兼容）
v1.3.0-beta.1   # Pre-release
```

**版本号选择指南**:

| 变更类型 | 示例 | 版本号 |
|---------|------|--------|
| Bug 修复 | 修复登录超时问题 | v1.0.0 → v1.0.1 |
| 新增功能（兼容） | 添加健康检查 API | v1.0.0 → v1.1.0 |
| 重大变更（不兼容） | 重构 API 接口 | v1.x.x → v2.0.0 |
| Pre-release | 公开测试版本 | v1.1.0 → v1.1.0-beta.1 |

---

## 📊 流程对比

| 阶段 | 环境 | Docker Hub | GitHub Release | latest 标签 | 公开访问 |
|------|------|-----------|---------------|------------|---------|
| **阶段 1: 测试** | test 服务器 | ❌ 不推送 | ❌ 不创建 | ❌ 不更新 | ❌ 内部 |
| **阶段 2: Pre-release** | staging (可选) | ✅ 推送 beta/rc | ✅ Pre-release | ❌ 不更新 | ⚠️ 选择性 |
| **阶段 3: 正式发布** | production | ✅ 推送 stable | ✅ Release | ✅ **更新** | ✅ 公开 |

---

## 🛡️ 回滚策略

### 场景 1: Pre-release 发现严重问题

```bash
# 删除有问题的 pre-release
gh release delete v1.3.0-beta.1 --yes
git push --delete origin v1.3.0-beta.1
git tag -d v1.3.0-beta.1

# Docker 镜像已推送但 latest 未受影响，不需要回滚
```

### 场景 2: 正式版本发现严重问题

**方案 A: 快速 Hotfix**
```bash
# 1. 在 main 分支修复问题
git checkout main
# ... 修复 bug ...
git commit -m "fix: 紧急修复 XXX 问题"

# 2. 发布 hotfix 版本
git tag v1.3.1
git push origin v1.3.1
```

**方案 B: 回滚 latest 标签**
```bash
# 在 Docker Hub 手动将 latest 指向上一个稳定版本
docker pull yxhpy520/qcc_plus:v1.2.0
docker tag yxhpy520/qcc_plus:v1.2.0 yxhpy520/qcc_plus:latest
docker push yxhpy520/qcc_plus:latest
```

---

## 📝 Commit Message 规范

为了让 GoReleaser 自动生成高质量的 CHANGELOG，请遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```bash
# 格式
<type>: <description>

# 类型
feat:     新功能
fix:      Bug 修复
docs:     文档更新
refactor: 代码重构
test:     测试相关
chore:    构建/工具（不包含在 CHANGELOG）
ci:       CI/CD 配置（不包含在 CHANGELOG）

# 示例
git commit -m "feat: 添加用户认证功能"
git commit -m "fix: 修复健康检查超时问题"
git commit -m "docs: 更新 README 安装说明"
git commit -m "feat!: 重构 API 接口（breaking change）"
```

**CHANGELOG 自动分类**:
- `feat:` → 🚀 新功能
- `fix:` → 🐛 Bug 修复
- `docs:` → 📝 文档更新
- `refactor:` → 🔨 重构
- `test:` → 🧪 测试

---

## 🎯 快速参考

### 日常开发迭代
```bash
git checkout test
# 开发 → 提交 → 推送
git push origin test  # 自动部署到测试环境
```

### 发布 Beta 版本
```bash
git tag v1.x.x-beta.1
git push origin v1.x.x-beta.1  # 自动发布到 Docker Hub (Pre-release)
```

### 发布正式版本
```bash
git tag v1.x.x
git push origin v1.x.x  # 自动发布到 Docker Hub + GitHub Release
```

### 紧急 Hotfix
```bash
git checkout main
# 修复 → 提交
git tag v1.x.(x+1)
git push origin v1.x.(x+1)  # 自动发布
```

---

## ⚙️ GitHub Actions 工作流对应

| 工作流 | 触发条件 | 行为 | Docker Hub | 环境 |
|-------|---------|------|-----------|------|
| `deploy-test.yml` | push test 分支 | 部署到测试服务器 | ❌ 不推送 | test |
| `deploy-prod.yml` | push prod 分支 | 部署到生产服务器 | ❌ 不推送 | prod |
| `release.yml` | push tag `v*.*.*` | GoReleaser 发布 | ✅ 推送 | - |

---

## 📚 相关文档

- [GoReleaser 使用指南](./goreleaser-guide.md)
- [多租户快速开始](./quick-start-multi-tenant.md)
- [CI/CD 故障排查](./ci-cd-troubleshooting.md)
- [语义化版本控制](https://semver.org/lang/zh-CN/)
- [Conventional Commits](https://www.conventionalcommits.org/)

---

## 🎓 最佳实践总结

1. ✅ **测试先行**: 所有更改先在 test 环境验证
2. ✅ **渐进发布**: 重大更新先发布 beta/rc 版本收集反馈
3. ✅ **保护 latest**: 只有正式版本更新 `latest` 标签
4. ✅ **自动化**: 使用 GoReleaser 避免手动错误
5. ✅ **可回滚**: 保留历史版本，支持快速回滚
6. ✅ **语义化版本**: 严格遵循版本号规范
7. ✅ **规范提交**: 使用 Conventional Commits 自动生成 CHANGELOG

---

**生成时间**: 2025-12-08
**适用版本**: v1.9.2+
