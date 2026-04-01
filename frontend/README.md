# qcc_plus 前端

qcc_plus 管理端基于 React 19、TypeScript 和 Vite 构建。

## 本地开发

```bash
cd frontend
npm install
npm run dev
```

默认开发地址：`http://localhost:5173`

## 生产构建

```bash
cd frontend
npm run build
npm run preview
```

如果要让 Go 服务加载最新前端，必须在仓库根目录执行：

```bash
bash scripts/build-frontend.sh
```

该脚本会把 `frontend/dist` 复制到 `web/dist`，供 `web/embed.go` 嵌入。

## 当前页面

- `Login`
- `Dashboard`
- `Accounts`
- `Nodes`
- `Monitor`
- `MonitorShares`
- `SharedMonitor`
- `Settings`
- `SystemSettings`
- `TunnelSettings`
- `Notifications`
- `ClaudeConfig`
- `Pricing`
- `Usage`
- `RequestLogs`
- `ModelRecovery`
- `ChangelogPage`

## 目录说明

```text
frontend/src/
├── App.tsx              # 路由入口
├── pages/               # 页面
├── components/          # 通用组件
├── contexts/            # React Context
├── hooks/               # 自定义 Hooks
├── services/            # API 调用
├── themes/              # 主题与设计令牌
├── types/               # 类型定义
└── utils/               # 工具函数
```

## 关键约束

- 路由定义以 `src/App.tsx` 为准。
- 权限控制由 `useAuth` 和 `ProtectedRoute` 实现。
- 颜色和主题统一走 CSS 变量，不写硬编码颜色。
- 监控实时更新依赖 `/api/monitor/ws`。

## 常见任务

### 新增页面

1. 在 `src/pages/` 新建页面组件
2. 在 `src/App.tsx` 注册路由
3. 在 `src/components/Layout.tsx` 增加导航
4. 如需接口，补充 `src/services/api.ts` 和 `src/types/index.ts`

### 修改接口

1. 先核对 `internal/proxy/handler.go` 和对应 `api_*.go`
2. 再修改 `src/services/api.ts`
3. 最后同步更新 `docs/api/INDEX.md`

## 相关文档

- [前端技术栈](../docs/frontend-tech-stack.md)
- [项目主页](../README.md)
- [多租户架构](../docs/multi-tenant-architecture.md)
