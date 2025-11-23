# qcc_plus 前端 - React 管理界面

这是 qcc_plus 项目的 Web 管理界面，基于 React + TypeScript + Vite 构建。

## 技术栈

- **React 18** - UI 框架
- **TypeScript** - 类型安全
- **Vite** - 快速的构建工具
- **React Router DOM** - 客户端路由
- **Chart.js + react-chartjs-2** - 数据可视化
- **CSS Variables** - 主题系统

## 快速开始

### 开发模式

```bash
# 安装依赖
npm install

# 启动开发服务器（带热重载）
npm run dev

# 访问 http://localhost:5173
```

### 生产构建

```bash
# 构建前端资源
npm run build

# 预览构建产物
npm run preview
```

## 项目结构

```
frontend/
├── src/
│   ├── App.tsx           # 主应用组件 + 路由配置
│   ├── main.tsx          # 应用入口
│   ├── pages/            # 页面组件
│   │   ├── Login.tsx     # 登录页
│   │   ├── Dashboard.tsx # 仪表盘（数据可视化）
│   │   ├── Accounts.tsx  # 账号管理（管理员）
│   │   ├── Nodes.tsx     # 节点管理
│   │   └── Settings.tsx  # 系统配置
│   ├── components/       # 可复用组件
│   │   ├── Layout.tsx    # 布局框架
│   │   ├── Card.tsx      # 卡片组件
│   │   └── Toast.tsx     # 通知组件
│   ├── hooks/            # 自定义 Hooks
│   │   └── useAuth.tsx   # 认证状态管理
│   ├── services/         # API 服务层
│   │   └── api.ts        # 后端 API 调用
│   ├── types/            # TypeScript 类型定义
│   │   └── index.ts      # 全局类型
│   └── styles/           # 全局样式
│       └── index.css     # CSS 变量 + 主题
├── dist/                 # 构建输出（Git 忽略）
├── index.html            # HTML 模板
├── package.json          # 依赖配置
├── tsconfig.json         # TypeScript 配置
└── vite.config.ts        # Vite 配置
```

## 功能特性

### 1. 仪表盘（Dashboard）
- 实时显示节点性能指标
- 水平柱状图展示节点速度对比
- 环形图展示请求分布
- 每 6 秒自动刷新数据

### 2. 账号管理（Accounts）
- 创建、编辑、删除账号
- 管理 proxy_api_key
- 设置管理员权限
- 仅管理员可访问

### 3. 节点管理（Nodes）
- 添加、编辑、删除节点
- 配置节点权重（优先级）
- 启用/禁用节点
- 查看节点状态和统计

### 4. 系统配置（Settings）
- 查看和更新系统配置
- 修改代理参数
- 健康检查设置

## 路由设计

| 路径 | 组件 | 权限 |
|------|------|------|
| `/` | 重定向到 `/admin/dashboard` | - |
| `/login` | Login | 公开 |
| `/admin/dashboard` | Dashboard | 已登录 |
| `/admin/accounts` | Accounts | 管理员 |
| `/admin/nodes` | Nodes | 已登录 |
| `/admin/settings` | Settings | 已登录 |

## API 调用

前端通过 `src/services/api.ts` 与后端通信：

```typescript
import api from './services/api';

// 获取账号列表
const accounts = await api.getAccounts();

// 创建节点
await api.createNode({
  name: 'node-1',
  base_url: 'https://api.anthropic.com',
  api_key: 'sk-ant-xxx',
  weight: 1
});

// 更新节点
await api.updateNode('node-id', { weight: 2 });
```

所有 API 调用自动处理：
- JSON 序列化/反序列化
- 错误处理和重试
- TypeScript 类型检查

## 认证机制

- **Cookie-Based Session**：登录后设置 `session_token` Cookie（HttpOnly）
- **自动跳转**：未登录访问受保护页面自动跳转到登录页
- **权限控制**：通过 `useAuth` Hook 检查用户权限

## 样式系统

### CSS Variables 主题

```css
:root {
  --bg: #f8fafc;           /* 背景色 */
  --card: #ffffff;         /* 卡片背景 */
  --border: #e2e8f0;       /* 边框颜色 */
  --text: #0f172a;         /* 主文本 */
  --primary: #2563eb;      /* 主色调 */
  --success: #16a34a;      /* 成功状态 */
  --warning: #eab308;      /* 警告状态 */
  --danger: #dc2626;       /* 危险状态 */
  --muted: #64748b;        /* 次要文本 */
}
```

### 设计特点

- **毛玻璃效果**（Glassmorphism）
- **渐变背景**（紫蓝色渐变）
- **柔和阴影**（多层阴影）
- **响应式设计**（移动端友好）
- **骨架屏加载**（优化加载体验）

## 构建与部署

### 集成到 Go 项目

前端构建产物通过 Go embed 嵌入到二进制文件：

```bash
# 1. 构建前端（使用脚本）
./scripts/build-frontend.sh

# 2. 脚本会自动：
#    - 进入 frontend 目录
#    - 执行 npm run build
#    - 复制 dist/ 到 web/dist/

# 3. Go 构建会自动嵌入 web/dist/
go build -o cccli_bin ./cmd/cccli
```

### 手动构建步骤

```bash
# 构建前端
cd frontend
npm run build

# 复制到 web 目录
cd ..
rm -rf web/dist
cp -R frontend/dist web/dist

# 构建 Go 项目
go build -o cccli_bin ./cmd/cccli
```

## 开发建议

### 添加新页面

1. 在 `src/pages/` 创建组件
2. 在 `src/App.tsx` 添加路由
3. 在 `src/components/Layout.tsx` 添加导航链接

### 添加 API 接口

1. 在 `src/types/index.ts` 定义类型
2. 在 `src/services/api.ts` 添加方法
3. 在组件中调用 API

### 样式修改

- 全局样式：修改 `src/styles/index.css`
- 组件样式：使用内联 `style` 或 CSS-in-JS
- 主题变量：修改 `:root` CSS 变量

## 常见问题

### 开发服务器无法启动

```bash
# 清除缓存重试
rm -rf node_modules package-lock.json
npm install
npm run dev
```

### 构建失败

```bash
# 检查 TypeScript 错误
npm run build

# 查看详细错误日志
npm run build -- --debug
```

### API 请求失败

- 确认后端服务已启动（`http://localhost:8000`）
- 检查浏览器控制台的网络请求
- 确认 Cookie 设置正确

## 相关文档

- [前端技术栈详细说明](../docs/frontend-tech-stack.md)
- [项目主文档](../README.md)
- [多租户架构](../docs/multi-tenant-architecture.md)

## 许可证

MIT
