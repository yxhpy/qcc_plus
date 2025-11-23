# GitHub 项目设置指南

本文档说明如何在 GitHub 上设置 qcc_plus 项目的简介和元数据。

## 1. 项目描述（About）

在 GitHub 仓库页面，点击右上角的 ⚙️ 图标，设置以下信息：

### Description（简介）
```
Claude Code CLI 多租户代理服务器 - 支持多账号隔离、智能节点切换、自动故障恢复和 React Web 管理界面
```

### Website（网站）
```
https://github.com/yxhpy/qcc_plus
```

### Topics（标签）
添加以下标签：
- `claude-code`
- `claude-ai`
- `proxy-server`
- `multi-tenant`
- `golang`
- `react`
- `typescript`
- `docker`
- `api-proxy`
- `load-balancer`
- `high-availability`
- `anthropic`

## 2. 社区健康文件

### README.md
已完成 ✅ - 项目根目录的 README.md

### LICENSE
建议添加 MIT License：
```
MIT License

Copyright (c) 2025 yxhpy

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## 3. Release 设置

### v1.0.0 Release
标题：
```
qcc_plus v1.0.0 - 首个正式版本
```

描述：
```markdown
# qcc_plus v1.0.0

## 🎉 首个正式版本

qcc_plus 是一个功能完整的 Claude Code CLI 多租户代理服务器，支持账号隔离、智能节点管理和自动故障恢复。

## ✨ 核心特性

- **多租户账号隔离**：每个账号拥有独立的节点池和配置
- **智能节点切换**：事件驱动的节点切换，仅在状态变化时触发
- **自动故障恢复**：失败节点定期探活，自动恢复可用节点
- **React Web 管理界面**：现代化 SPA 界面，可视化管理
- **MySQL 持久化**：配置和统计数据持久化存储
- **Docker 部署**：一键部署，支持 Docker Compose
- **Cloudflare Tunnel 集成**：内置隧道支持，无需公网 IP

## 📦 安装方式

### Docker（推荐）
```bash
docker pull yxhpy520/qcc_plus:v1.0.0
docker run -d -p 8000:8000 \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  yxhpy520/qcc_plus:v1.0.0
```

### Docker Compose
```bash
git clone https://github.com/yxhpy/qcc_plus.git
cd qcc_plus
cp .env.example .env
# 编辑 .env 配置
docker compose up -d
```

### 源码构建
```bash
git clone https://github.com/yxhpy/qcc_plus.git
cd qcc_plus
go build -o cccli_bin ./cmd/cccli
./cccli_bin proxy
```

## 🚀 快速开始

```bash
# 启动代理服务器
UPSTREAM_API_KEY=sk-ant-your-key go run ./cmd/cccli proxy

# 访问管理界面
open http://localhost:8000/admin?admin_key=admin

# 使用代理
curl http://localhost:8000/v1/messages \
  -H "x-api-key: default-proxy-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}'
```

## 📚 文档

- [完整文档](https://github.com/yxhpy/qcc_plus/blob/main/README.md)
- [多租户架构](https://github.com/yxhpy/qcc_plus/blob/main/docs/multi-tenant-architecture.md)
- [快速开始指南](https://github.com/yxhpy/qcc_plus/blob/main/docs/quick-start-multi-tenant.md)
- [前端技术栈](https://github.com/yxhpy/qcc_plus/blob/main/docs/frontend-tech-stack.md)

## ⚠️ 安全提醒

生产环境必须修改默认凭证：
- `ADMIN_API_KEY`（默认: admin）
- `DEFAULT_PROXY_API_KEY`（默认: default-proxy-key）

## 🙏 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License
```

## 4. Docker Hub 设置

### Repository Description
```
Claude Code CLI 多租户代理服务器 - 支持多账号隔离、智能节点切换、自动故障恢复
```

### Full Description
复制 README.md 的内容，或使用以下简化版：
```markdown
# qcc_plus - Claude Code CLI Proxy Server

多租户 Claude Code CLI 代理服务器，支持：

- 多账号隔离和独立节点池
- 智能节点选择和自动故障切换
- React Web 管理界面
- MySQL 持久化
- Docker 一键部署

## Quick Start

```bash
docker run -d -p 8000:8000 \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  yxhpy520/qcc_plus:latest
```

访问管理界面：http://localhost:8000/admin?admin_key=admin

GitHub: https://github.com/yxhpy/qcc_plus
```

## 5. 社交媒体预览

在 GitHub 上传 repository social image（推荐尺寸 1280x640px），可以使用以下设计元素：
- 项目名称：qcc_plus
- 标语：Claude Code CLI Multi-Tenant Proxy
- 关键词：Multi-Tenant, Auto Failover, React UI
- 配色：蓝紫渐变（与 Web UI 一致）

## 完成后检查清单

- [ ] 设置 repository description
- [ ] 添加 topics/标签
- [ ] 创建 v1.0.0 release
- [ ] 上传到 Docker Hub
- [ ] 设置 Docker Hub description
- [ ] 添加 LICENSE 文件
- [ ] （可选）上传 social image
