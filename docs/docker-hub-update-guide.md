# Docker Hub 信息更新指南

本文档说明如何完善 Docker Hub 仓库页面的信息展示。

## 仓库信息

- **仓库名称**: yxhpy520/qcc_plus
- **仓库地址**: https://hub.docker.com/r/yxhpy520/qcc_plus
- **GitHub 地址**: https://github.com/yxhpy/qcc_plus

## 需要更新的内容

### 1. Short Description（简短描述）

在 Docker Hub 仓库页面的顶部显示，限制 100 字符以内。

**推荐文案**：
```
功能完整的 Claude Code CLI 多租户代理服务器，支持多节点管理、自动故障切换和 React Web 管理界面
```

**英文版**（如需要）：
```
Full-featured Claude Code CLI proxy server with multi-tenancy, node management, and React web UI
```

### 2. Full Description（完整描述）

使用项目根目录下的 `README.dockerhub.md` 内容。

该文件包含：
- ✨ 核心特性列表
- 🚀 快速开始指南
- 🔧 完整的环境变量配置说明
- 📖 详细的使用示例
- 🎯 版本新特性说明
- 🔒 安全最佳实践
- 🐛 故障排查指南
- 📚 文档资源链接

## 更新步骤

### 方式一：通过 Docker Hub Web 界面（推荐）

1. **登录 Docker Hub**
   - 访问：https://hub.docker.com
   - 使用账号 `yxhpy520` 登录

2. **进入仓库设置**
   - 访问：https://hub.docker.com/r/yxhpy520/qcc_plus
   - 点击 "Manage Repository"

3. **更新 Short Description**
   - 在仓库主页，找到 "Short Description" 编辑框
   - 粘贴上面提供的简短描述
   - 点击 "Update"

4. **更新 Full Description**
   - 点击 "Description" 标签页
   - 选择 "Edit" 模式
   - 将 `README.dockerhub.md` 的完整内容粘贴进去
   - 支持 Markdown 格式
   - 点击 "Update" 保存

5. **设置 Overview 标签**
   - 点击 "Overview" 标签页
   - 确认信息显示正确
   - 可以添加标签（Tags）：
     - `claude`
     - `claude-code`
     - `proxy`
     - `multi-tenant`
     - `nodejs`
     - `react`
     - `go`

### 方式二：通过 Docker Hub API

如果需要通过 API 自动化更新：

```bash
# 设置环境变量
export DOCKERHUB_USERNAME=yxhpy520
export DOCKERHUB_TOKEN=your_access_token
export REPO_NAME=qcc_plus

# 登录获取 JWT Token
TOKEN=$(curl -s -H "Content-Type: application/json" -X POST \
  -d "{\"username\": \"$DOCKERHUB_USERNAME\", \"password\": \"$DOCKERHUB_TOKEN\"}" \
  https://hub.docker.com/v2/users/login/ | jq -r .token)

# 更新 Short Description
curl -X PATCH \
  -H "Authorization: JWT $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"description\": \"功能完整的 Claude Code CLI 多租户代理服务器，支持多节点管理、自动故障切换和 React Web 管理界面\"}" \
  "https://hub.docker.com/v2/repositories/$DOCKERHUB_USERNAME/$REPO_NAME/"

# 更新 Full Description
FULL_DESC=$(cat README.dockerhub.md | jq -Rs .)
curl -X PATCH \
  -H "Authorization: JWT $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"full_description\": $FULL_DESC}" \
  "https://hub.docker.com/v2/repositories/$DOCKERHUB_USERNAME/$REPO_NAME/"
```

### 方式三：连接 GitHub 自动同步

Docker Hub 支持从 GitHub 自动同步 README：

1. **在 Docker Hub 设置 GitHub 连接**
   - 访问：https://hub.docker.com/r/yxhpy520/qcc_plus/settings/general
   - 找到 "Repository Links" 部分
   - 点击 "Link to GitHub"
   - 授权并选择 `yxhpy/qcc_plus` 仓库

