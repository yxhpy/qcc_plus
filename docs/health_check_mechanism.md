# 节点健康检查机制

## 概述

qcc_plus 实现了自动故障检测和恢复机制，通过监控节点的请求状态和定期探活来确保服务可用性。

## 健康检查流程

### 1. 失败检测（被动）

系统在每次代理请求后都会检查响应状态：

```
请求代理 → 响应状态检查 → 非 200 状态计数 → 达到阈值标记失败
```

#### 触发条件
- **检测点**：每次代理请求完成后（见 `internal/proxy/handler.go` 记录 metrics 后的失败处理）
- **失败判定**：HTTP 状态码 ≠ 200
- **阈值**：连续失败次数 ≥ `PROXY_FAIL_THRESHOLD`（默认 3 次）

#### 执行逻辑
```go
// 1. 记录失败统计
node.Metrics.FailCount++      // 总失败次数 +1
node.Metrics.FailStreak++     // 连续失败次数 +1

// 2. 检查是否达到阈值
if node.Metrics.FailStreak >= failLimit {
    node.Failed = true        // 标记为失败
    p.failedSet[nodeID] = {}  // 加入失败集合
    p.selectBestAndActivate() // 切换到其他节点
}
```

**代码位置**：`internal/proxy/health.go` 中 `handleFailure` 方法

### 2. 探活恢复（主动）

系统会定期探活失败的节点，检测是否已恢复：

```
定时器触发 → 遍历失败节点 → HEAD 请求探活 → 成功则恢复
```

#### 探活配置
- **间隔**：`PROXY_HEALTH_INTERVAL_SEC`（默认 30 秒）
- **超时**：5 秒
- **方法**：
  - **有 API Key**：POST 请求到 `/v1/messages`（真实 API 端点）
  - **无 API Key**：HTTP HEAD 请求到节点的 Base URL

#### 执行逻辑
```go
// 1. 定时循环 (每 30 秒)
ticker := time.NewTicker(p.healthEvery)
for range ticker.C {
    p.checkFailedNodes()  // 检查所有失败节点
}

// 2. 对每个失败节点发送健康检查请求
if node.APIKey != "" {
    // 使用真实 API 端点 (推荐)
    payload := {
        "model": "claude-3-5-haiku-20241022",  // 最便宜的模型
        "max_tokens": 1,                        // 只生成 1 个 token
        "messages": [{"role": "user", "content": "hi"}]
    }
    req := POST("/v1/messages", payload)
    req.Header.Set("x-api-key", node.APIKey)
    req.Header.Set("anthropic-version", "2023-06-01")
} else {
    // 回退到 HEAD 请求
    req := HEAD(node.URL.String())
}

// 3. 如果响应成功 (200-299)
if resp.StatusCode >= 200 && resp.StatusCode < 300 {
    node.Failed = false           // 清除失败标记
    node.LastError = ""           // 清除错误信息
    node.Metrics.FailStreak = 0   // 重置连续失败计数
    node.Metrics.LastPingErr = "" // 清除 ping 错误
    delete(p.failedSet, id)       // 从失败集合移除

    // 4. 如果该节点权重更低（数值更小），自动切换回来
    if node.Weight < currentActiveNode.Weight {
        p.activeID = node.ID
        log.Printf("auto-switch to recovered node %s", node.Name)
    }
}
```

**代码位置**：
- 定时循环：`internal/proxy/health.go` (`healthLoop` 方法)
- 探活逻辑：`internal/proxy/health.go` (`checkNodeHealth` 方法)
- 自动切换：`internal/proxy/health.go` (`maybePromoteRecovered` 方法)

### 3. 手动 Ping（已下线）

2025-11-22 起移除管理页按钮与 `/admin/api/ping` 端点，健康率展示依赖自动探活与请求统计，无需手动触发。

## 节点状态转换

```
┌─────────┐  连续失败 >= 阈值   ┌─────────┐
│ 健康状态 │ ─────────────────> │ 失败状态 │
│ (Failed=│                     │ (Failed=│
│  false) │ <───────────────── │  true)  │
└─────────┘   探活成功 (200 OK) └─────────┘
```

### 状态说明

| 状态 | Failed | 行为 |
|------|--------|------|
| **健康** | false | 可以被选择为活跃节点 |
| **失败** | true | 跳过选择，定期探活 |
| **禁用** | disabled=true | 跳过选择，不探活 |

**注意**：
- `Failed` 是自动管理的（系统检测）
- `Disabled` 是手动管理的（用户操作）
- 两者都会阻止节点被选择使用

## 健康率计算

健康率显示在管理页面的"健康率"列：

