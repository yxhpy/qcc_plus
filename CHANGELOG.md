# 更新日志

所有重要的更改都将记录在此文件中。

日志格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [1.9.0] - 2025-12-07

### 新增
- **使用量统计和计费功能**
  - 新增 `usage_logs` 表记录每次请求的 token 使用量
  - 支持按账号统计输入/输出 token 数
  - 支持按时间范围查询使用量
  - 为后续计费功能提供数据基础

### 修复
- **修复 usage_logs 查询 NULL 值处理问题**
  - 解决统计查询中 NULL 值导致的错误

- **修复 WebSocket 消息批处理导致 health_check 事件丢失的问题**
  - 优化 WebSocket 消息推送机制
  - 确保健康检查事件能够及时推送到前端

- **补全节点状态变更的 WebSocket 事件推送**
  - 节点状态变更时实时推送到前端
  - 提升监控大屏的实时性

- **修复节点状态变更未实时推送到前端的问题**
  - 优化状态同步机制
  - 确保前端及时获取最新节点状态

## [1.8.5] - 2025-12-05

### 改进
- **简化健康检查显示为单行紧凑模式**
  - 优化监控大屏健康检查历史的展示方式
  - 减少视觉噪音，提升信息密度

### 修复
- **恢复健康检查历史红绿状态点**
  - 修复健康检查历史时间线状态点颜色显示

- **修复监控大屏节点显示 0 个在线的问题**
  - 正确统计在线节点数量

- **修复健康检查事件不能实时显示在大屏的问题**
  - 健康检查事件现在能够实时推送到监控大屏

- **监控大屏全局指标加载优化**
  - 加载时显示 skeleton 骨架屏而非 0
  - 提升用户体验，避免误导

## [1.8.4] - 2025-12-05

### 新增
- **监控大屏显示熔断状态**
  - 节点卡片展示熔断器当前状态（Closed/Open/HalfOpen）
  - 区分"激活节点"和"实际使用节点"，更清晰地反映请求路由情况
  - 帮助运维快速识别被熔断器阻止的节点

### 修复
- **透传上游状态码**
  - 修复代理返回 502 而非上游实际状态码的问题
  - 避免 502 触发 CLI 客户端不必要的重试
  - 保留原始错误信息便于排查

- **监控大屏显示所有来源的健康检查历史**
  - 健康检查历史时间线现在包含代理请求产生的健康事件
  - 统一显示 API/HEAD/CLI/Proxy 四种来源的健康检查记录

## [1.8.3] - 2025-12-05

### 修复
- **区分 context canceled 和真正的上游错误**
  - 客户端主动取消请求（如用户中断）不再触发熔断器
  - 只有真正的上游错误才会被记录为失败
  - 避免用户正常的中断操作导致节点被误标记为失败

### 性能优化
- **CLI 健康检查速度优化约 25%**
  - 优化 CLI 启动参数，减少不必要的初始化开销
  - 提升健康检查响应速度

## [1.8.2] - 2025-12-04

### 修复
- **优化节点重试策略**
  - 自动尝试所有可用节点，不再限制固定重试次数
  - 熔断器跳过节点时不计入重试次数
  - 禁用 HTTP 状态码自动重试，避免不必要的重试

### 改进
- **提升故障切换效率**
  - 智能节点选择：优先使用未被熔断器阻止的健康节点
  - 减少无效重试：只对真正可恢复的错误进行重试
  - 更快的故障响应：立即切换到下一个可用节点

### 文档
- 更新配置文件注释，说明新的节点尝试逻辑

## [1.8.1] - 2025-12-04

### 修复
- **🔴 紧急修复：重试逻辑导致节点快速耗尽的严重缺陷**
  - 修复重试过程中过早标记节点全局失败的问题
  - 原问题：每次重试失败都会立即调用 `handleFailure`，导致并发请求快速耗尽所有节点
  - 新逻辑：只在最后一次尝试失败时才全局标记节点失败
  - 影响：避免单个请求的重试导致所有节点下线，防止出现 502 "all nodes failed"
  - 技术细节：在 `internal/proxy/handler.go:296-303` 中区分重试中的失败和真正的节点故障

