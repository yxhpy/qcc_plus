# 健康检查改进说明（v2.1.0）

## 改进背景

### 之前的问题
旧版本使用 HTTP HEAD 请求到节点的 Base URL（如 `https://api.anthropic.com`）：

```bash
$ curl -I https://api.anthropic.com
HTTP/2 404 Not Found
```

**问题**：
- Anthropic API 的根路径返回 404
- 即使 API 服务正常，健康检查也会失败
- 无法验证 API Key 是否有效
- 无法验证 API 端点是否真正可用

## 改进方案

### 使用真实 API 端点

现在健康检查会根据节点配置选择不同的检查方式：

#### 方式 1：真实 API 请求（有 API Key 时）

```http
POST /v1/messages HTTP/1.1
Host: api.anthropic.com
Content-Type: application/json
x-api-key: sk-...
anthropic-version: 2023-06-01

{
  "model": "claude-3-5-haiku-20241022",
  "max_tokens": 1,
  "messages": [
    {"role": "user", "content": "hi"}
  ]
}
```

**优点**：
✅ 验证 API 服务真正可用
✅ 验证 API Key 有效性
✅ 验证网络连接正常
✅ 测试完整的请求链路
✅ 成本极低（Haiku + 1 token）

#### 方式 2：HEAD 请求（无 API Key 时）

```http
HEAD / HTTP/1.1
Host: api.anthropic.com
```

**作为回退方案**：
- 当节点没有配置 API Key 时使用
- 适合非 API 服务的健康检查
- 轻量级，无额外成本

## 实现细节

### 请求参数

```go
payload := map[string]interface{}{
    "model":      "claude-3-5-haiku-20241022",  // 最便宜的模型
    "max_tokens": 1,                             // 只生成 1 个 token
    "messages": []map[string]string{
        {"role": "user", "content": "hi"},      // 最简单的消息
    },
}
```

**成本计算**（每次健康检查）：
- 输入：~10 tokens
- 输出：1 token
- 总计：~11 tokens ≈ $0.00001（按 Haiku 定价）
- 每天探活：(86400 / 30) × 11 tokens ≈ 31,680 tokens/天 ≈ $0.03/天

### 成功判定

```go
// 成功：200-299 状态码
ok := resp.StatusCode >= 200 && resp.StatusCode < 300
```

支持的成功状态码：
- `200 OK` - 正常响应
- `201 Created` - 创建成功
- 其他 2xx 状态码

### 错误记录

失败时记录详细错误信息：

```go
if !ok {
    body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
    pingErr = fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))
}
```

**示例错误**：
```
status 401: {"error": {"type": "authentication_error", "message": "invalid x-api-key"}}
status 429: {"error": {"type": "rate_limit_error", "message": "rate limit exceeded"}}
status 500: {"error": {"type": "api_error", "message": "internal server error"}}
```

## 影响范围

### 1. 定时探活（自动）
- `healthLoop()` → `checkFailedNodes()` → `checkNodeHealth()`
- 每 30 秒检查失败的节点
- 使用真实 API 请求验证恢复状态

### 2. 手动 Ping（已下线）
- 2025-11-22 起移除管理页按钮与 `/admin/api/ping` 接口
- 仅保留自动定时探活与请求后被动检测

### 3. 不影响正常代理请求
- 代理请求仍然使用原有逻辑
- 健康检查独立运行

## 配置建议

### 有 API Key 的节点（推荐）
```env
UPSTREAM_BASE_URL=https://api.anthropic.com
UPSTREAM_API_KEY=sk-ant-...
PROXY_HEALTH_INTERVAL_SEC=30
```

**特点**：
- 使用真实 API 端点检查
- 准确验证服务可用性
- 每天成本约 $0.03

### 无 API Key 的节点
```env
UPSTREAM_BASE_URL=https://api.anthropic.com
# UPSTREAM_API_KEY 留空
PROXY_HEALTH_INTERVAL_SEC=30
```

**特点**：
- 回退到 HEAD 请求
- 无法验证 API 可用性
- 适合网络连通性检查

### 调整探活频率

**频繁检查**（快速恢复）：
```env
PROXY_HEALTH_INTERVAL_SEC=10  # 每 10 秒
```
成本：~$0.09/天

**标准检查**（平衡）：
```env
PROXY_HEALTH_INTERVAL_SEC=30  # 每 30 秒（默认）
```
成本：~$0.03/天

**低频检查**（节省成本）：
```env
PROXY_HEALTH_INTERVAL_SEC=120  # 每 2 分钟
```
成本：~$0.01/天

## 对比总结

| 检查方式 | 旧版本 (HEAD) | 新版本 (API) |
|---------|--------------|-------------|
| **请求目标** | Base URL | `/v1/messages` |
| **验证内容** | 网络连通性 | API 服务可用性 |
| **验证 API Key** | ❌ | ✅ |
| **准确性** | 低（易误判） | 高（真实测试） |
| **成本** | 免费 | ~$0.03/天/节点 |
| **超时** | 5 秒 | 5 秒 |
| **回退方案** | - | 无 Key 时使用 HEAD |

## 最佳实践

1. **务必配置 API Key**：确保健康检查使用真实 API 端点
2. **合理设置探活间隔**：默认 30 秒适合大多数场景
3. **监控健康率**：通过管理页面观察节点稳定性
4. **查看错误详情**：检查"最后错误"列了解失败原因
5. **成本控制**：每个节点每天约 $0.03，多节点可调低频率

## 版本信息
- 改进版本：v2.1.0
- 改进日期：2025-11-22
- 影响模块：定时探活（手动 Ping 已下线）
