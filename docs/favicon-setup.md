# Favicon 设置文档

## 概述

QCC Plus 项目现已在 frontend（React）和 website（Next.js）中集成统一的品牌 favicon，使用 `website/public/qcc_plus_icon_dark.png` 作为源图标。

## 源图标

- **文件**: `website/public/qcc_plus_icon_dark.png`
- **尺寸**: 1024x1024 (原始)
- **格式**: PNG with transparency
- **设计**: 黑底白字，Q 字母为中心的量子网络节点图案

## Frontend (React + Vite)

### 生成的文件

```
frontend/public/
├── favicon.ico           # 多尺寸 ICO (16x16, 32x32, 48x48) - 446 bytes
├── qcc-icon-192.png      # PWA 图标 - 12 KB
└── qcc-icon-180.png      # Apple Touch 图标 - 11 KB
```

### HTML 配置

`frontend/index.html`:
```html
<link rel="icon" type="image/x-icon" href="/favicon.ico" />
<link rel="icon" type="image/png" sizes="192x192" href="/qcc-icon-192.png" />
<link rel="apple-touch-icon" sizes="180x180" href="/qcc-icon-180.png" />
<title>QCC Plus - Admin Dashboard</title>
```

### 构建输出

Vite 自动复制 `public/` 目录下的所有文件到 `dist/`，favicon 正确包含在构建产物中：

```
frontend/dist/
├── favicon.ico
├── qcc-icon-192.png
└── qcc-icon-180.png
```

### Go Embed

构建脚本将前端产物复制到 `web/dist/`，由 Go 的 `embed.FS` 嵌入到二进制文件中：

```
web/dist/
├── favicon.ico
├── qcc-icon-192.png
└── qcc-icon-180.png
```

## Website (Next.js 16 App Router)

### 生成的文件

```
website/public/
├── favicon.ico           # 多尺寸 ICO - 446 bytes
├── icon-16.png           # 16x16 PNG - 433 bytes
├── icon-32.png           # 32x32 PNG - 1.1 KB
├── icon-192.png          # 192x192 PNG - 12 KB
├── icon-512.png          # 512x512 PNG - 47 KB
└── apple-touch-icon.png  # 180x180 PNG - 11 KB

website/app/
└── favicon.ico           # App Router 自动识别
```

### Metadata 配置

`website/app/layout.tsx`:
```typescript
export const metadata: Metadata = {
  // ...
  icons: {
    icon: [
      { url: '/favicon.ico' },
      { url: '/icon-16.png', sizes: '16x16', type: 'image/png' },
      { url: '/icon-32.png', sizes: '32x32', type: 'image/png' },
      { url: '/icon-192.png', sizes: '192x192', type: 'image/png' },
      { url: '/icon-512.png', sizes: '512x512', type: 'image/png' },
    ],
    shortcut: '/favicon.ico',
    apple: '/apple-touch-icon.png',
  },
}
```

### Next.js 自动优化

Next.js 16 App Router 会：
1. 自动识别 `app/favicon.ico`
2. 根据 metadata 配置生成 `<link>` 标签
3. 优化图标加载顺序和缓存策略

## 生成工具

### `scripts/generate-favicon.py`

Python 脚本使用 Pillow (PIL) 库将 PNG 转换为多尺寸 ICO：

```bash
# 用法
python3 scripts/generate-favicon.py <input.png> <output.ico>

# 示例
python3 scripts/generate-favicon.py \
  website/public/qcc_plus_icon_dark.png \
  frontend/public/favicon.ico
```

**特性**:
- 高质量 LANCZOS 重采样算法
- 自动生成 16x16, 32x32, 48x48 三种尺寸
- RGBA 转 RGB（ICO 格式要求）
- 显示文件大小和尺寸信息

## 浏览器兼容性

| 浏览器/平台 | 支持的图标 | 尺寸 |
|------------|-----------|------|
| Chrome/Edge (桌面) | favicon.ico | 32x32 |
| Firefox (桌面) | favicon.ico | 16x16 |
| Safari (桌面) | favicon.ico | 32x32 |
| Chrome (移动) | icon-192.png | 192x192 |
| iOS Safari | apple-touch-icon.png | 180x180 |
| PWA 安装 | icon-192.png, icon-512.png | 192x192, 512x512 |
| 书签/快捷方式 | favicon.ico | 16x16, 32x32 |

## 文件大小

| 文件 | 大小 | 备注 |
|-----|------|------|
| favicon.ico | 446 B | 包含 3 个尺寸 |
| icon-16.png | 433 B | 最小尺寸 |
| icon-32.png | 1.1 KB | 小尺寸 |
| qcc-icon-180.png | 11 KB | Apple Touch |
| qcc-icon-192.png | 12 KB | PWA |
| icon-512.png | 47 KB | PWA 高分辨率 |

