# qcc_plus 文档索引

欢迎来到 qcc_plus 项目文档中心。本文档提供了所有项目文档的导航和概览。

## 快速导航

### 核心文档
- [项目主页 (README.md)](../README.md) - 项目概述、快速开始、安装部署
- [项目记忆 (CLAUDE.md)](../CLAUDE.md) - 开发规范、工作流程、版本发布规范

### 架构与设计
- [多租户架构设计](./multi-tenant-architecture.md) - 完整的多租户系统架构说明
- [前端技术栈](./frontend-tech-stack.md) - React Web 界面技术栈和开发流程

### 使用指南
- [多租户快速开始](./quick-start-multi-tenant.md) - 多租户模式快速上手指南
- [Cloudflare Tunnel 集成](./cloudflare-tunnel.md) - 内网穿透和隧道配置指南

### 技术机制
- [健康检查机制](./health_check_mechanism.md) - 节点故障检测与自动恢复机制
- [监控数据持久化](./monitoring-data-persistence.md) - 多维度监控数据聚合与持久化存储

### 部署与发布
- [Docker Hub 发布指南](./docker-hub-publish.md) - 镜像构建与发布流程
- [GoReleaser 自动化发布](./goreleaser-guide.md) - 一键发布流程（开发者必读）⭐
- [发布流程详解](./release-workflow.md) - 从开发到正式发布的完整流程
- [CI/CD 故障排查](./ci-cd-troubleshooting.md) - 部署问题诊断与解决
- [飞牛 NAS 部署指南](https://p.kdocs.cn/s/PNCAUCBEABAES) ⭐ - 飞牛 NAS Docker 部署教程（感谢 [@circircir-circle](https://github.com/circircir-circle) 贡献）

## 按主题分类

### 🏗️ 架构设计

#### 多租户系统
- **[多租户架构设计](./multi-tenant-architecture.md)**
  - 数据模型（accounts、nodes、config）
  - 路由逻辑和权限模型
  - API 端点设计
  - 安全考虑和性能优化

#### 故障恢复
- **[健康检查机制](./health_check_mechanism.md)**
  - 被动失败检测（连续失败阈值）
  - 主动探活恢复（定期健康检查）
  - 事件驱动节点切换
  - 详细代码位置和实现逻辑

### 📊 监控与可视化

- **[监控数据持久化](./monitoring-data-persistence.md)**
  - 多维度指标聚合与保留策略
  - 代理流量与健康检查指标分离
  - 实时大屏与分享页面的数据源

### 💻 前端开发

- **[前端技术栈](./frontend-tech-stack.md)**
  - React 18 + TypeScript + Vite
  - 项目结构和组件设计
  - 路由和 API 服务
  - 认证机制和样式系统
  - 构建与部署流程

- **[前端 README](../frontend/README.md)**
  - 快速开始和开发模式
  - 功能特性详解
  - API 调用示例
  - 常见问题解决

### 📚 使用指南

#### 快速开始
- **[多租户快速开始](./quick-start-multi-tenant.md)**
  - 开箱即用示例
  - 生产化配置
  - 账号和节点管理
  - Docker 部署

#### Cloudflare Tunnel
- **[Cloudflare Tunnel 集成](./cloudflare-tunnel.md)**
  - 环境变量配置
  - 快速开始指南
  - Web 界面管理
  - 管理 API 使用
  - 故障排查和最佳实践

### 🚀 部署与运维

#### 自动化发布（推荐）
- **[GoReleaser 自动化发布](./goreleaser-guide.md)** ⭐
  - 一键发布流程
  - 多平台二进制构建
  - Docker 镜像自动发布
  - CHANGELOG 自动生成

#### 发布流程
- **[发布流程详解](./release-workflow.md)**
  - 测试环境验证
  - Pre-release 公开测试
  - 正式版本发布
  - 回滚策略

#### CI/CD
- **[CI/CD 故障排查](./ci-cd-troubleshooting.md)**
  - 健康检查超时问题
  - 部署脚本问题
  - GitHub Actions 配置
  - 服务器环境配置

#### Docker 部署
- **[Docker Hub 发布](./docker-hub-publish.md)**
  - 发布前准备
  - 自动化脚本使用
  - 镜像验证和测试
  - 版本规范和最佳实践

#### NAS 部署
- **[飞牛 NAS 部署指南](https://p.kdocs.cn/s/PNCAUCBEABAES)** ⭐ 外部文档
  - 飞牛 NAS Docker 部署教程
  - 图文详解安装步骤
  - 感谢 [@circircir-circle](https://github.com/circircir-circle) 贡献

#### 环境配置
- 参见 [主文档 - 环境变量配置](../README.md#环境变量配置)
- 参见 `.env.example` 文件

### 🔧 历史文档（仅供参考）

以下文档记录了特定功能的开发过程和修复历史，作为参考保留：

- [持久化修复](./persistence_fix.md) - 节点持久化问题修复记录
- [健康检查改进](./health_check_improvement.md) - 健康检查机制改进历史
- [工具清理修复](./tool-cleaning-fix.md) - 工具定义清理功能实现
- [禁用节点功能](./disable_node_feature.md) - 节点禁用功能开发记录

## 文档维护

### 文档更新原则
1. 所有文档必须与代码保持同步
2. 重大功能变更必须更新相关文档
3. 文档使用中文编写，保持简洁准确
4. 代码示例必须可以实际运行

### 文档结构
```
docs/
├── README.md                      # 本文档（索引）
├── multi-tenant-architecture.md   # 多租户架构
├── quick-start-multi-tenant.md    # 快速开始
├── frontend-tech-stack.md         # 前端技术栈
├── health_check_mechanism.md      # 健康检查
├── monitoring-data-persistence.md # 监控数据持久化
├── notification-system.md         # 通知系统
├── goreleaser-guide.md            # GoReleaser 自动化发布 ⭐
├── release-workflow.md            # 发布流程详解
├── ci-cd-deployment.md            # CI/CD 部署
├── ci-cd-troubleshooting.md       # CI/CD 故障排查
├── docker-hub-publish.md          # Docker 发布（手动，已弃用）
├── docker-hub-update-guide.md     # Docker Hub 信息更新
├── docker-cli-health-check-deployment.md # Docker CLI 健康检查部署
├── cloudflare-tunnel.md           # Cloudflare Tunnel 集成
├── cost-first-node-switching.md   # 成本优先节点切换
├── node-switch-optimization.md    # 节点切换优化
├── favicon-setup.md               # Favicon 配置
├── cli_health_check_implementation.md # CLI 健康检查实现
├── api/                           # API 文档
│   └── INDEX.md                   # API 索引
├── claude/                        # Claude 专用文档
│   ├── coding-standards.md        # 编码规范
│   ├── git-workflow.md            # Git 工作流
│   ├── release-policy.md          # 版本发布规范
│   ├── task-lifecycle.md          # 任务执行流程
│   ├── debug-playbook.md          # 调试排查手册
│   ├── deployment-private.md      # 私有部署配置
│   └── lessons-learned.md         # 踩坑记录
├── modules/                       # 模块文档
│   └── REGISTRY.md                # 模块注册表
├── website-design-concept.md      # 官网设计概念
├── website-design.md              # 官网设计
├── website-technical-spec.md      # 官网技术规格
├── website-implementation-roadmap.md # 官网实现路线图
├── website-README.md              # 官网说明
├── persistence_fix.md             # 历史：持久化修复
├── health_check_improvement.md    # 历史：健康检查改进
├── tool-cleaning-fix.md           # 历史：工具清理
├── disable_node_feature.md        # 历史：禁用节点
├── bugfix_failed_set_restore.md   # 历史：故障集恢复修复
├── crash-fix-v1.7.2.md            # 历史：v1.7.2 崩溃修复
├── deploy-crash-diagnosis.md      # 历史：部署崩溃诊断
└── time-unification-summary.md    # 历史：时间统一总结
```

### 需要帮助？

- **Bug 报告**：[GitHub Issues](https://github.com/yxhpy/qcc_plus/issues)
- **功能建议**：[GitHub Discussions](https://github.com/yxhpy/qcc_plus/discussions)
- **技术问题**：查阅相关文档或提交 Issue

## 版本信息

- **当前版本**：v1.9.4
- **最后更新**：2025-12-10
- **文档维护**：Claude Code

## 下一步

- 新用户：阅读 [README.md](../README.md) 和 [快速开始指南](./quick-start-multi-tenant.md)
- 前端开发：阅读 [前端技术栈](./frontend-tech-stack.md)
- 系统架构：阅读 [多租户架构设计](./multi-tenant-architecture.md)
- 部署运维：阅读 [Docker Hub 发布](./docker-hub-publish.md)
