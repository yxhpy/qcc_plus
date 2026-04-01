# 前端技术栈

最后校准：2026-04-02

## 核心栈

- React 19
- TypeScript 5
- Vite 7
- React Router DOM 7
- Chart.js 4 + react-chartjs-2
- CSS Variables 主题体系

## 目录结构

```text
frontend/
├── src/
│   ├── App.tsx
│   ├── components/
│   ├── contexts/
│   ├── hooks/
│   ├── pages/
│   ├── services/
│   ├── themes/
│   ├── types/
│   └── utils/
├── package.json
├── vite.config.ts
└── eslint.config.js
```

## 当前页面

### 登录与导航

- `Login`
- `Dashboard`
- `ChangelogPage`

### 账号与节点

- `Accounts`
- `Nodes`
- `Settings`
- `SystemSettings`

### 监控与分析

- `Monitor`
- `MonitorShares`
- `SharedMonitor`
- `Usage`
- `RequestLogs`
- `ModelRecovery`

### 配置与扩展

- `ClaudeConfig`
- `Pricing`
- `Notifications`
- `TunnelSettings`

## 路由口径

当前路由以 `frontend/src/App.tsx` 为准：

- `/login`
- `/admin/dashboard`
- `/admin/accounts`
- `/admin/nodes`
- `/admin/claude-config`
- `/admin/monitor`
- `/admin/monitor-shares`
- `/admin/settings`
- `/settings`
- `/admin/notifications`
- `/changelog`
- `/admin/tunnel`
- `/admin/pricing`
- `/admin/usage`
- `/admin/request-logs`
- `/admin/model-recovery`
- `/monitor/share/:token`

## 状态管理

- `useAuth`：登录态与管理员权限
- `NodeMetricsContext`：节点监控指标
- `SettingsContext`：设置缓存
- `ModelRecoveryContext`：模型恢复状态
- `useMonitorWebSocket`：监控 WebSocket 实时同步

## API 交互

- `src/services/api.ts`：主要业务 API
- `src/services/settingsApi.ts`：设置相关 API

前端主要消费以下后端能力：

- 账号 / 节点管理
- 监控大屏和 WebSocket
- 定价、使用量、请求日志
- 通知渠道与订阅
- Claude Code 配置模板
- Tunnel 管理
- 模型恢复状态

## 构建方式

开发时可直接在 `frontend/` 中运行：

```bash
npm install
npm run dev
```

集成到 Go 服务时必须使用：

```bash
bash scripts/build-frontend.sh
```

原因：

- 该脚本会先执行 `frontend` 的 `npm run build`
- 再将 `frontend/dist` 同步到 `web/dist`
- Go 二进制通过 `web/embed.go` 嵌入 `web/dist`

## 样式约束

- 禁止硬编码颜色，统一使用 CSS 变量
- 主题相关定义位于 `src/themes/` 和 `src/themes/variables.css`
- 页面样式主要分布在 `src/pages/*.css` 和 `src/components/*.css`

## 相关文档

- [前端 README](../frontend/README.md)
- [多租户架构](./multi-tenant-architecture.md)
- [监控数据持久化](./monitoring-data-persistence.md)
- [API 索引](./api/INDEX.md)