**总大小**: ~72 KB（所有尺寸）

## 更新流程

如果需要更新 favicon（例如更换 logo）：

1. **准备新图标**：
   ```bash
   # 放置新的源图标（推荐 PNG，1024x1024 或更大）
   cp new-icon.png website/public/qcc_plus_icon_dark.png
   ```

2. **生成 Frontend 图标**：
   ```bash
   python3 scripts/generate-favicon.py \
     website/public/qcc_plus_icon_dark.png \
     frontend/public/favicon.ico

   # 生成 PNG 图标
   python3 << 'EOF'
   from PIL import Image
   img = Image.open('website/public/qcc_plus_icon_dark.png')
   for size, output in [(192, 'frontend/public/qcc-icon-192.png'),
                        (180, 'frontend/public/qcc-icon-180.png')]:
       img.resize((size, size), Image.Resampling.LANCZOS).save(output, 'PNG')
   EOF
   ```

3. **生成 Website 图标**：
   ```bash
   python3 scripts/generate-favicon.py \
     website/public/qcc_plus_icon_dark.png \
     website/public/favicon.ico

   cp website/public/favicon.ico website/app/favicon.ico

   # 生成其他尺寸
   python3 << 'EOF'
   from PIL import Image
   img = Image.open('website/public/qcc_plus_icon_dark.png')
   for size, name in [(16, 'icon-16'), (32, 'icon-32'),
                      (192, 'icon-192'), (512, 'icon-512'),
                      (180, 'apple-touch-icon')]:
       img.resize((size, size), Image.Resampling.LANCZOS).save(
           f'website/public/{name}.png', 'PNG')
   EOF
   ```

4. **重新构建 Frontend**：
   ```bash
   bash scripts/build-frontend.sh
   ```

5. **提交更改**：
   ```bash
   git add frontend/public/*.{ico,png} \
           website/public/*.{ico,png} \
           website/app/favicon.ico \
           web/dist/*.{ico,png}
   git commit -m "chore: update favicon"
   git push
   ```

## 测试验证

### Frontend

1. 启动开发服务器：
   ```bash
   cd frontend
   npm run dev
   ```

2. 访问 `http://localhost:5173`

3. 检查：
   - 浏览器标签页显示 favicon
   - 开发者工具 Network 面板确认加载成功
   - 添加书签查看图标

### Website

1. 启动开发服务器：
   ```bash
   cd website
   npm run dev
   ```

2. 访问 `http://localhost:3000`

3. 检查：
   - 浏览器标签页显示 favicon
   - 查看页面源代码确认 `<link>` 标签
   - iOS Safari 添加到主屏幕测试 Apple Touch Icon

### 生产环境

1. **Frontend**：构建并部署后访问管理界面
   ```bash
   docker compose up -d
   # 访问 http://localhost:8000/admin
   ```

2. **Website**：部署 Next.js 应用后访问官网
   ```bash
   cd website
   npm run build
   npm run start
   # 访问 http://localhost:3000
   ```

## 故障排查

### Favicon 不显示

1. **清除浏览器缓存**：
   - Chrome: `Ctrl/Cmd + Shift + Delete`
   - Firefox: `Ctrl/Cmd + Shift + Delete`
   - Safari: `Cmd + Option + E`

2. **强制刷新**：
   - Chrome/Firefox: `Ctrl/Cmd + Shift + R`
   - Safari: `Cmd + R`

3. **检查文件路径**：
   ```bash
   # Frontend
   ls -lh frontend/public/favicon.ico
   ls -lh web/dist/favicon.ico

   # Website
   ls -lh website/app/favicon.ico
   ls -lh website/public/favicon.ico
   ```

4. **验证 ICO 文件**：
   ```bash
   file frontend/public/favicon.ico
   # 应该输出: MS Windows icon resource - 3 icons, 16x16, 32x32, 48x48
   ```

### 构建时丢失图标

- 确保 `frontend/public/` 中的文件存在
- 检查 `.gitignore` 没有忽略图标文件
- 重新运行 `bash scripts/build-frontend.sh`

### Next.js favicon 不更新

- 删除 `.next/` 目录并重新构建
- 检查 `app/layout.tsx` metadata 配置
- 确认 `app/favicon.ico` 存在

## 参考资源

- [Vite 静态资源处理](https://vitejs.dev/guide/assets.html)
- [Next.js Metadata API](https://nextjs.org/docs/app/api-reference/functions/generate-metadata)
- [Web App Manifest Icons](https://developer.mozilla.org/en-US/docs/Web/Manifest/icons)
- [Apple Touch Icon](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)

## 版本历史

- **v1.0.1** (2025-11-23): 初始 favicon 集成
  - 添加 frontend favicon (React + Vite)
  - 添加 website favicon (Next.js 16)
  - 创建 `generate-favicon.py` 工具
  - 更新构建流程
