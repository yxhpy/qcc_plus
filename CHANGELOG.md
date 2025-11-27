# 更新日志

所有重要的更改都将记录在此文件中。

日志格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [1.7.1] - 2025-11-27

### 修复
- 全面统一 Claude Code 快速配置页面样式与系统风格一致
  - 移除所有硬编码颜色，全部使用 CSS 主题变量
  - install-card 改用主题变量，深色主题下显示渐变背景
  - tabs 样式改用 SystemSettings 的 tab-btn 规范
  - badge/pill/eyebrow 等组件使用主题变量
  - 表单输入框样式统一，添加 hover/focus 效果
  - 间距和圆角统一使用 CSS 变量

## [1.7.0] - 2025-11-27

### 新增
- **Claude Code 快速配置**：一键生成 settings.json 配置文件
  - 可视化配置生成器页面（/admin/claude-config）
  - 后端配置模板 API 和临时下载链接（24h TTL）
  - 支持 macOS/Linux 和 Windows 安装命令
  - 实时预览 JSON 配置内容
  - 侧边栏「快速配置」入口（核心功能分组）

## [1.6.1] - 2025-11-27

### 修复
- 修复健康检查历史数据显示不完整的问题（c4a206d, 6200426）
  - 当记录数超过 limit 时，只返回最新的 N 条记录
  - 使用子查询：先取最新 N 条，再按时间正序返回用于显示

## [1.6.0] - 2025-11-27

### 新增
- **配置中心**（#19）：统一的系统配置管理
  - 阶段1：配置中心后端实现（2bc48d7）
  - 阶段2&3：配置热更新与管理界面（68b234e）
  - 阶段4：config 表数据迁移到 settings 表（7046b75）
- 错误信息图标化显示与统计栏配置化（1309461）
- 监控大屏节点卡片视觉优化（3e51c8c）

### 修复
- 修复趋势数据实时性问题，包含当前小时的原始数据（d814e37）
- 修复系统设置配置项生效性问题（ed35229）
- 统一系统设置页面样式与其他页面一致（9b603c8）
- 修复 Tooltip 内容每行只显示一个字的问题（218a9e2）

## [1.5.0] - 2025-11-26

### 改进
- 简化节点状态显示为在线/离线两态，移除卡片顶部状态徽章，信息展示更聚焦（6c95008, 72f7531）
- 优化节点管理列表展示，提升密度与可读性（a237282）

### 文档
- 补充分支开发规范到记忆文件（d1ea930）

## [1.4.0] - 2025-11-26

### 新增
- 前端主题系统与侧边栏布局重构，支持多主题色并统一配色方案（aa7e33f, 5542e25）
- 图表颜色映射到主题系统，保持可视化一致性（202942b）

### 改进
- 全局间距与排版基线紧凑化，页面逐一优化为更高信息密度（61580b0, b895f16）

### 修复
- 移除 body/root 级别的居中和间距限制，避免布局被强制居中（1e335b3）

## [1.3.0] - 2025-11-26

### 新增

#### 监控大屏（重大特性）
- **实时监控大屏**：全新的监控大屏界面，实时展示节点状态和流量指标
- **健康检查历史时间线**：可视化展示节点健康检查历史记录
- **监控数据持久化**：支持多维度统计数据的持久化存储
- **分离代理流量和健康检查指标**：独立展示代理流量统计和健康检查数据

#### 分享功能
- **共享监控页面**：支持生成分享链接，允许外部访问监控大屏
- **实时 WebSocket 推送**：分享页面支持实时数据更新
- **完整指标展示**：分享大屏实时更新请求数、失败数等完整指标

#### 健康检查改进
- **默认使用 CLI 健康检查**：新建节点默认使用 CLI 方式进行健康检查

### 修复
- 修复节点累计统计数据持久化问题
- 修复撤销分享链接返回 unexpected response 错误
- 修复健康检查历史 tooltip 定位和重复恢复通知问题
- 修复分享页面健康检查历史 unauthorized 错误
- 修复分享链接时间统一使用北京时间
- 同步 SharedMonitor 与 Monitor 的 WebSocket 更新逻辑
- 添加 /monitor/ 路径到 SPA 路由白名单

### 改进
- 优化分享大屏布局，提升信息密度约 35%
- 后端统一使用 `timeutil.FormatBeijingTime` 输出北京时间
- 添加 UI 高信息密度规范

### 文档
- 添加监控数据持久化技术文档

## [1.2.0] - 2025-11-25

### 新增
- **节点拖拽排序功能**：支持通过拖拽调整节点优先级顺序
- **统一时间显示**：所有时间字段统一显示为北京时间（UTC+8）
  - 版本信息构建时间
  - 节点健康检查时间
  - 通知时间
  - 会话过期时间

### 修复
- 修复时区处理和通知时间显示问题
- 修复拖拽排序时的报错和时间格式显示
- 修复登录页面版本号显示双 v 的问题（v1.1.0 → 1.1.0）
- 修复部署脚本不同步 tags 导致版本号不正确的问题

