# qcc_plus 文档索引

欢迎来到 qcc_plus 项目文档中心。本文档提供了所有项目文档的导航和概览。

## 快速导航

### 核心文档
- [项目主页 (README.md)](../README.md) - 项目概述、快速开始、安装部署
- [项目记忆 (AGENTS.md)](../AGENTS.md) - 开发规范、工作流程、版本发布规范

### 架构与设计
- [多租户架构设计](./multi-tenant-architecture.md) - 完整的多租户系统架构说明
- [前端技术栈](./frontend-tech-stack.md) - React Web 界面技术栈和开发流程

### 使用指南
- [多租户快速开始](./quick-start-multi-tenant.md) - 多租户模式快速上手指南
- [Cloudflare Tunnel 集成](./cloudflare-tunnel.md) - 内网穿透和隧道配置指南

### 技术机制
- [健康检查机制](./health_check_mechanism.md) - 节点故障检测与自动恢复机制
- [监控数据持久化](./monitoring-data-persistence.md) - 多维度监控数据聚合与持久化存储
- [通知系统](./notification-system.md) - 通知系统设计与实现
- [节点切换优化](./node-switch-optimization.md) - 节点切换策略与优化
- [成本优先节点切换](./cost-first-node-switching.md) - 基于成本的节点选择策略

### 部署与发布
- [GoReleaser 自动化发布](./goreleaser-guide.md) - 一键发布流程（开发者必读）⭐
- [发布流程详解](./release-workflow.md) - 从开发到正式发布的完整流程
- [CI/CD 故障排查](./ci-cd-troubleshooting.md) - 部署问题诊断与解决
- [飞牛 NAS 部署指南](https://p.kdocs.cn/s/PNCAUCBEABAES) ⭐ - 飞牛 NAS Docker 部署教程（感谢 [@circircir-circle](https://github.com/circircir-circle) 贡献）

### 模块与 API
- [模块注册表](./modules/REGISTRY.md) - 所有模块的功能索引 ⭐
- [API 索引](./api/INDEX.md) - API 端点文档

### Claude 专用
- [踩坑记录](./claude/lessons-learned.md) - 开发过程中的经验教训
- [私有部署配置](./claude/deployment-private.md) - 🔒 私有部署配置

## 文档结构

```
docs/
├── README.md                      # 本文档（索引）
├── multi-tenant-architecture.md   # 多租户架构
├── quick-start-multi-tenant.md    # 快速开始
├── frontend-tech-stack.md         # 前端技术栈
├── health_check_mechanism.md      # 健康检查
├── monitoring-data-persistence.md # 监控数据持久化
├── notification-system.md         # 通知系统
├── node-switch-optimization.md    # 节点切换优化
├── cost-first-node-switching.md   # 成本优先节点切换
├── cloudflare-tunnel.md           # Cloudflare Tunnel 集成
├── goreleaser-guide.md            # GoReleaser 自动化发布 ⭐
├── release-workflow.md            # 发布流程详解
├── ci-cd-troubleshooting.md       # CI/CD 故障排查
├── api/                           # API 文档
│   └── INDEX.md                   # API 索引
├── claude/                        # Claude 专用文档
│   ├── lessons-learned.md         # 踩坑记录
│   └── deployment-private.md      # 私有部署配置
└── modules/                       # 模块文档
    └── REGISTRY.md                # 模块注册表
```

## 需要帮助？

- **Bug 报告**：[GitHub Issues](https://github.com/yxhpy/qcc_plus/issues)
- **功能建议**：[GitHub Discussions](https://github.com/yxhpy/qcc_plus/discussions)
- **技术问题**：查阅相关文档或提交 Issue

## 版本信息

- **当前版本**：v1.11.0
- **最后更新**：2026-02-10
- **文档维护**：Claude Code
