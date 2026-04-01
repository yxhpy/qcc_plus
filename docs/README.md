# qcc_plus 文档索引

最后校准：2026-04-02

本文档按“入门、架构、功能、运维、开发”五个层次整理 qcc_plus 当前文档，口径以仓库代码为准。

## 入门

- [项目主页](../README.md)：安装方式、运行模式、关键环境变量、主要路由
- [多租户快速开始](./quick-start-multi-tenant.md)：从登录到创建账号和节点的最短路径
- [前端 README](../frontend/README.md)：前端本地开发与构建说明
- [npm CLI README](../npm-packages/@qccplus/cli/README.md)：`qccplus` 安装和命令用法

## 架构与核心机制

- [多租户架构](./multi-tenant-architecture.md)：账号、节点、路由、权限模型
- [健康检查机制](./health_check_mechanism.md)：`cli/api/head` 三种探活模式与调度配置
- [监控数据持久化](./monitoring-data-persistence.md)：监控指标、聚合与清理
- [通知系统](./notification-system.md)：通知渠道、订阅与事件模型
- [Cloudflare Tunnel](./cloudflare-tunnel.md)：Tunnel 配置与管理

## 索引型文档

- [API 索引](./api/INDEX.md)：HTTP 路由与核心包入口
- [模块注册表](./modules/REGISTRY.md)：主要模块职责、关键文件与对应文档

## 前端与界面

- [前端技术栈](./frontend-tech-stack.md)：React 19 管理端结构、路由、状态与构建
- [官网文档总览](./website-README.md)
- [官网设计概念](./website-design-concept.md)
- [官网技术规格](./website-technical-spec.md)
- [官网实现路线图](./website-implementation-roadmap.md)

## 发布与部署

- [发布流程](./release-workflow.md)
- [GoReleaser 指南](./goreleaser-guide.md)
- [Docker Hub 发布](./docker-hub-publish.md)
- [Docker Hub 信息更新](./docker-hub-update-guide.md)
- [CI/CD 部署说明](./ci-cd-deployment.md)
- [CI/CD 故障排查](./ci-cd-troubleshooting.md)
- [Docker CLI 健康检查部署](./docker-cli-health-check-deployment.md)

## Claude / Codex 协作

- [项目记忆](../CLAUDE.md)
- [编码规范](./claude/coding-standards.md)
- [Git 工作流](./claude/git-workflow.md)
- [发布规范](./claude/release-policy.md)
- [任务生命周期](./claude/task-lifecycle.md)
- [调试排查手册](./claude/debug-playbook.md)
- [踩坑记录](./claude/lessons-learned.md)
- [私有部署配置](./claude/deployment-private.md)

## 历史记录与专题

以下文档主要记录阶段性实现过程或历史修复，阅读时应以当前代码和主索引文档为准：

- [持久化修复](./persistence_fix.md)
- [健康检查改进](./health_check_improvement.md)
- [成本优先切换](./cost-first-node-switching.md)
- [节点切换优化](./node-switch-optimization.md)
- [禁用节点功能](./disable_node_feature.md)
- [监控相关修复](./bugfix_failed_set_restore.md)
- [部署崩溃诊断](./deploy-crash-diagnosis.md)
- [时间格式统一](./time-unification-summary.md)
- [工具清理修复](./tool-cleaning-fix.md)

## 当前事实口径

- 当前对外版本口径：`v1.9.4`
- 管理端前端：React 19 + TypeScript + Vite
- 默认存储：SQLite，本地路径默认 `~/.qccplus/qccplus.db`
- 可选持久化：MySQL（设置 `PROXY_MYSQL_DSN`）
- 默认只自动创建管理员账号 `admin/admin123`
- 默认普通账号不会自动创建

## 维护原则

1. 以代码和脚本为事实源，文档之间不得互相引用过时结论。
2. 涉及路由、命令、环境变量、默认值时，优先核对 `cmd/cccli/`、`internal/proxy/`、`npm-packages/@qccplus/cli/`。
3. 涉及前端页面和权限时，优先核对 `frontend/src/App.tsx` 与 `frontend/src/components/Layout.tsx`。
4. 修改功能后，至少同步更新 `README.md`、本索引、相关专题文档、必要时更新 API 索引和模块注册表。
