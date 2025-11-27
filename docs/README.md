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
- [飞牛 NAS 部署指南](https://p.kdocs.cn/s/PNCAUCBEABAES) ⭐ - 飞牛 NAS Docker 部署教程

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
├── docker-hub-publish.md          # Docker 发布
├── persistence_fix.md             # 历史：持久化修复
├── health_check_improvement.md    # 历史：健康检查改进
├── tool-cleaning-fix.md           # 历史：工具清理
└── disable_node_feature.md        # 历史：禁用节点
```

### 需要帮助？

- **Bug 报告**：[GitHub Issues](https://github.com/yxhpy/qcc_plus/issues)
- **功能建议**：[GitHub Discussions](https://github.com/yxhpy/qcc_plus/discussions)
- **技术问题**：查阅相关文档或提交 Issue

## 版本信息

- **当前版本**：v1.7.0
- **最后更新**：2025-11-27
- **文档维护**：Claude Code

## 下一步

- 新用户：阅读 [README.md](../README.md) 和 [快速开始指南](./quick-start-multi-tenant.md)
- 前端开发：阅读 [前端技术栈](./frontend-tech-stack.md)
- 系统架构：阅读 [多租户架构设计](./multi-tenant-architecture.md)
- 部署运维：阅读 [Docker Hub 发布](./docker-hub-publish.md)
