# Cloudflare Tunnel 集成指南

qcc_plus 内置了 Cloudflare Tunnel 支持，无需公网 IP 即可将本地服务暴露到互联网。

## 功能概述

- **自动隧道管理**：通过 Web 界面启动/停止 Cloudflare Tunnel
- **域名绑定**：自动配置子域名指向隧道
- **API 集成**：使用 Cloudflare API 自动创建和管理隧道
- **状态监控**：实时查看隧道运行状态

## 环境变量配置

| 变量名 | 说明 | 必需 | 默认值 |
|--------|------|------|--------|
| CF_API_TOKEN | Cloudflare API Token | ✅ | - |
| TUNNEL_SUBDOMAIN | 隧道子域名（如 `my-proxy`） | ✅ | - |
| TUNNEL_ZONE | Cloudflare Zone（域名，如 `example.com`） | ✅ | - |
| TUNNEL_ENABLED | 启用隧道功能 | ❌ | `false` |

## 前置准备

### 1. Cloudflare 账号和域名

- 注册 Cloudflare 账号：https://dash.cloudflare.com/sign-up
- 添加你的域名到 Cloudflare
- 确保域名 DNS 已指向 Cloudflare

### 2. 创建 API Token

1. 访问 Cloudflare Dashboard: https://dash.cloudflare.com/profile/api-tokens
2. 点击 "Create Token"
3. 使用 "Edit Cloudflare Tunnels" 模板或自定义权限：
   - **Account** → **Cloudflare Tunnel** → **Edit**
   - **Zone** → **DNS** → **Edit**
   - **Zone** → **Zone** → **Read**
4. 选择你的账号和域名
5. 创建并复制 Token

## 快速开始

### 方式一：通过环境变量启动

```bash
# 设置环境变量
export CF_API_TOKEN=your-cloudflare-api-token
export TUNNEL_SUBDOMAIN=my-proxy
export TUNNEL_ZONE=example.com
export TUNNEL_ENABLED=true

# 设置代理配置
export UPSTREAM_API_KEY=sk-ant-your-key

# 启动代理服务器
qccplus proxy  # 或 go run ./cmd/cccli proxy
```

启动后：
- 隧道会自动创建并启动
- 访问地址：`https://my-proxy.example.com`
- 管理界面：`https://my-proxy.example.com/admin`

### 方式二：通过 Web 界面管理

```bash
# 不启用自动启动，通过 Web 界面手动控制
export CF_API_TOKEN=your-cloudflare-api-token
export TUNNEL_SUBDOMAIN=my-proxy
export TUNNEL_ZONE=example.com
# 不设置 TUNNEL_ENABLED

# 启动服务
qccplus proxy  # 或 go run ./cmd/cccli proxy
```

然后：
1. 访问管理界面：`http://localhost:8000/admin`
2. 登录（username: `admin`, password: `admin123`）
3. 进入 "Tunnel" 页面
4. 点击 "Start Tunnel" 启动隧道

### Docker 部署

```yaml
# docker-compose.yml
version: '3.9'

services:
  proxy:
    image: yxhpy520/qcc_plus:latest
    environment:
      # 代理配置
      - UPSTREAM_API_KEY=sk-ant-your-key

      # Cloudflare Tunnel 配置
      - CF_API_TOKEN=your-cloudflare-api-token
      - TUNNEL_SUBDOMAIN=my-proxy
      - TUNNEL_ZONE=example.com
      - TUNNEL_ENABLED=true

      # 其他配置
      - PROXY_MYSQL_DSN=qcc:example@tcp(mysql:3306)/qcc_proxy?parseTime=true
    ports:
      - "8000:8000"
    depends_on:
      - mysql

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=example
      - MYSQL_DATABASE=qcc_proxy
      - MYSQL_USER=qcc
      - MYSQL_PASSWORD=example
    volumes:
      - mysql_data:/var/lib/mysql

volumes:
  mysql_data:
```

启动：
```bash
docker compose up -d
```

## 管理 API

### 获取隧道配置

```bash
# 需要先登录获取 session_token Cookie
curl http://localhost:8000/admin/api/tunnel \
  --cookie "session_token=your-session-token"
```

响应：
```json
{
  "enabled": true,
  "subdomain": "my-proxy",
  "zone": "example.com",
  "status": "running",
  "url": "https://my-proxy.example.com"
}
```

### 启动隧道

```bash
curl -X POST http://localhost:8000/admin/api/tunnel/start \
  --cookie "session_token=your-session-token" \
  -H "Content-Type: application/json"
```

### 停止隧道

