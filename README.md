# qcc_plus - Claude Code CLI 多租户代理服务器

[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/yxhpy/qcc_plus/releases/tag/v1.0.0)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/docker-yxhpy520%2Fqcc__plus-blue?logo=docker)](https://hub.docker.com/r/yxhpy520/qcc_plus)

## 概述

qcc_plus 是一个功能完整的 Claude Code CLI 代理服务器，支持多租户账号隔离、多节点管理、自动故障切换和 Web 管理界面。

### 核心特性

- **多租户账号隔离**：每个账号拥有独立的节点池和配置
- **智能路由**：根据 API Key 自动路由到对应账号的节点
- **多节点管理**：支持配置多个上游节点，权重优先级控制
- **智能故障切换**：事件驱动的节点切换，仅在状态变化时触发
- **自动探活恢复**：失败节点定期探活，自动恢复可用节点
- **React Web 管理界面**：现代化 SPA 界面，可视化管理账号和节点
- **MySQL 持久化**：配置和统计数据持久化存储
- **Docker 部署**：一键部署，支持 Docker Compose
- **Cloudflare Tunnel 集成**：内置隧道支持，无需公网 IP

## 快速开始

### Docker 部署（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/yxhpy/qcc_plus.git
cd qcc_plus

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件，修改 UPSTREAM_API_KEY 和安全凭证

# 3. 启动服务
docker compose up -d

# 4. 访问管理界面
open http://localhost:8000/admin
```

### 从源码运行

```bash
# 启动代理服务器
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy
```

启动后输出默认登录凭证（内存模式）：
- 管理员：username=`admin` password=`admin123`
- 默认账号：username=`default` password=`default123`
- 提示：配置了 `PROXY_MYSQL_DSN`（持久化模式）时不会自动创建默认账号，请登录后自行创建账号与节点。

### 访问管理界面

http://localhost:8000/admin

### 使用代理

```bash
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":"hi"}],"max_tokens":100}'
```
> 仅当存在默认账号且其 proxy_api_key 为 `default-proxy-key` 时可直接使用；持久化模式需先创建账号和节点。

## ⚠️ 默认凭证

**安全警告**：以下默认凭证仅供本地测试，生产环境必须修改！

| 类型 | 默认值 |
|------|--------|
| 管理员登录 | username `admin` / password `admin123` |
| 默认账号登录 | （仅内存模式自动创建）username `default` / password `default123` |

修改服务端配置密钥：
```bash
# 影响后台默认密钥注入，不改变已存在用户密码
export ADMIN_API_KEY=your-secure-key
export DEFAULT_PROXY_API_KEY=your-proxy-key
```

## 多租户使用

系统默认启用多租户模式，支持完全的账号隔离。管理界面与管理 API 需先通过 `/login` 表单登录（`username`/`password`，获取 `session_token` Cookie），再携带 Cookie 访问。

### 创建新账号（需先登录获取 Cookie）

```bash
# 先登录并保存 Cookie（表单提交）
auth_cookie=cookies.txt
curl -c "$auth_cookie" -X POST \
  -d "username=admin&password=admin123" \
  http://localhost:8000/login

# 使用 Cookie 调用管理 API
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "name":"team-alpha",
    "proxy_api_key":"alpha-key",
    "is_admin":false
  }'
```

### 为账号添加节点

```bash
curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name":"node-1",
    "base_url":"https://api.anthropic.com",
    "api_key":"sk-ant-xxx",
    "weight":1
  }'
```

### 使用账号代理

```bash
curl http://localhost:8000/v1/messages \
  -H "x-api-key: alpha-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-5-20250929",
    "messages":[{"role":"user","content":"Hello"}],
    "max_tokens":1024
  }'
```

## 环境变量配置

### 基础配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| LISTEN_ADDR | 监听地址 | `:8000` |
| UPSTREAM_BASE_URL | 上游 API 地址 | `https://api.anthropic.com` |
| UPSTREAM_API_KEY | 默认上游 API Key | - |
| UPSTREAM_NAME | 默认节点名称 | `default` |

### 代理配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| PROXY_RETRY_MAX | 重试次数 | `3` |
| PROXY_FAIL_THRESHOLD | 失败阈值（连续失败多少次标记失败） | `3` |
| PROXY_HEALTH_INTERVAL_SEC | 探活间隔（秒） | `30` |
| PROXY_MYSQL_DSN | MySQL 连接字符串 | - |

### 多租户配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| ADMIN_API_KEY | 管理员访问密钥（服务内部校验，非前端登录口令） | `admin` ⚠️ |
| DEFAULT_ACCOUNT_NAME | 默认账号名称（仅内存模式自动创建） | `default` |
| DEFAULT_PROXY_API_KEY | 默认代理 API Key（仅内存模式自动创建） | `default-proxy-key` ⚠️ |

### Cloudflare Tunnel 配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| CF_API_TOKEN | Cloudflare API Token | - |
| TUNNEL_SUBDOMAIN | 隧道子域名（如 `my-proxy`） | - |
| TUNNEL_ZONE | Cloudflare Zone（域名，如 `example.com`） | - |
| TUNNEL_ENABLED | 启用隧道功能 | `false` |

⚠️ **安全警告**：生产环境必须修改默认的 `ADMIN_API_KEY` 和 `DEFAULT_PROXY_API_KEY`！

## 文档

- [多租户架构设计](docs/multi-tenant-architecture.md) - 完整的多租户系统架构
- [快速开始指南](docs/quick-start-multi-tenant.md) - 多租户模式使用指南
- [Cloudflare Tunnel 集成](docs/cloudflare-tunnel.md) - 内网穿透和隧道配置
- [前端技术栈](docs/frontend-tech-stack.md) - React Web 界面开发文档
- [健康检查机制](docs/health_check_mechanism.md) - 节点故障检测与恢复
- [Docker Hub 发布](docs/docker-hub-publish.md) - 镜像发布流程
- [文档索引](docs/README.md) - 所有文档导航
- [项目记忆](CLAUDE.md) - 开发规范和工作流程

## 项目结构

```
qcc_plus/
├── cmd/cccli/          # 程序入口
│   └── main.go         # 支持消息模式和代理模式
├── internal/
│   ├── client/         # Claude API 客户端（请求构造、预热、SSE）
│   ├── proxy/          # 反向代理服务器（多租户、节点管理）
│   └── store/          # 数据持久化层（MySQL）
├── frontend/           # React 前端源码
│   ├── src/            # TypeScript/React 组件
│   ├── dist/           # 构建输出（Git 忽略）
│   └── package.json
├── web/                # Go embed 前端资源
│   ├── embed.go        # 资源嵌入声明
│   └── dist/           # 前端构建产物（从 frontend/dist 复制）
├── cccli/              # 系统 prompt 和工具定义（embed）
├── scripts/            # 部署和构建脚本
├── docs/               # 项目文档
├── docker-compose.yml  # Docker Compose 配置
└── Dockerfile          # Docker 镜像构建
```

## 技术栈

- **后端**：Go 1.21, MySQL 8.0, Docker
- **前端**：React 18, TypeScript, Vite, Chart.js
- **部署**：Docker Compose, Cloudflare Tunnel

## 开源协议

MIT