2. **配置自动构建（可选）**
   - 访问：https://hub.docker.com/r/yxhpy520/qcc_plus/builds
   - 配置自动构建规则（当 GitHub 有新 tag 时自动构建）

3. **使用 GitHub README**
   - 如果选择自动同步，需要将 `README.dockerhub.md` 重命名为 `README.md`
   - 或者在 GitHub 仓库设置中指定使用 `README.dockerhub.md`

## 推荐标签（Tags）

在 Docker Hub 仓库页面添加以下标签，帮助用户发现项目：

- `claude`
- `claude-code`
- `claude-api`
- `proxy`
- `reverse-proxy`
- `multi-tenant`
- `multi-tenancy`
- `nodejs`
- `react`
- `golang`
- `docker`
- `api-gateway`
- `load-balancer`

## 更新后验证

更新完成后，访问以下页面验证：

1. **仓库主页**
   - https://hub.docker.com/r/yxhpy520/qcc_plus
   - 检查 Short Description 是否显示正确

2. **Description 标签页**
   - https://hub.docker.com/r/yxhpy520/qcc_plus
   - 点击 "Description" 标签
   - 检查完整文档是否正确显示
   - 检查 Markdown 格式是否正确渲染

3. **Tags 标签页**
   - https://hub.docker.com/r/yxhpy520/qcc_plus/tags
   - 确认 `latest` 和当前发布标签存在
   - 检查镜像大小和更新时间

## 维护建议

每次发布新版本时：

1. 更新 `README.dockerhub.md` 中的版本号和新特性
2. 通过上述方式同步到 Docker Hub
3. 确保 Short Description 保持最新
4. 添加新的相关标签（如果有新功能）

## 附录：文案模板

### Short Description 备选方案

方案 1（当前推荐）：
```
功能完整的 Claude Code CLI 多租户代理服务器，支持多节点管理、自动故障切换和 React Web 管理界面
```

方案 2（强调技术栈）：
```
Go + React 构建的 Claude Code CLI 代理服务器，支持多租户、健康检查和 Web 管理界面
```

方案 3（强调特性）：
```
Claude Code CLI 代理 | 多租户隔离 | 智能故障切换 | 三种健康检查 | React 管理界面
```

### 推广文案（社交媒体）

Twitter/X:
```
🚀 QCC Plus 新版本发布！

功能完整的 Claude Code CLI 代理服务器：
✅ 多租户账号隔离
✅ 三种健康检查方式
✅ React Web 管理界面
✅ 一键 Docker 部署

Docker Hub: https://hub.docker.com/r/yxhpy520/qcc_plus
GitHub: https://github.com/yxhpy/qcc_plus

#Claude #Docker #Golang #React
```

Reddit:
```
[Release] QCC Plus - Claude Code CLI Multi-tenant Proxy Server

I've just released a new version of QCC Plus, a full-featured Claude Code CLI proxy server.

Key Features:
- Multi-tenant account isolation
- Three health check methods (API/HEAD/CLI)
- React web management interface
- One-click Docker deployment
- MySQL persistence
- Cloudflare Tunnel integration

Docker Hub: https://hub.docker.com/r/yxhpy520/qcc_plus
GitHub: https://github.com/yxhpy/qcc_plus

Happy to answer any questions!
```

## 相关文件

- `README.dockerhub.md` - Docker Hub 完整描述文档
- `README.md` - GitHub 主文档
- `CHANGELOG.md` - 版本更新日志
- `docs/docker-hub-publish.md` - Docker 发布流程文档

## 下次更新清单

每次发布新版本时的检查清单：

- [ ] 更新 `README.md` 版本号和徽章
- [ ] 更新 `README.dockerhub.md` 版本号和新特性
- [ ] 更新 `CHANGELOG.md` 版本条目
- [ ] 更新 `CLAUDE.md` 记忆文件
- [ ] 创建 GitHub Release
- [ ] 发布 Docker 镜像到 Docker Hub
- [ ] 更新 Docker Hub Short Description
- [ ] 更新 Docker Hub Full Description
- [ ] 验证所有链接和文档正确性
- [ ] 在社交媒体发布更新公告（可选）
