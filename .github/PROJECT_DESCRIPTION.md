# qcc_plus 项目描述

> 用于 GitHub 仓库 About 部分的项目描述

## 简短描述（Short Description）

```
Claude Code CLI 多租户代理服务器 - 支持多账号隔离、智能节点切换、自动故障恢复和 React Web 管理界面
```

## 完整描述（Full Description）

```markdown
qcc_plus 是一个功能完整的 Claude Code CLI 多租户代理服务器，为团队和企业提供强大的 API 管理和路由能力。

## 核心特性

🏢 **多租户架构** - 每个账号拥有独立的节点池和配置，完全隔离
🔄 **智能节点切换** - 事件驱动的节点选择，仅在状态变化时触发
💚 **自动故障恢复** - 失败节点定期探活，自动恢复可用节点
🎨 **现代化 Web 界面** - React 18 + TypeScript SPA，可视化管理
💾 **MySQL 持久化** - 配置和统计数据持久化存储
🐳 **一键 Docker 部署** - 完整的 Docker Compose 支持
🌐 **Cloudflare Tunnel 集成** - 内置隧道支持，无需公网 IP

## 快速开始

```bash
docker run -d -p 8000:8000 \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  yxhpy520/qcc_plus:latest
```

访问 http://localhost:8000/admin 开始使用！

## 文档

- [完整文档](https://github.com/yxhpy/qcc_plus#readme)
- [多租户架构](https://github.com/yxhpy/qcc_plus/blob/main/docs/multi-tenant-architecture.md)
- [快速开始指南](https://github.com/yxhpy/qcc_plus/blob/main/docs/quick-start-multi-tenant.md)
```

## 网站（Website）

```
https://github.com/yxhpy/qcc_plus
```

## Topics（标签）

请在 GitHub 仓库设置中添加以下 Topics：

```
claude-code
claude-ai
anthropic
proxy-server
reverse-proxy
multi-tenant
golang
go
react
typescript
vite
docker
api-gateway
load-balancer
high-availability
failover
mysql
cloudflare-tunnel
web-ui
dashboard
```

## GitHub 徽章

可以在 README.md 中使用的徽章：

```markdown
[![Version](https://img.shields.io/badge/version-1.8.5-blue.svg)](https://github.com/yxhpy/qcc_plus/releases/tag/v1.8.5)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/docker-yxhpy520%2Fqcc__plus-blue?logo=docker)](https://hub.docker.com/r/yxhpy520/qcc_plus)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](./LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/yxhpy/qcc_plus)](https://github.com/yxhpy/qcc_plus/issues)
[![GitHub stars](https://img.shields.io/github/stars/yxhpy/qcc_plus)](https://github.com/yxhpy/qcc_plus/stargazers)
```

## 社交媒体卡片（Social Preview）

建议创建一个 1280x640px 的图片，包含以下元素：

- 项目名称：qcc_plus
- 标语：Claude Code CLI Multi-Tenant Proxy
- 关键特性图标：
  - 多租户 🏢
  - 自动切换 🔄
  - Web UI 🎨
  - Docker 🐳
- 配色方案：蓝紫渐变（与 Web UI 一致）
- GitHub Logo

## 在线示例

如果你部署了公开的演示实例，可以添加：

```
Demo: https://demo.example.com
Username: demo
Password: demo123
```

## 统计信息

在 GitHub Insights 中可以看到：

- Stars（收藏数）
- Forks（分支数）
- Contributors（贡献者）
- Used by（被使用情况）

## 相关链接

- **Docker Hub**: https://hub.docker.com/r/yxhpy520/qcc_plus
- **文档**: https://github.com/yxhpy/qcc_plus/tree/main/docs
- **问题反馈**: https://github.com/yxhpy/qcc_plus/issues
- **贡献指南**: https://github.com/yxhpy/qcc_plus/blob/main/CONTRIBUTING.md
- **安全策略**: https://github.com/yxhpy/qcc_plus/blob/main/SECURITY.md

---

**最后更新**: 2025-12-06