### 改进
- 优化时间处理方案，统一使用 `BeijingTime` 类型
- 规范节点排序逻辑，使用 `weight` 字段替代 `order`

## [1.1.0] - 2025-11-24

### 新增

#### CLI 健康检查系统（重大特性）
- 新增 CLI 健康检查方式（Claude Code CLI 无头模式验证）
- 支持三种健康检查方式：API、HEAD、CLI
- 节点健康检查信息实时显示（最后检查时间、延迟、错误信息）
- CLI 健康检查架构简化：容器内直接安装 Claude CLI，移除 Docker-in-Docker

#### 版本管理系统
- 添加版本系统和 CHANGELOG 支持
- `/version` API 接口，返回版本、构建信息
- 前端侧边栏底部显示版本号

#### 通知系统
- 添加完整的通知系统支持
- 节点故障和恢复的实时通知
- 通知管理界面（查看、标记已读、删除）

#### CI/CD 自动化
- GitHub Actions 自动部署到测试环境
- 推送到 test 分支自动触发部署
- 健康检查验证部署成功

#### 品牌和 UI
- 统一前端品牌为 "QCC Plus"
- 添加品牌 favicon（frontend 和 website）
- 完整的 SEO meta 标签支持

### 重构
- **CLI 健康检查架构重大简化**
  - 移除 Docker-in-Docker 依赖
  - 不再需要挂载 Docker socket
  - 在容器内直接安装 Node.js 和 Claude Code CLI
  - 更快的健康检查响应（无容器启动开销）
  - 更简单的部署配置
- 移除 CLI 健康检查自动降级逻辑，保留真实错误信息

### 修复

#### CLI 健康检查
- 修复 CLI 健康检查超时问题（增加到 15 秒）
- 修复重启后失败节点不会自动进行健康检查的问题
- 修正 Claude CLI 参数：使用 `-p` 代替不存在的 `--non-interactive`
- 修复 Docker 构建问题：NodeSource nodejs 包已包含 npm

#### 节点管理
- 修复节点恢复时自动切换到优先级最高的健康节点
- 修复节点更新时保留 APIKey（api_key 为可选参数）

#### 通知系统
- 修复通知 API 返回数据格式解析问题
- 修复通知页面 `map is not a function` 错误

#### CI/CD 和部署
- 增强 CI/CD 健康检查机制：10s 初始等待 + 6 次重试
- 修复健康检查 curl 命令输出异常问题
- 改进 npm 安装错误处理逻辑
- 增强部署脚本的 npm 安装健壮性
- 修复部署脚本中的 awk 语法错误
- 修复部署脚本 Git 同步问题
- 在 workflow 中先强制更新代码

#### 其他
- 允许 favicon 和图标文件访问
- 降级 Docker Compose 版本到 3.7 兼容旧版本

### 改进

#### 安全性
- 移除 Docker socket 挂载要求，减少安全风险
- 容器隔离性更好，无法访问宿主机 Docker

#### 部署和配置
- 简化 `docker-compose.yml` 配置
- 更新 entrypoint 脚本，添加 Claude CLI 版本检查
- 优化前端构建流程

#### 文档
- 完善项目文档，同步与代码一致
- 重写 `docs/docker-cli-health-check-deployment.md`
- 更新 `docs/health_check_mechanism.md`
- 更新 `docs/cli_health_check_implementation.md`
- 添加 favicon 设置文档
- 添加版本发布规范文档
- 添加 GitHub 社区健康文件（CONTRIBUTING、CODE_OF_CONDUCT 等）

### 构建
- 更新前端构建产物（包含版本显示和通知功能）
- Docker Compose 升级到 v2.24.0

## [1.0.0] - 2025-11-23

### 新增
- 多租户架构支持，实现账号隔离
- React Web 管理界面（React 18 + TypeScript + Vite）
- Cloudflare Tunnel 集成，支持内网穿透
- Docker 化部署支持
- MySQL 数据持久化
- 多节点管理和自动故障切换
- 健康检查和自动探活机制（API、HEAD 方式）
- 会话管理和权限控制
- 实时监控和指标统计

### 核心特性
- Claude Code CLI 请求复刻
- 反向代理服务器（端口转发）
- 工具定义自动清理
- 事件驱动节点切换
- 管理员和普通账号权限分离

### 技术栈
- 后端：Go 1.21, MySQL, Docker
- 前端：React 18, TypeScript, Vite, Chart.js
- 部署：Docker Compose, Cloudflare Tunnel

[unreleased]: https://github.com/yxhpy/qcc_plus/compare/v1.5.0...HEAD
[1.5.0]: https://github.com/yxhpy/qcc_plus/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/yxhpy/qcc_plus/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.3.0
[1.2.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.2.0
[1.1.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.1.0
[1.0.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.0.0
