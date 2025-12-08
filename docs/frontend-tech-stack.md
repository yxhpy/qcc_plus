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
├── frontend/                    # React 前端源码
│   ├── src/
│   │   ├── App.tsx             # 主应用组件 + 路由
│   │   ├── main.tsx            # 入口文件
│   │   ├── index.css           # 全局样式
│   │   ├── pages/              # 页面组件（15个）
│   │   │   ├── Login.tsx       # 登录页
│   │   │   ├── Dashboard.tsx   # 仪表盘
│   │   │   ├── Accounts.tsx    # 账号管理（管理员）
│   │   │   ├── Nodes.tsx       # 节点管理
│   │   │   ├── Monitor.tsx     # 实时监控大屏 ⭐
│   │   │   ├── MonitorShares.tsx   # 分享链接管理
│   │   │   ├── SharedMonitor.tsx   # 公开监控视图
│   │   │   ├── Settings.tsx    # 账号级配置
│   │   │   ├── SystemSettings.tsx  # 环境变量总览
│   │   │   ├── TunnelSettings.tsx  # Cloudflare Tunnel
│   │   │   ├── Notifications.tsx   # 通知管理
│   │   │   ├── ClaudeConfig.tsx    # Claude Code 快速配置 ⭐
│   │   │   ├── Pricing.tsx     # 模型定价
│   │   │   ├── Usage.tsx       # 使用量统计 ⭐
│   │   │   └── ChangelogPage.tsx   # 更新日志
│   │   ├── components/         # 通用组件（11个）
│   │   │   ├── Layout.tsx      # 主布局 + 侧边栏
│   │   │   ├── Card.tsx        # 卡片容器
│   │   │   ├── Toast.tsx       # 提示条
│   │   │   ├── Modal.tsx       # 模态框
│   │   │   ├── Dialog.tsx      # 确认对话框
│   │   │   ├── PromptDialog.tsx    # 输入提示框
│   │   │   ├── Tooltip.tsx     # 悬停提示
│   │   │   ├── Loading.tsx     # 加载占位
│   │   │   ├── NodeCard.tsx    # 监控节点卡片
│   │   │   ├── HealthTimeline.tsx  # 24h 探活时间线
│   │   │   └── TrendChart.tsx  # 趋势图表
│   │   ├── hooks/              # 自定义 Hooks（6个）
│   │   │   ├── useAuth.tsx     # 认证状态管理
│   │   │   ├── useDialog.tsx   # 对话框控制
│   │   │   ├── usePrompt.tsx   # 输入提示控制
│   │   │   ├── useMonitorWebSocket.tsx  # 监控 WebSocket
│   │   │   ├── useVersion.ts   # 版本信息
│   │   │   └── useChartColors.ts   # 图表颜色
│   │   ├── services/           # API 服务层
│   │   │   ├── api.ts          # 后端 API 聚合
│   │   │   └── settingsApi.ts  # 配置 API
│   │   ├── contexts/           # React Context
│   │   │   ├── NodeMetricsContext.tsx
│   │   │   └── SettingsContext.tsx
│   │   ├── themes/             # 主题系统
│   │   │   ├── ThemeProvider.tsx
│   │   │   ├── useTheme.ts
│   │   │   └── tokens/         # 设计令牌
│   │   ├── types/              # TypeScript 类型定义
│   │   │   └── index.ts
│   │   └── utils/              # 工具函数
│   │       └── date.ts
│   ├── dist/                   # 构建输出（Git 忽略）
│   ├── package.json
│   └── vite.config.ts
├── web/                         # Go embed 目录
│   ├── embed.go                # Embed 声明
│   └── dist/                   # 前端构建产物
└── internal/proxy/
    └── proxy.go                # SPA 文件服务器
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

**公开路由**：
- `/` → 根据登录状态跳转到 `/admin/dashboard` 或 `/login`
- `/login` → 登录页面
- `/monitor/share/:token` → 公开监控视图（只读分享）

