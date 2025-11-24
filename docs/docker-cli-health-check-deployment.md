# CLI 健康检查 Docker 部署指南

## 概述
qcc_plus 支持三种健康检查方式，其中 CLI 方式需要 Docker 环境支持。本文档说明如何在 Docker 部署环境中启用 CLI 健康检查。

## 架构说明

### 方案：挂载宿主机 Docker Socket
- **原理**：将宿主机的 `/var/run/docker.sock` 挂载到容器内
- **优点**：简单、高效、资源消耗低
- **缺点**：需要宿主机有 Docker（生产环境通常都有）

### 组件
1. **qcc_plus 容器**：包含 Docker CLI 客户端
2. **宿主机 Docker Daemon**：通过 socket 提供服务
3. **claude-code-cli-verify 镜像**：自动构建于容器启动时

## 部署步骤

### 1. 使用 docker-compose 部署（推荐）

```bash
# 克隆仓库
git clone https://github.com/yxhpy/qcc_plus.git
cd qcc_plus

# 编辑配置
cp .env.example .env
# 修改 UPSTREAM_API_KEY 等配置

# 启动服务
docker-compose up -d

# 查看日志（确认 CLI 镜像构建成功）
docker-compose logs proxy
```

**预期输出**：
```
=== qcc_plus Docker Entrypoint ===
✓ Docker socket detected at /var/run/docker.sock
✓ Docker CLI available
✓ Docker daemon accessible
⚠ Claude CLI verify image not found, building...
✓ Claude CLI verify image built successfully
=== Starting ccproxy ===
```

### 2. 使用 docker run 部署

```bash
# 构建镜像
docker build -t qcc_plus:latest .

# 运行容器（注意挂载 Docker socket）
docker run -d \
  --name qcc_plus \
  -p 8000:8000 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e UPSTREAM_BASE_URL=https://api.anthropic.com \
  -e UPSTREAM_API_KEY=your-api-key \
  qcc_plus:latest
```

### 3. 使用 Docker Hub 镜像

```bash
docker pull yxhpy520/qcc_plus:latest

docker run -d \
  --name qcc_plus \
  -p 8000:8000 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e UPSTREAM_BASE_URL=https://api.anthropic.com \
  -e UPSTREAM_API_KEY=your-api-key \
  yxhpy520/qcc_plus:latest
```

## 配置说明

### docker-compose.yml 关键配置

```yaml
services:
  proxy:
    build: .
    volumes:
      # 挂载宿主机 Docker socket，支持 CLI 健康检查
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      # ... 其他配置
```

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `UPSTREAM_BASE_URL` | 上游 API 地址 | https://api.anthropic.com |
| `UPSTREAM_API_KEY` | 上游 API Key | - |
| `PROXY_HEALTH_INTERVAL_SEC` | 健康检查间隔（秒） | 30 |

## 验证部署

### 1. 检查容器状态

```bash
docker ps | grep qcc_plus
```

### 2. 检查日志

```bash
docker logs qcc_plus
```

应该看到：
- ✓ Docker socket detected
- ✓ Docker CLI available
- ✓ Claude CLI verify image built successfully

### 3. 检查 CLI 镜像

```bash
docker exec qcc_plus docker images | grep claude-code-cli-verify
```

应该看到：
```
claude-code-cli-verify   latest   ...   ...   ...
```

### 4. 测试 CLI 健康检查

1. 登录管理界面：http://localhost:8000/admin
2. 创建一个节点，选择健康检查方式为 **"Claude Code CLI (Docker)"**
3. 等待健康检查执行（默认 30 秒）
4. 查看节点详情，`last_ping_error` 应该为空（成功）或显示具体错误

## 故障排查

### 问题 1：容器启动失败

**症状**：
```
ERROR: failed to bind to /var/run/docker.sock
```

**原因**：Docker socket 权限问题

**解决方案**：
```bash
# 检查 socket 权限
ls -l /var/run/docker.sock

# 如果需要，修改权限
sudo chmod 666 /var/run/docker.sock
```

### 问题 2：CLI 健康检查失败

**症状**：节点详情显示 `executable file not found in $PATH`

**检查步骤**：
1. 进入容器
   ```bash
   docker exec -it qcc_plus bash
   ```

2. 检查 Docker CLI
   ```bash
   which docker
   docker version
   ```

3. 检查 socket 挂载
   ```bash
   ls -l /var/run/docker.sock
   ```

4. 检查镜像
   ```bash
   docker images | grep claude-code-cli-verify
   ```

### 问题 3：CLI 镜像构建失败

**症状**：启动日志显示 `Failed to build Claude CLI verify image`

**原因**：
- 网络问题（无法下载 Node.js 镜像）
- Docker daemon 不可用

**解决方案**：
1. 手动构建镜像
   ```bash
   cd verify/claude_code_cli
   docker build -f Dockerfile.verify_pass -t claude-code-cli-verify .
   ```

2. 然后重启 qcc_plus 容器
   ```bash
   docker restart qcc_plus
   ```

### 问题 4：容器内 Docker 不可用

**症状**：`Cannot connect to the Docker daemon`

**检查**：
```bash
# 在宿主机上检查 Docker daemon
sudo systemctl status docker

# 重启 Docker daemon
sudo systemctl restart docker
```

## 安全注意事项

### Docker Socket 挂载风险

⚠️ **警告**：挂载 Docker socket 相当于给容器完全的宿主机 Docker 控制权。

**风险**：
- 容器可以启动任意容器
- 可以挂载宿主机任意路径
- 可以执行特权操作

**缓解措施**：
1. 仅在受信任的环境中使用
2. 使用防火墙限制容器网络访问
3. 定期审查容器日志
4. 考虑使用 Docker-in-Docker (DinD) 替代方案（更复杂）

### 生产环境建议

1. **使用专用 Docker 用户**
   ```bash
   sudo groupadd docker
   sudo usermod -aG docker qcc_plus
   ```

2. **限制容器能力**
   ```yaml
   security_opt:
     - no-new-privileges:true
   ```

3. **使用只读挂载（如果可能）**
   ```yaml
   volumes:
     - /var/run/docker.sock:/var/run/docker.sock:ro
   ```
   注意：只读模式下无法构建镜像，需要预先构建 CLI 镜像

## 替代方案

### 方案 A：使用 API 健康检查（推荐）

如果不想挂载 Docker socket，可以使用 API 健康检查：

1. 创建节点时选择 **"API 调用 (/v1/messages)"**
2. 不需要 Docker 支持
3. 验证完整的 API 调用链路

### 方案 B：使用 HEAD 健康检查

最轻量的方式：

1. 创建节点时选择 **"HEAD 请求"**
2. 不需要 Docker，不需要 API Key
3. 仅验证连通性

## 更新日志

- **2025-11-24**：初始版本
  - 支持挂载 Docker socket
  - 自动构建 CLI 镜像
  - 完整的部署说明

## 相关文档

- [健康检查机制](health_check_mechanism.md)
- [CLI 健康检查实现](cli_health_check_implementation.md)
- [Docker Hub 发布流程](docker-hub-publish.md)