```go
healthRate = (总请求数 - 失败次数) / 总请求数 * 100%
```

**示例**：
- 总请求：100 次
- 失败次数：5 次
- 健康率：(100 - 5) / 100 = 95%

**颜色标识**：
- 绿色：≥ 90%
- 黄色：≥ 70% 且 < 90%
- 红色：< 70%

## 自动故障切换

**权重语义**：权重值越小，优先级越高。例如：weight=1 的节点优先于 weight=2 的节点。

当活跃节点失败时，系统自动切换到最佳节点：

### 切换逻辑
```go
// 1. 选择权重值最小（优先级最高）的健康节点
for id, node := range nodes {
    if node.Failed || node.Disabled {
        continue  // 跳过失败或禁用的节点
    }
    if node.Weight < bestNode.Weight {
        bestNode = node
    }
}

// 2. 激活最佳节点
p.activeID = bestNode.ID
```

### 自动恢复切换

当失败节点探活成功后，如果其权重（数值）低于当前活跃节点，会自动切换回来：

```go
if recoveredNode.Weight < currentActiveNode.Weight {
    p.activeID = recoveredNode.ID
    log.Printf("auto-switch to recovered node %s", recoveredNode.Name)
}
```

**代码位置**：`internal/proxy/health.go` (`maybePromoteRecovered` 方法)

## 配置参数

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PROXY_FAIL_THRESHOLD` | 连续失败多少次标记为失败 | 3 |
| `PROXY_HEALTH_INTERVAL_SEC` | 探活间隔（秒） | 30 |
| `PROXY_RETRY_MAX` | 非 200 状态重试次数 | 3 |

### 示例配置

**快速故障切换**（敏感模式）：
```bash
PROXY_FAIL_THRESHOLD=1           # 1 次失败即切换
PROXY_HEALTH_INTERVAL_SEC=10     # 每 10 秒探活
```

**稳定优先**（容错模式）：
```bash
PROXY_FAIL_THRESHOLD=5           # 5 次失败才切换
PROXY_HEALTH_INTERVAL_SEC=60     # 每分钟探活
```

## 监控指标

管理页面显示以下健康相关指标：

### 节点级别
- **状态**：Active / Failed / Disabled
- **健康率**：成功率百分比
- **Ping 延时**：最后一次 Ping 测试的延时（ms）
- **失败次数**：总失败次数
- **最后错误**：最后一次失败的错误信息

### 全局统计
- **总节点数**：所有节点数量
- **活跃节点数**：当前健康且活跃的节点数
- **平均健康率**：所有节点的平均健康率
- **总请求数**：所有节点的请求总和

## 常见问题

### Q: 为什么节点标记为失败？
A: 连续 3 次（默认）请求返回非 200 状态，系统会自动标记为失败。检查：
- 节点的 Base URL 是否正确
- API Key 是否有效
- 网络连接是否正常
- 查看"最后错误"列的具体错误信息

### Q: 失败节点何时恢复？
A: 系统每 30 秒（默认）探活一次失败节点，如果 HEAD 请求返回 200 OK，立即恢复。

### Q: 可以手动恢复失败节点吗？
A: 失败节点会自动探活恢复，无需手动操作。如果需要强制使用，可以：
1. 点击"切换"按钮手动激活该节点
2. 或等待自动探活恢复

### Q: 禁用和失败有什么区别？
A:
- **Failed（失败）**：系统自动检测，自动恢复
- **Disabled（禁用）**：用户手动操作，不会自动恢复，不参与探活

### Q: 如何调整健康检查敏感度？
A: 修改环境变量：
- 调低 `PROXY_FAIL_THRESHOLD` → 更快切换
- 调高 `PROXY_FAIL_THRESHOLD` → 更容错
- 调低 `PROXY_HEALTH_INTERVAL_SEC` → 更快恢复
- 调高 `PROXY_HEALTH_INTERVAL_SEC` → 降低探活开销

## 最佳实践

1. **合理设置失败阈值**：根据节点稳定性调整 `PROXY_FAIL_THRESHOLD`
2. **监控健康率**：健康率低于 90% 的节点需要检查
3. **配置多个节点**：至少配置 2 个节点以实现高可用
4. **使用权重控制**：主节点设置低权重值（如 1），备份节点设置高权重值（如 10）
5. **定期查看日志**：关注故障切换和恢复的日志消息

## 相关代码

- 失败检测：`internal/proxy/health.go` `handleFailure`
- 探活循环：`internal/proxy/health.go` `healthLoop`
- 健康检查：`internal/proxy/health.go` `checkNodeHealth`
- 自动切换：`internal/proxy/health.go` `maybePromoteRecovered`
