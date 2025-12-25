---
name: qcc-debug
description: Use for debugging, troubleshooting, and diagnosing issues in qcc_plus project
---

# QCC Plus 调试排查手册

## 请求类问题

### 400 错误

| 检查项 | 解决方案 |
|--------|----------|
| USER_HASH | 检查是否匹配账号 |
| 预热 | 尝试 `NO_WARMUP=1` 跳过预热 |
| 系统提示 | 确认使用精简系统提示 `MINIMAL_SYSTEM=1` |

### 工具定义格式错误 (tools.*.custom)

- v3.0.1+ 自动清理工具定义中的非标准字段（如 custom、input_examples）
- 代理会自动移除 Anthropic API 不支持的字段，保留 name/description/input_schema
- 如需查看清理日志，检查代理服务器输出

## 节点/连接类问题

### 代理连接失败

| 检查项 | 解决方案 |
|--------|----------|
| 上游地址 | 检查 `UPSTREAM_BASE_URL` 配置 |
| 网络 | 确认网络连通性 |
| 重试 | 查看 `PROXY_RETRY_MAX` 重试配置 |

### MySQL 连接问题

| 检查项 | 解决方案 |
|--------|----------|
| DSN 格式 | 检查 `PROXY_MYSQL_DSN` 格式 |
| 服务状态 | 确认 MySQL 服务运行状态 |
| 网络 | 检查防火墙和端口配置 |

## CI/CD 类问题

### 健康检查超时

| 检查项 | 解决方案 |
|--------|----------|
| 版本 | v1.0.1+ 已增强健康检查：10s 初始等待 + 6 次重试 |
| 端口 | 检查服务器端口和防火墙配置 |
| 日志 | 查看部署日志：`docker logs qcc_test-proxy-1` |

## 日志位置

| 组件 | 日志命令 |
|------|----------|
| Docker 容器 | `docker logs <container_name>` |
| 代理服务器 | 标准输出 |
| 健康检查 | 代理服务器日志中的 `[health]` 标签 |

## 测试环境参数

| 参数 | 值 |
|------|-----|
| 重试次数 | `PROXY_RETRY_MAX=3` |
| 单次超时 | 12/6/3s（递减） |
| 总超时 | `RETRY_TOTAL_TIMEOUT_SEC=25` |
| 重试状态码 | `RETRY_ON_STATUS=502,503,504` |
| 健康检查模式 | `PROXY_HEALTH_CHECK_MODE=cli` |
| 失败阈值 | `PROXY_FAIL_THRESHOLD=3` |
| 探活间隔 | `PROXY_HEALTH_INTERVAL_SEC=30` |

## 熔断器配置

| 参数 | 值 |
|------|-----|
| 窗口 | 120s |
| 失败率 | 0.8 |
| 连续失败 | 10 次 |
| 冷却时间 | 60s |
| 半开探测 | 5 次 |

## 常见踩坑

### 前端页面 CSS 硬编码颜色
- **现象**：新页面在深色主题下显示异常
- **原因**：直接使用硬编码颜色值而非 CSS 变量
- **解决**：使用 CSS 变量（`var(--bg)`、`var(--text)`、`var(--border)`）
- **预防**：开发新页面时禁止使用硬编码颜色

### Codex 执行卡住无响应
- **现象**：`codex exec` 运行超过 30 分钟无输出
- **原因**：网络问题或 API 超时
- **解决**：使用 `KillShell` 终止后重试，或设置 timeout
- **预防**：对于简单分析任务，直接用 Claude 而非 Codex

### Shell 转义问题导致 Codex 失败
- **现象**：直接传递含特殊字符的 prompt 导致命令解析错误
- **原因**：Shell 对引号、换行等字符的转义处理
- **解决**：写入临时文件后用 `cat file | codex exec`
- **预防**：始终使用临时文件方式调用 Codex
