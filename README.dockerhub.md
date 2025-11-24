# QCC Plus - Claude Code CLI 多租户代理服务器

[![Version](https://img.shields.io/badge/version-1.1.0-blue.svg)](https://github.com/yxhpy/qcc_plus/releases/tag/v1.1.0)
[![GitHub](https://img.shields.io/badge/GitHub-yxhpy%2Fqcc__plus-181717?logo=github)](https://github.com/yxhpy/qcc_plus)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/yxhpy/qcc_plus/blob/main/LICENSE)

> 功能完整的 Claude Code CLI 代理服务器，支持多租户账号隔离、多节点管理、自动故障切换和 React Web 管理界面。

## ✨ 核心特性

- 🏢 **多租户账号隔离** - 每个账号拥有独立��节点池和配置
- 🔀 **智能路由** - 根据 API Key 自动路由到对应账号的节点
- 🌐 **多节点管理** - 支持配置多个上游节点，权重优先级控制
- 🔄 **智能故障切换** - 事件驱动的节点切换，自动故障转移
- 💚 **三种健康检查** - API/HEAD/CLI 三种健康检查方式，支持自动恢复
- 🎨 **React Web 管理界面** - 现代化 SPA 界面，可视化管理账号和节点
- 💾 **MySQL 持久化** - 配置和统计数据持久化存储
- 🚀 **一键 Docker 部署** - 支持 Docker Compose，开箱即用
- 🌩️ **Cloudflare Tunnel 集成** - 内置隧道支持，无需公网 IP

## 🚀 快速开始

### 使用 Docker Compose（推荐）

```bash
# 1. 下载 docker-compose.yml
curl -O https://raw.githubusercontent.com/yxhpy/qcc_plus/main/docker-compose.yml
curl -O https://raw.githubusercontent.com/yxhpy/qcc_plus/main/.env.example
mv .env.example .env

# 2. 编辑 .env 文件，配置你的 API Key
vim .env  # 修改 UPSTREAM_API_KEY 和其他配置

# 3. 启动服务
docker compose up -d

# 4. 访问管理界面
open http://localhost:8000/admin
```

### 单容器运行

```bash
docker run -d \
  --name qcc_plus \
  -p 8000:8000 \
  -e UPSTREAM_BASE_URL=https://api.anthropic.com \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  yxhpy520/qcc_plus:latest
```

### 访问管理界面

启动后访问：http://localhost:8000/admin

默认登录凭证（仅限内存模式）：
- 管理员：`admin` / `admin123`
- 默认账号：`default` / `default123`

⚠️ **生产环境请务必修改默认密码！**

## 📦 可用标签

- `latest` - 最新稳定版本
- `v1.1.0` - 指定版本
- `v1.0.0` - 首个正式版本

## 🔧 环境变量配置

### 基础配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LISTEN_ADDR` | 监听地址 | `:8000` |
| `UPSTREAM_BASE_URL` | 上游 API 地址 | `https://api.anthropic.com` |
| `UPSTREAM_API_KEY` | 上游 API Key（必填） | - |
| `UPSTREAM_NAME` | 默认节点名称 | `default` |

### 代理配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PROXY_RETRY_MAX` | 请求重试次数 | `3` |
| `PROXY_FAIL_THRESHOLD` | 失败阈值（连续失败多少次标记失败） | `3` |
| `PROXY_HEALTH_INTERVAL_SEC` | 健康检查间隔（秒） | `30` |
| `PROXY_MYSQL_DSN` | MySQL 连接字符串（持久化） | - |

### 安全配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ADMIN_API_KEY` | 管理员密钥（服务端配置） | `admin` ⚠️ |
| `DEFAULT_ACCOUNT_NAME` | 默认账号名称（仅内存模式） | `default` |
| `DEFAULT_PROXY_API_KEY` | 默认代理 API Key（仅内存模式） | `default-proxy-key` ⚠️ |

### Cloudflare Tunnel 配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `CF_API_TOKEN` | Cloudflare API Token | - |
| `TUNNEL_SUBDOMAIN` | 隧道子域名 | - |
| `TUNNEL_ZONE` | Cloudflare Zone（域名） | - |
| `TUNNEL_ENABLED` | 启用隧道功能 | `false` |

## 📖 使用示例

### 基本使用

```bash
# 使用默认账号调用 API
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 1024
  }'
```

### 创建新账号

```bash
# 1. 先登录获取 Cookie
curl -c cookies.txt -X POST \
  -d "username=admin&password=admin123" \
  http://localhost:8000/login

# 2. 创建新账号
curl -b cookies.txt -X POST \
  http://localhost:8000/admin/api/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "team-alpha",
    "proxy_api_key": "alpha-secure-key",
    "is_admin": false
  }'

# 3. 为账号添加节点
curl -b cookies.txt -X POST \
  http://localhost:8000/admin/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "node-1",
    "base_url": "https://api.anthropic.com",
    "api_key": "sk-ant-xxx",
    "weight": 1
  }'
```

### MySQL 持久化部署

```yaml
# docker-compose.yml
version: '3.7'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: qcc_plus
      MYSQL_USER: qcc_user
      MYSQL_PASSWORD: qcc_pass
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3306:3306"

  proxy:
    image: yxhpy520/qcc_plus:latest
    ports:
      - "8000:8000"
    environment:
      - UPSTREAM_BASE_URL=https://api.anthropic.com
      - UPSTREAM_API_KEY=sk-ant-your-key
      - PROXY_MYSQL_DSN=qcc_user:qcc_pass@tcp(mysql:3306)/qcc_plus?parseTime=true
    depends_on:
      - mysql

volumes:
  mysql_data:
```

## 🎯 新版本特性 (v1.1.0)

### CLI 健康检查系统
- ✅ 新增 CLI 健康检查方式（Claude Code CLI 无头模式验证）
- ✅ 支持三种健康检查：API、HEAD、CLI
- ✅ 节点健康信息实时显示（最后检查时间、延迟、错误）
- ✅ 架构简化：容器内直接安装 Claude CLI，移除 Docker-in-Docker

### 版本管理系统
- ✅ `/version` API 接口，返回版本和构建信息
- ✅ 前端侧边栏显示版本号
- ✅ CHANGELOG 支持

### 通知系统
- ✅ 节点故障和恢复的实时通知
- ✅ 通知管理界面（查看、标记已读、删除）

### CI/CD 自动化
- ✅ GitHub Actions 自动部署
- ✅ 健康检查验证部署成功

## 🔒 安全最佳实践

1. **修改默认凭证**
   ```bash
   # 在 .env 或环境变量中设置
   export ADMIN_API_KEY=your-strong-admin-key
   export DEFAULT_PROXY_API_KEY=your-strong-proxy-key
   ```

2. **使用强密码**
   - 登录后立即修改管理员密码
   - 为生产账号设置复杂的 API Key

3. **启用 HTTPS**
   ```bash
   # 使用反向代理（推荐）
   # Nginx/Caddy + Let's Encrypt
   # 或使用 Cloudflare Tunnel
   export TUNNEL_ENABLED=true
   export CF_API_TOKEN=your-cf-token
   ```

4. **限制访问**
   ```bash
   # 绑定到本地接口
   export LISTEN_ADDR=127.0.0.1:8000
   # 或使用防火墙规则
   ```

## 🐛 故障排查

### 容器无法启动
```bash
# 查看日志
docker logs qcc_plus

# 检查环境变量
docker exec qcc_plus env | grep UPSTREAM
```

### 健康检查失败
```bash
# 检查节点状态
curl http://localhost:8000/admin/api/nodes

# 手动触发健康检查
curl -b cookies.txt -X POST \
  http://localhost:8000/admin/api/nodes/{node_id}/health-check
```

### 数据库连接失败
```bash
# 检查 MySQL 容器
docker logs mysql_container

# 测试连接
docker exec qcc_plus mysql -h mysql -u qcc_user -p qcc_pass qcc_plus
```

## 📚 文档资源

- **GitHub 仓库**: https://github.com/yxhpy/qcc_plus
- **完整文档**: https://github.com/yxhpy/qcc_plus/tree/main/docs
- **多租户架构**: [docs/multi-tenant-architecture.md](https://github.com/yxhpy/qcc_plus/blob/main/docs/multi-tenant-architecture.md)
- **健康检查机制**: [docs/health_check_mechanism.md](https://github.com/yxhpy/qcc_plus/blob/main/docs/health_check_mechanism.md)
- **Cloudflare Tunnel**: [docs/cloudflare-tunnel.md](https://github.com/yxhpy/qcc_plus/blob/main/docs/cloudflare-tunnel.md)
- **更新日志**: [CHANGELOG.md](https://github.com/yxhpy/qcc_plus/blob/main/CHANGELOG.md)

## 🤝 支持与反馈

- **问题反馈**: https://github.com/yxhpy/qcc_plus/issues
- **���能建议**: https://github.com/yxhpy/qcc_plus/discussions
- **贡献指南**: https://github.com/yxhpy/qcc_plus/blob/main/CONTRIBUTING.md

## 📄 开源协议

MIT License - 详见 [LICENSE](https://github.com/yxhpy/qcc_plus/blob/main/LICENSE)

---

**Made with ❤️ by the QCC Plus Team**
