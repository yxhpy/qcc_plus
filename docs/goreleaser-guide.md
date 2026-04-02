# GoReleaser 自动化发布指南

## 概述

本项目已集成 GoReleaser，实现完全自动化的版本发布流程。使用 GoReleaser 后，发布新版本只需要创建并推送一个 Git tag，所有其他步骤都会自动完成。

## 功能特性

GoReleaser 为本项目提供以下自动化能力：

- ✅ **自动构建**: 跨平台 Go 二进制文件（Linux、macOS、Windows，支持 amd64 和 arm64）
- ✅ **版本注入**: 自动将版本号、Git commit、构建日期注入到二进制文件
- ✅ **Docker 构建**: 自动构建和推送多架构 Docker 镜像（amd64、arm64）
- ✅ **GitHub Release**: 自动创建 GitHub Release 并上传构建产物
- ✅ **CHANGELOG 生成**: 根据 commit message 自动生成分类的 CHANGELOG
- ✅ **Docker Hub 更新**: 自动更新 Docker Hub 仓库信息

## 快速开始

### 前置要求

1. **本地开发环境**（可选，仅本地测试需要）：
   ```bash
   # macOS
   brew install goreleaser

   # Linux
   # 参考: https://goreleaser.com/install/
   ```

2. **GitHub Secrets 配置**（必需，用于 CI/CD）：
   在 GitHub 仓库设置中添加以下 Secrets：
   - `DOCKER_USERNAME`: Docker Hub 用户名（例如：yxhpy520）
   - `DOCKER_TOKEN`: Docker Hub Personal Access Token

### 发布新版本（完全自动化）

**旧的手动流程**（已淘汰）：
```bash
# ❌ 不再需要这些手动步骤
1. 手动更新 CHANGELOG.md
2. 手动更新 CLAUDE.md
3. git tag vX.Y.Z && git push origin vX.Y.Z
4. gh release create vX.Y.Z --title "..." --notes "..."
5. ./scripts/publish-docker.sh yxhpy520 vX.Y.Z
6. 手动更新 Docker Hub 仓库信息
```

**新的自动化流程**（推荐）：
```bash
# ✅ 只需要这一步！
git tag v1.2.0
git push origin v1.2.0

# 完成！GoReleaser 会自动执行：
# 1. 构建多平台二进制文件
# 2. 构建和推送 Docker 镜像（amd64 + arm64）
# 3. 生成 CHANGELOG
# 4. 创建 GitHub Release
# 5. 上传所有构建产物
# 6. 更新 Docker Hub 仓库信息
```

### 版本号规范

遵循语义化版本控制（Semantic Versioning）：

- **v1.0.0** → **v1.0.1**: Bug 修复（patch）
- **v1.0.0** → **v1.1.0**: 新功能（minor）
- **v1.0.0** → **v2.0.0**: 重大变更（major）

## Commit Message 规范

为了自动生成高质量的 CHANGELOG，请遵循 Conventional Commits 规范：

### 格式
```
<type>: <description>

[optional body]

[optional footer]
```

### 类型（Type）

| 类型 | 说明 | 版本影响 | CHANGELOG 分类 |
|------|------|----------|----------------|
| `feat` | 新功能 | minor | 🚀 新功能 |
| `fix` | Bug 修复 | patch | 🐛 Bug 修复 |
| `docs` | 文档更新 | - | 📝 文档更新 |
| `refactor` | 代码重构 | - | 🔨 重构 |
| `test` | 测试相关 | - | 🧪 测试 |
| `chore` | 构建/工具 | - | 不包含在 CHANGELOG |
| `ci` | CI/CD 配置 | - | 不包含在 CHANGELOG |

### 示例

```bash
# 新功能（会出现在 CHANGELOG 的"🚀 新功能"部分）
git commit -m "feat: 添加健康检查 API 端点"
git commit -m "feat(proxy): 支持自定义重试策略"

# Bug 修复（会出现在"🐛 Bug 修复"部分）
git commit -m "fix: 修复 Docker 容器健康检查超时"
git commit -m "fix(client): 处理 SSE 流中断异常"

# 文档更新（会出现在"📝 文档更新"部分）
git commit -m "docs: 更新 GoReleaser 使用说明"

# 重大变更（会触发 major 版本升级，使用 ! 标记）
git commit -m "feat!: 重构 API 接口，移除 v1 兼容性"
git commit -m "fix!: 修改配置文件格式"

# 不会出现在 CHANGELOG 的提交
git commit -m "chore: 更新依赖版本"
git commit -m "ci: 优化 GitHub Actions 配置"
```

## 本地测试

在推送 tag 之前，可以在本地测试 GoReleaser 配置：

```bash
# 1. 检查配置文件是否有效
goreleaser check

# 2. 构建快照（不会发布，仅用于测试）
goreleaser build --snapshot --clean

# 3. 测试完整发布流程（不会真正发布）
goreleaser release --snapshot --clean --skip=publish

# 4. 查看构建产物
ls -lh dist/
```

## GitHub Actions 工作流

GoReleaser 通过 GitHub Actions 运行（`.github/workflows/release.yml`），默认仍由版本 tag 自动触发，同时支持 `workflow_dispatch` 手动补触发：

```yaml
# 触发条件：推送以 v 开头的 tag，或手动指定已存在的版本 tag
on:
  push:
    tags:
      - 'v*.*.*'
  workflow_dispatch:
    inputs:
      ref:
        description: 已存在的版本 tag（例如 v1.2.0、v1.2.0-rc.1）
        required: true

# 主要步骤：
1. Checkout 指定 ref（包含完整 git 历史）
2. 拉取全部 tags 并校验当前 ref 对应已有版本 tag
3. 设置 Go 环境
4. 设置 Docker Buildx（多架构构建）
5. 登录 Docker Hub
6. 运行 GoReleaser
7. 更新 Docker Hub 仓库信息
```