### 改进
- **优化重试和超时配置**
  - 减少重试次数：3 次 → 2 次
  - 简化超时配置：统一使用 60 秒超时
  - CLI 健康检查超时：15 秒 → 30 秒
  - 提高系统在高并发场景下的稳定性

## [1.8.0] - 2025-12-02

### 新增
- **重试超时优化配置**：优化代理请求重试机制
  - 总超时时间限制为 25 秒
  - 递减超时策略：第1次 12s，第2次 6s，第3次 3s
  - 避免长时间等待失败节点，提升故障切换速度

### 修复
- **修复重试时 request body 被重用导致失败的问题**
  - 修复 HTTP request body 被读取后无法重用的问题
  - 在重试前正确克隆 request body
  - 确保每次重试都使用完整的原始请求体

### 文档
- **添加测试部署流程说明并保护敏感信息**
  - 新增测试服务器连接文档
  - 添加部署配置示例
  - 完善安全信息保护指南

## [1.7.7] - 2025-12-02

### 修复
- **🔴 紧急修复：熔断器死锁导致健康节点无法被选中**
  - 修复节点健康检查通过后显示绿色，但代理请求仍然重试的问题
  - 移除选择阶段对熔断器 Open 状态的硬过滤，允许进入 Half-Open 试探
  - 健康恢复时同步重置熔断器状态，清空失败计数
  - 解决"有健康节点但 all nodes failed after retry"的问题

### 技术细节
- `internal/proxy/node_manager.go`:
  - 移除 `selectHealthyNodeExcluding` 中的熔断器状态过滤（第277-283行）
  - 交由请求阶段的 `AllowRequest()` 控制熔断器状态转换
- `internal/proxy/health.go`:
  - 在 `checkNodeHealth` 成功分支添加熔断器重置逻辑（第299-304行）
  - 确保节点恢复后熔断器状态完全清理

### 影响
- 熔断器可以正常进入 Half-Open 状态进行试探
- 节点恢复后立即可用，无需等待冷却时间
- 监控大屏状态与实际可用性完全一致

## [1.7.6] - 2025-12-02

### 修复
- **完善健康检查历史记录机制**：解决实时状态与大屏显示不一致问题
  - 修复代理请求失败未记录到健康检查历史的问题
  - 修复健康检查历史记录过于冗余的问题
  - 新增智能去重机制：只在状态变化或超过5分钟时才持久化
  - 新增 `HealthCheckMethodProxy` 标识代理请求健康信号
  - 新增 `Store.LatestHealthCheck` 方法查询最近记录
  - WebSocket 推送不受影响，保持实时性

### 技术细节
- `internal/proxy/health.go`:
  - 新增 `HealthCheckMethodProxy` 常量和 `sameStateRecordGap` (5分钟)
  - 新增 `shouldInsertHealthRecord` 函数实现状态变更检测
  - 优化 `recordHealthEvent` 在写库前检查是否需要持久化
- `internal/proxy/handler.go`:
  - 代理失败时调用 `recordHealthEvent` 记录健康事件
- `internal/store/health_check.go`:
  - 新增 `LatestHealthCheck` 方法

### 效果
- 代理失败实时同步到监控大屏
- 历史记录减少约 98% 冗余（稳定运行时）
- 实时状态与历史数据完全一致

## [1.7.5] - 2025-12-01

### 改进
- **监控大屏默认展开健康检查历史**：优化用户体验，健康检查历史始终展开显示
  - 移除"展开加载/收起历史"按钮
  - 简化数据加载逻辑，健康检查历史自动加载
  - 减少用户交互步骤，提升监控效率

## [1.7.4] - 2025-11-30

### 性能优化
- **CLI 健康检查速度优化**：将检查时间从 10-20 秒降低到 7-8 秒
  - 添加 `--tools ""` 参数禁用工具加载，减少启动时间
  - 优化 prompt 从 `"hi"` 改为 `"say ok"`，获得稳定的短输出 "OK"
  - 保持 CLI 方式的反爬虫优势，同时大幅提升检查速度

