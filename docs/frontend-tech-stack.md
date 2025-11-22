# 前端技术栈

## 技术栈概述

项目前端已从内联 HTML/JS 迁移到 **React + TypeScript + Vite** 技术栈。

### 核心技术
- **React 18** - UI 框架
- **TypeScript** - 类型安全
- **Vite** - 构建工具
- **React Router DOM** - 客户端路由
- **Chart.js + react-chartjs-2** - 数据可视化
- **CSS Variables** - 主题系统

## 项目结构

```
qcc_plus/
├── frontend/               # React 前端源码
│   ├── src/
│   │   ├── App.tsx        # 主应用组件 + 路由
│   │   ├── main.tsx       # 入口文件
│   │   ├── pages/         # 页面组件
│   │   │   ├── Login.tsx
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Accounts.tsx
│   │   │   ├── Nodes.tsx
│   │   │   └── Settings.tsx
│   │   ├── components/    # 通用组件
│   │   │   ├── Layout.tsx
│   │   │   ├── Card.tsx
│   │   │   └── Toast.tsx
│   │   ├── hooks/         # 自定义 Hooks
│   │   │   └── useAuth.tsx
│   │   ├── services/      # API 服务层
│   │   │   └── api.ts
│   │   ├── types/         # TypeScript 类型定义
│   │   │   └── index.ts
│   │   └── styles/        # 全局样式
│   │       └── index.css
│   ├── dist/              # 构建输出（Git 忽略）
│   ├── package.json
│   └── vite.config.ts
├── web/                    # Go embed 目录
│   ├── embed.go           # Embed 声明
│   └── dist/              # 前端构建产物（复制自 frontend/dist）
└── internal/proxy/
    └── proxy.go           # SPA 文件服务器
```

## 开发流程

### 1. 前端开发

```bash
# 进入前端目录
cd frontend

# 安装依赖（首次）
npm install

# 启动开发服务器（热重载）
npm run dev

# 访问 http://localhost:5173
```

### 2. 构建前端

```bash
# 方式 1：使用脚本（推荐）
./scripts/build-frontend.sh

# 方式 2：手动构建
cd frontend
npm run build
cd ..
rm -rf web/dist
cp -R frontend/dist web/dist
```

### 3. 构建 Go 项目

```bash
# 构建二进制
go build -o cccli_bin ./cmd/cccli

# 或者直接运行
go run ./cmd/cccli proxy
```

### 4. 部署

单个二进制文件 `cccli_bin` 包含了前端和后端的所有内容，可直接部署。

```bash
# 启动代理服务器
./cccli_bin proxy
```

## 路由设计

### 客户端路由（React Router）
- `/` → 重定向到 `/admin/dashboard`
- `/login` → 登录页面
- `/admin/dashboard` → 仪表盘
- `/admin/accounts` → 账号管理（管理员）
- `/admin/nodes` → 节点管理
- `/admin/settings` → 系统配置

### 服务端路由（Go）
- **SPA 路由**：`/`, `/login`, `/admin/*`, `/assets/*` → 返回 `index.html` 或静态资源
- **API 路由**：`/admin/api/*` → JSON API
- **认证路由**：`POST /login`, `/logout` → 会话管理
- **代理路由**：其他所有路径 → 反向代理

## API 服务

前端通过 `src/services/api.ts` 与后端交互：

```typescript
// 示例：获取账号列表
const response = await api.getAccounts();
const accounts = response.accounts;

// 示例：创建节点
await api.createNode({
  name: '节点1',
  base_url: 'https://api.example.com',
  api_key: 'sk-...',
  weight: 1
});
```

所有 API 请求自动处理：
- 错误处理
- JSON 序列化/反序列化
- TypeScript 类型检查

## 认证机制

### Cookie-Based Session
- 登录成功后设置 `session_token` Cookie（HttpOnly）
- 有效期 24 小时
- 前端通过 `useAuth` Hook 管理登录状态
- 受保护路由自动检查认证状态

### 权限控制
- 普通用户：可访问 Dashboard、Nodes、Settings
- 管理员：额外可访问 Accounts 页面

## 样式系统

### CSS Variables 主题
```css
:root {
  --bg: #f8fafc;
  --card: #ffffff;
  --border: #e2e8f0;
  --text: #0f172a;
  --primary: #2563eb;
  --success: #16a34a;
  --danger: #dc2626;
  ...
}
```

### 设计特点
- 毛玻璃效果（Glassmorphism）
- 渐变背景
- 柔和阴影
- 响应式设计（移动端友好）
- 骨架屏加载状态

## 数据可视化

Dashboard 使用 Chart.js 展示：

1. **节点性能对比**（水平柱状图）
   - 绿色：>50KB/s
   - 黄色：10-50KB/s
   - 红色：<10KB/s

2. **请求分布**（环形图）
   - Top 5 节点 + "其他"
   - 显示百分比和数量

3. **自动刷新**
   - 每 6 秒自动更新数据
   - 可手动刷新

## 构建优化

### Vite 配置
- 生产构建压缩（Gzip）
- Tree-shaking 去除未使用代码
- Code splitting（按需加载）
- Asset 自动哈希（缓存优化）

### Go Embed
- 前端资源嵌入到二进制
- 单文件部署
- 无需额外文件服务器

## 依赖管理

### npm 依赖
```json
{
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^7.1.3",
    "chart.js": "^4.4.7",
    "react-chartjs-2": "^5.3.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.18",
    "@types/react-dom": "^18.3.5",
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "~5.6.2",
    "vite": "^6.0.5"
  }
}
```

### Go 依赖
```go
import (
    "io/fs"
    "qcc_plus/web"
)
```

## 故障排查

### 前端构建失败
```bash
# 清除缓存重新安装
rm -rf frontend/node_modules frontend/dist
cd frontend && npm install && npm run build
```

### Go 构建失败
```bash
# 确保前端已构建并复制到 web/dist
ls web/dist/index.html  # 应该存在

# 清除 Go 缓存
go clean -cache
go build -o cccli_bin ./cmd/cccli
```

### 页面空白
- 检查浏览器控制台错误
- 确认 `/assets/*.js` 和 `/assets/*.css` 可访问
- 检查后端日志

## 未来改进

- [ ] 添加单元测试（Jest + React Testing Library）
- [ ] 添加 E2E 测试（Playwright）
- [ ] 支持暗色主题切换
- [ ] 添加国际化（i18n）
- [ ] 性能监控（Web Vitals）
- [ ] PWA 支持

## 相关文档

- [多租户架构](./multi-tenant-architecture.md)
- [快速开始](./quick-start-multi-tenant.md)
- [健康检查机制](./health_check_mechanism.md)
