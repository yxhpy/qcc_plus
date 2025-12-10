---
name: qcc-deploy
description: Use for deployment, test server connection, and Docker operations in qcc_plus project
---

# QCC Plus 部署指南

## 服务器信息

### 测试服务器
- **IP**: 43.156.77.170
- **用户**: ubuntu
- **SSH 密钥**: ~/.ssh/qcc_deploy
- **端口**: 8001
- **容器名**: qcc_test_proxy

### 生产服务器
- **IP**: 43.156.77.170
- **用户**: ubuntu
- **SSH 密钥**: ~/.ssh/qcc_deploy
- **端口**: 8000
- **容器名**: qcc_prod_proxy

## 自动部署到测试环境

```bash
# 1. 确认在 test 分支
git checkout test

# 2. 提交代码
git add .
git commit -m "fix: 描述修复内容"

# 3. 推送触发自动部署
git push origin test

# GitHub Actions 会自动：
# - 构建前端
# - 构建 Docker 镜像
# - 部署到测试服务器（端口 8001）
# - 执行健康检查
```

## 快速操作命令

### 连接服务器
```bash
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170
```

### 查看日志
```bash
# 测试环境
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170 "docker logs qcc_test_proxy -f"

# 生产环境
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170 "docker logs qcc_prod_proxy -f"
```

### 查看容器状态
```bash
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170 "docker ps -a"
```

### 重启服务
```bash
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170 "docker restart qcc_test_proxy"
```

### 进入容器
```bash
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170 "docker exec -it qcc_test_proxy sh"
```

### 查看最近错误日志
```bash
ssh -i ~/.ssh/qcc_deploy ubuntu@43.156.77.170 "docker logs qcc_test_proxy --tail 100 2>&1 | grep -i 'error\|failed\|panic'"
```

## 快速启动方式

### npm 全局安装（推荐）
```bash
npm install -g @qccplus/cli
qccplus start
```

### Docker 部署
```bash
docker compose up -d
```

### 从源码运行
```bash
UPSTREAM_BASE_URL=https://api.anthropic.com \
UPSTREAM_API_KEY=sk-ant-your-key \
go run ./cmd/cccli proxy
```

## 默认凭证（仅内存模式）

| 类型 | 默认值 |
|------|--------|
| 管理员登录 | `admin` / `admin123` |
| 默认账号 | `default` / `default123` |
| 管理界面 | http://localhost:8000/admin |

**安全警告**: 生产环境必须修改默认密码与密钥！

## 环境变量速查

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LISTEN_ADDR` | 代理监听地址 | :8000 |
| `UPSTREAM_BASE_URL` | 上游 API 地址 | https://api.anthropic.com |
| `UPSTREAM_API_KEY` | 默认上游 API Key | - |
| `PROXY_RETRY_MAX` | 重试次数 | 3 |
| `PROXY_FAIL_THRESHOLD` | 失败阈值 | 3 |
| `PROXY_HEALTH_INTERVAL_SEC` | 探活间隔（秒） | 30 |
| `PROXY_MYSQL_DSN` | MySQL 连接 | - |
| `ADMIN_API_KEY` | 管理员密钥 | admin |

## 部署架构

```
本地开发环境
    ↓ git push origin test
GitHub (test 分支)
    ↓ GitHub Actions 触发
测试服务器 (43.156.77.170:8001)
    ├── 拉取代码
    ├── 构建镜像
    ├── 重启容器
    └── 健康检查
```

## 安全提醒

- ✅ 使用 SSH 密钥而非密码认证
- ⚠️ 定期更换 SSH 密钥
- ⚠️ 不要将服务器 IP 硬编码到公开代码中
- ⚠️ 使用环境变量管理敏感配置