## 构建产物

每次发布会生成以下产物：

### 1. Go 二进制文件
- `qcc_plus_v1.2.0_linux_x86_64.tar.gz`
- `qcc_plus_v1.2.0_linux_arm64.tar.gz`
- `qcc_plus_v1.2.0_darwin_x86_64.tar.gz`
- `qcc_plus_v1.2.0_darwin_arm64.tar.gz`
- `qcc_plus_v1.2.0_windows_x86_64.zip`

### 2. Docker 镜像
- `yxhpy520/qcc_plus:v1.2.0` (multi-arch manifest)
- `yxhpy520/qcc_plus:latest` (multi-arch manifest)
- `yxhpy520/qcc_plus:v1.2.0-amd64`
- `yxhpy520/qcc_plus:v1.2.0-arm64`

### 3. 其他文件
- `checksums.txt`: 所有文件的校验和
- 自动生成的 CHANGELOG

## 配置文件说明

### .goreleaser.yml

主配置文件，定义了构建、打包、发布的所有行为。关键配置：

```yaml
# 构建配置
builds:
  - main: ./cmd/cccli
    binary: ccproxy
    ldflags:
      - -X 'qcc_plus/internal/version.Version={{.Version}}'
      - -X 'qcc_plus/internal/version.GitCommit={{.ShortCommit}}'
      - -X 'qcc_plus/internal/version.BuildDate={{.Date}}'

# Docker 配置
dockers:
  - dockerfile: Dockerfile
    image_templates:
      - "yxhpy520/{{.ProjectName}}:{{.Version}}-amd64"
      - "yxhpy520/{{.ProjectName}}:latest-amd64"

# CHANGELOG 配置
changelog:
  groups:
    - title: '🚀 新功能'
      regexp: "^.*feat[(\\w)]*:+.*$"
    - title: '🐛 Bug 修复'
      regexp: "^.*fix[(\\w)]*:+.*$"
```

## 故障排查

### 问题 1: Docker 推送失败

**错误**: `denied: requested access to the resource is denied`

**解决方案**:
```bash
# 检查 GitHub Secrets 是否正确配置
# DOCKER_USERNAME 和 DOCKER_TOKEN 必须正确

# 验证本地 Docker Hub 登录
docker login
```

### 问题 2: CHANGELOG 为空或格式不正确

**原因**: Commit message 不符合 Conventional Commits 规范

**解决方案**:
```bash
# 确保 commit message 遵循格式
git commit -m "feat: 添加新功能"  # ✅ 正确
git commit -m "添加新功能"        # ❌ 错误，缺少 type

# 检查现有提交
git log --oneline
```

### 问题 3: 版本信息未注入

**原因**: ldflags 配置错误或包路径不正确

**解决方案**:
```bash
# 检查 internal/version/version.go 是否存在
cat internal/version/version.go

# 本地测试版本注入
go build -ldflags "-X 'qcc_plus/internal/version.Version=test'" ./cmd/cccli
./cccli version
```

## 迁移指南

### 从旧的手动流程迁移

1. **删除或归档旧脚本**（可选）:
   ```bash
   # 旧的发布脚本仍然可用，但建议使用 GoReleaser
   # scripts/publish-docker.sh 已被 GoReleaser 替代
   ```

2. **配置 GitHub Secrets**:
   - 添加 `DOCKER_USERNAME`
   - 添加 `DOCKER_TOKEN`

3. **更新工作流程**:
   - 不再需要手动运行 `gh release create`
   - 不再需要手动运行 `./scripts/publish-docker.sh`
   - 只需创建并推送 tag

## 高级用法

### 自定义 Release Notes

如果想要手动编辑 Release Notes：

```bash
# 1. 使用 GoReleaser 生成草稿
goreleaser release --draft

# 2. 在 GitHub 上编辑 Draft Release
# 3. 手动发布
```

### 发布预发布版本

```bash
# 创建预发布 tag（会自动标记为 pre-release）
git tag v1.2.0-beta.1
git push origin v1.2.0-beta.1
```

### 跳过某些步骤

```yaml
# 在 .goreleaser.yml 中配置
release:
  disable: true  # 跳过 GitHub Release

dockers:
  - skip_push: true  # 跳过 Docker 推送
```

## 最佳实践

1. **使用语义化版本号**: 严格遵循 vX.Y.Z 格式
2. **遵循 Commit 规范**: 确保 CHANGELOG 自动生成质量
3. **本地测试**: 推送 tag 前先运行 `goreleaser build --snapshot`
4. **保持 CHANGELOG.md 更新**: GoReleaser 生成的 changelog 可以手动补充到 CHANGELOG.md
5. **定期检查 GitHub Actions**: 确保发布流程正常运行

## 相关资源

- [GoReleaser 官方文档](https://goreleaser.com/)
- [Conventional Commits 规范](https://www.conventionalcommits.org/)
- [语义化版本控制](https://semver.org/lang/zh-CN/)
- [GitHub Actions 文档](https://docs.github.com/en/actions)

## 总结

使用 GoReleaser 后，发布流程从 **10+ 分钟的多步手动操作** 简化为 **一条命令的全自动流程**：

```bash
# 以前（10+ 分钟，6+ 个步骤）
更新文档 → 创建 tag → 创建 Release → 构建 Docker → 推送镜像 → 更新 Docker Hub

# 现在（2 分钟，1 个步骤）
git tag v1.2.0 && git push origin v1.2.0
```

享受自动化的快乐吧！🚀