## [1.7.3] - 2025-11-30

### 修复
- **🔴 紧急修复：节点故障切换机制重大缺陷**
  - 修复请求失败后节点未立即下线的问题（首次失败立即标记 Failed=true）
  - 修复健康检查失败后节点未标记失败状态的问题（所有失败节点加入 FailedSet）
  - 修复失败状态未及时持久化到数据库的问题（同步调用 UpsertNode）
  - 修复节点选择逻辑未检查 FailedSet 的问题（添加 isInFailedSet 检查）
  - 修复并发访问 FailedSet 的竞态条件（先复制 key 列表再迭代）
  - 修复节点恢复后未激活的问题（放宽恢复切换触发条件）
  - 生产日志验证：节点标记失败后仍持续使用 3+ 分钟，现已修复为立即切换

### 技术细节
- `handleFailure`: 首次失败立即标记并持久化，触发切换
- `checkNodeHealth`: 健康检查失败立即设置 Failed=true，所有失败节点加入 FailedSet
- `checkNodeHealth`: 放宽恢复切换触发条件，节点恢复后如果优先级更高立即激活
- `checkFailedNodes`: 消除并发竞态，先复制 key 列表再迭代
- `selectBestAndActivate`: 同时检查 Failed/Disabled/FailedSet，防止选到故障节点

### 性能提升
- 故障切换延迟：从 3+ 分钟降低到 < 1 秒（180x 提升）
- 节点恢复激活：从不激活到立即激活（秒级响应）
- 状态一致性：内存和数据库状态实时同步

## [1.7.2] - 2025-11-29

### 新增
- **健康检查失败立即切换**：探活发现节点故障后秒级切换到下一个可用节点
  - 将失败节点加入 FailedSet，异步触发 selectBestAndActivate
  - 发送节点离线通知和 WebSocket 事件
  - 保持探活周期不变，实现秒级故障响应

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

[unreleased]: https://github.com/yxhpy/qcc_plus/compare/v1.9.0...HEAD
[1.9.0]: https://github.com/yxhpy/qcc_plus/compare/v1.8.5...v1.9.0
[1.8.5]: https://github.com/yxhpy/qcc_plus/compare/v1.8.4...v1.8.5
[1.8.4]: https://github.com/yxhpy/qcc_plus/compare/v1.8.3...v1.8.4
[1.8.3]: https://github.com/yxhpy/qcc_plus/compare/v1.8.2...v1.8.3
[1.8.2]: https://github.com/yxhpy/qcc_plus/compare/v1.8.1...v1.8.2
[1.8.1]: https://github.com/yxhpy/qcc_plus/compare/v1.8.0...v1.8.1
[1.8.0]: https://github.com/yxhpy/qcc_plus/compare/v1.7.7...v1.8.0
[1.7.7]: https://github.com/yxhpy/qcc_plus/compare/v1.7.6...v1.7.7
[1.7.6]: https://github.com/yxhpy/qcc_plus/compare/v1.7.5...v1.7.6
[1.7.5]: https://github.com/yxhpy/qcc_plus/compare/v1.7.4...v1.7.5
[1.7.4]: https://github.com/yxhpy/qcc_plus/compare/v1.7.3...v1.7.4
[1.7.3]: https://github.com/yxhpy/qcc_plus/compare/v1.7.2...v1.7.3
[1.7.2]: https://github.com/yxhpy/qcc_plus/compare/v1.7.1...v1.7.2
[1.7.1]: https://github.com/yxhpy/qcc_plus/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/yxhpy/qcc_plus/compare/v1.6.1...v1.7.0
[1.6.1]: https://github.com/yxhpy/qcc_plus/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/yxhpy/qcc_plus/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/yxhpy/qcc_plus/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/yxhpy/qcc_plus/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.3.0
[1.2.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.2.0
[1.1.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.1.0
[1.0.0]: https://github.com/yxhpy/qcc_plus/releases/tag/v1.0.0
