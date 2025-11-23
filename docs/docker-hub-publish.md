# Docker Hub 发布指南

本文档说明如何将 qcc_plus 镜像发布到 Docker Hub。

## 前置要求

1. 已安装 Docker
2. 拥有 Docker Hub 账号
3. 已克隆项目代码

## 发布步骤

### 1. 登录 Docker Hub

```bash
docker login
```

输入你的 Docker Hub 用户名和密码。

### 2. 使用发布脚本

项目提供了自动化发布脚本 `scripts/publish-docker.sh`：

```bash
# 用法: ./scripts/publish-docker.sh <dockerhub-username> <version>
./scripts/publish-docker.sh yourusername v1.0.0
```

脚本会自动完成以下操作：
1. 检查 Docker Hub 登录状态
2. 构建 Docker 镜像
3. 打上版本标签和 latest 标签
4. 推送到 Docker Hub

### 3. 手动发布（可选）

如果需要手动控制发布过程：

```bash
# 1. 构建镜像
docker build -t yourusername/qcc_plus:v1.0.0 .

# 2. 添加 latest 标签
docker tag yourusername/qcc_plus:v1.0.0 yourusername/qcc_plus:latest

# 3. 推送到 Docker Hub
docker push yourusername/qcc_plus:v1.0.0
docker push yourusername/qcc_plus:latest
```

## 镜像验证

### 本地测试

发布前先在本地测试镜像：

```bash
# 构建测试镜像
docker build -t qcc_plus:test .

# 运行测试容器
docker run -d \
  -p 8000:8000 \
  -e UPSTREAM_BASE_URL=https://api.anthropic.com \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  -e ADMIN_API_KEY=test-admin-key \
  qcc_plus:test

# 测试登录页可访问
curl -I http://localhost:8000/login

# 停止并删除测试容器
docker stop $(docker ps -q --filter ancestor=qcc_plus:test)
docker rm $(docker ps -aq --filter ancestor=qcc_plus:test)
```

### 拉取验证

发布后验证镜像可以正常拉取：

```bash
# 拉取镜像
docker pull yourusername/qcc_plus:latest

# 查看镜像信息
docker images | grep qcc_plus

# 运行容器
docker run -d -p 8000:8000 \
  -e UPSTREAM_BASE_URL=https://api.anthropic.com \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  yourusername/qcc_plus:latest
```

## 版本规范

建议使用语义化版本号：
- `v1.0.0` - 首个正式版本

每次发布同时打上：
- 具体版本标签（如 `v1.0.0`）
- `latest` 标签（指向最新稳定版本）

## 镜像大小优化

当前 Dockerfile 已采用多阶段构建：
- 构建阶段：使用 `golang:1.21`
- 运行阶段：使用 `gcr.io/distroless/base-debian12`（最小化镜像）

这可以显著减小最终镜像大小。

## 故障排查

### 构建失败

1. 检查网络连接（需要下载依赖）
2. 确认 Go 模块配置正确
3. 查看构建日志中的错误信息

### 推送失败

1. 确认已登录 Docker Hub：`docker info`
2. 检查网络连接
3. 确认镜像名称格式：`username/repository:tag`

### 镜像无法运行

1. 检查环境变量是否正确配置
2. 查看容器日志：`docker logs <container_id>`
3. 确认端口映射正确

## Docker Hub 仓库设置

在 Docker Hub 网站上：

1. 创建仓库
   - 仓库名称：`qcc_plus`
   - 可见性：Public（公开）或 Private（私有）

2. 添加描述
   - 简短描述：Claude Code CLI 多租户代理服务器
   - 详细说明：可以复制 README.md 内容

3. 添加标签
   - `claude-code`
   - `proxy`
   - `multi-tenant`
   - `golang`

## 更新 README

发布后，建议在项目 README.md 中添加 Docker Hub 链接：

```markdown
## Docker Hub

镜像已发布到 Docker Hub：

- Repository: [yourusername/qcc_plus](https://hub.docker.com/r/yourusername/qcc_plus)
- Latest: `docker pull yourusername/qcc_plus:latest`
```

## 自动化 CI/CD（可选）

可以使用 GitHub Actions 自动化发布流程。创建 `.github/workflows/docker-publish.yml`：

```yaml
name: Docker Publish

on:
  push:
    tags:
      - 'v*'

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v4
        with:
          push: true
          tags: |
            ${{ secrets.DOCKERHUB_USERNAME }}/qcc_plus:${{ github.ref_name }}
            ${{ secrets.DOCKERHUB_USERNAME }}/qcc_plus:latest
```

## 注意事项

1. **不要**将敏感信息（API Keys、密码等）写入镜像
2. 所有敏感配置通过环境变量注入
3. 发布前确保代码已提交到 Git
4. 测试镜像功能正常后再推送
5. 保持 latest 标签指向最新稳定版本