```bash
curl -X POST http://localhost:8000/admin/api/tunnel/stop \
  --cookie "session_token=your-session-token" \
  -H "Content-Type: application/json"
```

### 获取可用 Zones

```bash
curl http://localhost:8000/admin/api/tunnel/zones \
  --cookie "session_token=your-session-token"
```

响应：
```json
{
  "zones": [
    {
      "id": "zone-id-1",
      "name": "example.com",
      "status": "active"
    },
    {
      "id": "zone-id-2",
      "name": "another.com",
      "status": "active"
    }
  ]
}
```

## Web 界面使用

### 1. 访问 Tunnel 页面

- URL: `http://localhost:8000/admin/tunnel`
- 需要先登录管理界面

### 2. 配置隧道

- **Subdomain**：输入子域名（如 `my-proxy`）
- **Zone**：选择你的域名（下拉列表自动加载）
- 点击 "Save" 保存配置

### 3. 启动/停止隧道

- 点击 "Start Tunnel" 启动
- 点击 "Stop Tunnel" 停止
- 查看当前状态和访问 URL

## 工作原理

1. **隧道创建**：使用 Cloudflare API 创建 Tunnel
2. **DNS 配置**：自动添加 CNAME 记录指向隧道
3. **流量路由**：
   ```
   用户请求 → Cloudflare CDN → Cloudflare Tunnel → 本地服务 (localhost:8000)
   ```
4. **自动重连**：隧道断开时自动重新连接

## 常见问题

### 1. 隧道启动失败

**问题**：点击 "Start Tunnel" 没有反应或报错

**解决**：
- 检查 `CF_API_TOKEN` 是否正确
- 确认 Token 有足够权限（Tunnel Edit + DNS Edit）
- 查看服务器日志：`docker logs qcc_plus_proxy_1`

### 2. DNS 解析失败

**问题**：访问 `https://my-proxy.example.com` 无法访问

**解决**：
- 等待 DNS 传播（可能需要几分钟）
- 使用 `dig my-proxy.example.com` 检查 DNS 记录
- 确认域名已正确添加到 Cloudflare

### 3. 隧道频繁断开

**问题**：隧道状态经常变为 "disconnected"

**解决**：
- 检查网络连接稳定性
- 查看 Cloudflare Dashboard 中的隧道状态
- 重启代理服务器

### 4. 端口冲突

**问题**：本地已有服务占用 8000 端口

**解决**：
```bash
# 修改监听端口
export LISTEN_ADDR=:8080
go run ./cmd/cccli proxy
```

然后在 Cloudflare Tunnel 配置中指向 `http://localhost:8080`

## 安全建议

1. **保护 API Token**
   - 不要将 Token 提交到 Git
   - 使用环境变量或密钥管理工具
   - 定期轮换 Token

2. **启用 HTTPS**
   - Cloudflare 自动提供免费 SSL/TLS
   - 确保代理服务器使用 HTTPS 与上游通信

3. **访问控制**
   - 修改默认登录凭证
   - 使用强密码
   - 限制管理界面访问

4. **日志监控**
   - 定期检查访问日志
   - 监控异常流量
   - 设置告警

## 最佳实践

1. **生产环境**
   - 使用专用 Cloudflare 账号
   - 为隧道创建单独的 Token
   - 启用 Cloudflare Access（可选，额外安全层）

2. **多环境部署**
   - 开发：`dev-proxy.example.com`
   - 测试：`test-proxy.example.com`
   - 生产：`proxy.example.com`

3. **监控和告警**
   - 使用 Cloudflare Analytics 监控流量
   - 设置 Uptime 监控
   - 配置故障告警

## 参考链接

- [Cloudflare Tunnel 文档](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/)
- [Cloudflare API 文档](https://developers.cloudflare.com/api/)
- [创建 API Token](https://dash.cloudflare.com/profile/api-tokens)

## 故障排查

### 查看日志

```bash
# Docker 部署
docker logs -f qcc_plus_proxy_1

# 源码运行
# 日志会输出到 stdout
```

### 测试隧道连接

```bash
# 测试本地服务
curl http://localhost:8000/admin

# 测试隧道 URL
curl https://my-proxy.example.com/admin
```

### Cloudflare Dashboard 检查

1. 登录 Cloudflare Dashboard
2. 选择你的域名
3. 进入 "Zero Trust" → "Networks" → "Tunnels"
4. 查看隧道状态和连接信息

## 下一步

- [主文档](../README.md)
- [快速开始](./quick-start-multi-tenant.md)
- [多租户架构](./multi-tenant-architecture.md)