**受保护路由**（需登录）：
| 路径 | 页面 | 说明 |
|------|------|------|
| `/admin/dashboard` | Dashboard | 仪表盘，KPI 统计和图表 |
| `/admin/nodes` | Nodes | 节点管理，拖拽排序 |
| `/admin/monitor` | Monitor | 实时监控大屏 ⭐ |
| `/admin/monitor-shares` | MonitorShares | 分享链接管理 |
| `/admin/settings` | Settings | 账号级配置 |
| `/admin/notifications` | Notifications | 通知渠道管理 |
| `/admin/claude-config` | ClaudeConfig | Claude Code 快速配置 ⭐ |
| `/admin/usage` | Usage | 使用量统计 ⭐ |
| `/changelog` | ChangelogPage | 更新日志 |

**管理员专属路由**：
| 路径 | 页面 | 说明 |
|------|------|------|
| `/admin/accounts` | Accounts | 账号管理 |
| `/settings` | SystemSettings | 环境变量总览 |
| `/admin/tunnel` | TunnelSettings | Cloudflare Tunnel 配置 |
| `/admin/pricing` | Pricing | 模型定价管理 |

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

## 页面功能详解

### 核心功能页面

| 页面 | 功能描述 |
|------|----------|
| **Dashboard** | 账号切换、节点 KPI 统计、柱状图/环形图、告警列表，6 秒自动刷新 |
| **Nodes** | 节点列表拖拽排序、添加/编辑/启停/删除、健康检查、请求统计、详情弹窗 |
| **Monitor** ⭐ | 实时监控大屏，WebSocket 驱动节点状态/流量，支持创建分享链接 |
| **ClaudeConfig** ⭐ | 生成 Claude Code CLI 配置模板，复制安装命令/下载 JSON |
| **Usage** ⭐ | 调用费用/Token 统计，按模型或节点汇总，日志明细 |

### 管理页面

| 页面 | 功能描述 |
|------|----------|
| **Accounts** | 管理员创建/编辑/删除账号，管理 Proxy API Key |
| **SystemSettings** | 按分类查看环境变量当前值/默认值，支持搜索 |
| **TunnelSettings** | Cloudflare Tunnel 配置（API Token、子域名、域名、启停） |
| **Pricing** | 模型定价管理（model_id、输入/输出价格、启用状态） |
| **Notifications** | 通知渠道 CRUD、事件订阅勾选、测试通知发送 |

### 其他页面

| 页面 | 功能描述 |
|------|----------|
| **Login** | 账号密码登录，展示版本号/构建信息 |
| **Settings** | 账号级配置（重试次数、失败阈值、健康检查间隔） |
| **SharedMonitor** | 公开只读监控页，基于 token + WebSocket 实时更新 |
| **MonitorShares** | 分页列出分享链接，创建/复制/撤销 |
| **ChangelogPage** | 拉取并渲染后端 CHANGELOG（Markdown 格式） |

## 组件清单

| 组件 | 用途 |
|------|------|
| **Layout** | 侧边栏导航、主题切换、版本显示、登出封装主布局 |
| **Card** | 统一卡片容器（可选标题/extra） |
| **Modal** | 可遮罩关闭、焦点管理的通用模态框 |
| **Dialog** | 确认对话框，基于 Modal |
| **PromptDialog** | 输入/表单式提示框，供 usePrompt 动态挂载 |
| **Toast** | 顶部提示条，success/error 状态 |
| **Tooltip** | 悬停/点击提示，支持位置与固定 |
| **Loading** | 全局加载占位 |
| **NodeCard** | 监控卡片，展示节点状态/流量/健康轨迹 |
| **HealthTimeline** | 节点 24h 探活时间线，含缓存与 WebSocket 增量更新 |
| **TrendChart** | 折线图封装（成功率/响应时间，基于 Chart.js） |

## Hooks 清单

| Hook | 用途 |
|------|------|
| **useAuth** | 认证状态管理，登录/登出/权限检查 |
| **useDialog** | 对话框控制，confirm/cancel 回调 |
| **usePrompt** | 输入提示控制，动态挂载 PromptDialog |
| **useMonitorWebSocket** | 监控 WebSocket 连接，实时节点状态推送 |
| **useVersion** | 获取后端版本信息 |
| **useChartColors** | 图表颜色配置，适配主题 |

## 相关文档

- [多租户架构](./multi-tenant-architecture.md)
- [快速开始](./quick-start-multi-tenant.md)
- [健康检查机制](./health_check_mechanism.md)
- [监控数据持久化](./monitoring-data-persistence.md)
