# qcc_plus

[![Version](https://img.shields.io/badge/version-1.12.1-blue.svg)](https://github.com/yxhpy/qcc_plus/releases/tag/v1.12.1)
[![npm](https://img.shields.io/npm/v/@qccplus/cli.svg?logo=npm)](https://www.npmjs.com/package/@qccplus/cli)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/docker-yxhpy520%2Fqcc__plus-blue?logo=docker)](https://hub.docker.com/r/yxhpy520/qcc_plus)

qcc_plus 是一个面向 Claude Code CLI 的多租户代理服务，提供账号隔离、节点管理、自动故障切换、React 管理后台、监控大屏、通知、Cloudflare Tunnel 和 npm/Docker 分发能力。

## 核心能力

- 多租户账号隔离，按 `proxy_api_key` 路由到账号独立节点池
- 多节点管理，支持权重优先、禁用、激活、自动故障切换
- 健康检查支持 `cli`、`api`、`head` 三种模式
- React 19 + TypeScript + Vite 管理界面
- 监控大屏、分享监控页、WebSocket 实时推送
- 模型定价、使用量汇总、请求日志、模型恢复跟踪
- 通知渠道和订阅管理
- Cloudflare Tunnel 集成
- npm CLI、Docker 镜像、源码三种使用方式

## 运行模式

- 默认存储不是内存模式，而是本地 SQLite。
- 未设置 `PROXY_MYSQL_DSN` 时，服务会默认使用 `~/.qccplus/qccplus.db`。
- 可通过 `PROXY_SQLITE_PATH` 指定 SQLite 文件路径。
- 设置 `PROXY_MYSQL_DSN` 后切换到 MySQL。
- 启动时会自动创建管理员账号 `admin/admin123`。
- 默认普通账号不会自动创建；首次启动后需要登录管理后台手动创建账号和节点。

## 快速开始

### npm 安装

```bash
npm install -g @qccplus/cli

# 初始化本地配置（会写入 ~/.qccplus/config.yaml）
qccplus config init

# 配置上游 API Key
qccplus config set upstream.api_key sk-ant-your-key

# 启动服务
qccplus start
```

访问管理界面：`http://localhost:8000/admin`

首次登录：

- 管理员账号：`admin`
- 管理员密码：`admin123`

### Docker 本地体验

```bash
git clone https://github.com/yxhpy/qcc_plus.git
cd qcc_plus
docker compose up -d
```

上面的 `docker compose up -d` 仅适合本地开发 / 快速体验。

正式测试环境或生产环境部署，请统一使用 `scripts/deploy-server.sh test` / `scripts/deploy-server.sh prod`，由脚本负责代码同步、前端构建和 `docker compose` 重建。

### 源码运行

```bash
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy
```

## 最小可用流程

1. 登录 `http://localhost:8000/admin`
2. 使用 `admin/admin123` 登录
3. 创建一个普通账号，例如 `team-alpha`
4. 为该账号添加至少一个节点
5. 使用该账号的 `proxy_api_key` 调用 `/v1/messages`

示例：

```bash
auth_cookie=cookies.txt

curl -c "$auth_cookie" -X POST \
  -d "username=admin&password=admin123" \
  http://localhost:8000/login

curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "name":"team-alpha",
    "proxy_api_key":"alpha-key",
    "is_admin":false
  }'

curl -b "$auth_cookie" -X POST http://localhost:8000/admin/api/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name":"alpha-node-1",
    "base_url":"https://api.anthropic.com",
    "api_key":"sk-ant-your-key",
    "weight":1
  }'

curl http://localhost:8000/v1/messages \
  -H "x-api-key: alpha-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-5-20250929",
    "messages":[{"role":"user","content":"Hello"}],
    "max_tokens":128
  }'
```

## 常用命令

```bash
qccplus start
qccplus stop
qccplus restart
qccplus status
qccplus logs -f
qccplus proxy
qccplus config list
qccplus version
```

## 关键环境变量

### 基础

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LISTEN_ADDR` | 服务监听地址 | `:8000` |
| `UPSTREAM_BASE_URL` | 默认上游地址 | `https://api.anthropic.com` |
| `UPSTREAM_API_KEY` | 默认上游 API Key | - |
| `UPSTREAM_NAME` | 默认节点名称 | `default` |
| `PROXY_SQLITE_PATH` | SQLite 文件路径 | `~/.qccplus/qccplus.db` |
| `PROXY_MYSQL_DSN` | MySQL DSN，设置后切换为 MySQL | - |

### 账号与安全

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ADMIN_API_KEY` | 管理员代理 Key | `admin` |
| `DEFAULT_ACCOUNT_NAME` | 默认账号名称口径 | `default` |
| `DEFAULT_PROXY_API_KEY` | 默认账号代理 Key 口径 | `default-proxy-key` |

### 健康检查

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PROXY_HEALTH_CHECK_MODE` | 全局健康检查方式：`cli/api/head` | `cli` |
| `PROXY_FAIL_THRESHOLD` | 连续失败阈值 | `3` |
| `PROXY_HEALTH_INTERVAL_SEC` | 失败节点探活间隔 | `30` |
| `PROXY_HEALTH_CHECK_ALL_INTERVAL` | 全量健康检查间隔 | `10m` |
| `HEALTH_ALL_INTERVAL_MIN` | 全量健康检查旧格式备选值 | `10` |
| `HEALTH_CHECK_CONCURRENCY` | 全量健康检查并发数 | `2` |
| `HEALTH_CHECK_CONCURRENCY_CLI` | CLI 健康检查并发数 | `1` |

### Cloudflare Tunnel

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `TUNNEL_ENABLED` | 是否启用 Tunnel | `false` |
| `CF_API_TOKEN` | Cloudflare API Token | - |
| `TUNNEL_SUBDOMAIN` | 子域名 | - |
| `TUNNEL_ZONE` | Zone 名称 | - |

## 主要路由

### 前端

- `/login`
- `/admin/dashboard`
- `/admin/accounts`
- `/admin/nodes`
- `/admin/monitor`
- `/admin/monitor-shares`
- `/admin/settings`
- `/settings`
- `/admin/notifications`
- `/admin/claude-config`
- `/admin/pricing`
- `/admin/usage`
- `/admin/request-logs`
- `/admin/model-recovery`
- `/admin/tunnel`
- `/changelog`
- `/monitor/share/:token`

### 后端

- `POST /login`
- `POST /logout`
- `GET /version`
- `POST /v1/messages`
- `GET|POST|PUT|DELETE /admin/api/accounts`
- `GET|POST|PUT|DELETE /admin/api/nodes`
- `GET|PUT /admin/api/config`
- `POST /admin/api/nodes/activate`
- `POST /admin/api/nodes/disable`
- `POST /admin/api/nodes/enable`
- `GET|PUT /admin/api/tunnel`
- `POST /admin/api/tunnel/start`
- `POST /admin/api/tunnel/stop`
- `GET /admin/api/tunnel/zones`
- `GET /api/monitor/dashboard`
- `GET /api/monitor/ws`
- `GET|POST|DELETE /api/monitor/shares`
- `GET /api/monitor/share/:token`
- `GET /api/nodes/:id/metrics`
- `GET /api/nodes/:id/health-history`
- `GET /api/accounts/:id/metrics`
- `POST /api/metrics/aggregate`
- `POST /api/metrics/cleanup`
- `GET|POST|DELETE /api/pricing`
- `POST /api/pricing/sync`
- `GET /api/usage/logs`
- `GET /api/usage/summary`
- `POST /api/usage/cleanup`
- `GET /api/claude-config/template`
- `GET /api/claude-config/download/:id`
- `GET|POST|PUT|DELETE /api/notification/*`
- `GET|PUT|DELETE /api/settings*`
- `GET /api/envvars`
- `GET /api/envvars/categories`
- `GET|POST /api/model-recovery*`

## 项目结构

```text
qcc_plus/
├── cmd/cccli/              # Go 入口
├── internal/client/        # Claude API 客户端
├── internal/proxy/         # 代理服务与 HTTP API
├── internal/store/         # MySQL / SQLite 存储
├── internal/notify/        # 通知系统
├── internal/tunnel/        # Cloudflare Tunnel
├── internal/timeutil/      # 时间工具
├── internal/version/       # 版本信息
├── frontend/               # React 19 前端源码
├── web/                    # Go embed 前端产物
├── npm-packages/           # npm 多平台分发包
├── website/                # 官网项目
├── scripts/                # 构建与部署脚本
├── docs/                   # 项目文档
└── .claude/                # 项目内 Claude Skills / Agents / Scripts
```

## 技术栈

- 后端：Go 1.21+
- 存储：SQLite / MySQL
- 前端：React 19、TypeScript、Vite、Chart.js
- 传输：HTTP、SSE、WebSocket
- 部署：Docker Compose、npm CLI、Cloudflare Tunnel

## 文档

- [文档索引](docs/README.md)
- [多租户快速开始](docs/quick-start-multi-tenant.md)
- [多租户架构](docs/multi-tenant-architecture.md)
- [健康检查机制](docs/health_check_mechanism.md)
- [前端技术栈](docs/frontend-tech-stack.md)
- [API 索引](docs/api/INDEX.md)
- [模块注册表](docs/modules/REGISTRY.md)
- [项目记忆](CLAUDE.md)

## 开源协议

MIT
