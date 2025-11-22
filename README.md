# qcc_plus - Claude Code CLI 多租户代理服务器

[![Version](https://img.shields.io/badge/version-3.0.0-blue.svg)](https://github.com/yourusername/qcc_plus)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)

## 概述

qcc_plus 是一个功能完整的 Claude Code CLI 代理服务器，支持多租户账号隔离、多节点管理、自动故障切换和 Web 管理界面。

### 核心特性

- **多租户账号隔离**：每个账号拥有独立的节点池和配置
- **智能路由**：根据 API Key 自动路由到对应账号的节点
- **多节点管理**：支持配置多个上游节点，自动负载均衡
- **故障切换**：节点失败自动切换，支持探活恢复
- **Web 管理界面**：可视化管理账号、节点和配置
- **MySQL 持久化**：配置和统计数据持久化存储

## 快速开始

### 启动代理

```bash
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy
```

启动后输出默认凭证：
- 管理员密钥：`admin`
- 默认账号 Proxy Key：`default-proxy-key`

### 访问管理界面

http://localhost:8000/admin?admin_key=admin

### 使用代理

```bash
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":"hi"}],"max_tokens":100}'
```

## ⚠️ 默认凭证

**安全警告**：以下默认凭证仅供本地测试，生产环境必须修改！

| 类型 | 默认值 |
|------|--------|
| 管理员密钥 | `admin` |
| 默认 Proxy Key | `default-proxy-key` |

修改凭证：
```bash
export ADMIN_API_KEY=your-secure-key
export DEFAULT_PROXY_API_KEY=your-proxy-key
```

## 多租户使用

### 创建账号

```bash
curl -X POST http://localhost:8000/admin/api/accounts \
  -H "x-admin-key: admin" \
  -d '{"name":"team-alpha","proxy_api_key":"alpha-key","is_admin":false}'
```

### 添加节点

```bash
curl -X POST http://localhost:8000/admin/api/nodes \
  -H "x-api-key: alpha-key" \
  -d '{"name":"node-1","base_url":"https://api.anthropic.com","api_key":"sk-ant-xxx","weight":1}'
```

## 文档

- [完整架构](docs/multi-tenant-architecture.md)
- [快速指南](docs/quick-start-multi-tenant.md)
- [项目记忆](CLAUDE.md)

## 许可证

MIT
