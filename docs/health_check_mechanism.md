# 健康检查机制

最后校准：2026-04-02

## 概览

qcc_plus 通过“请求失败检测 + 失败节点探活 + 定时全量检查”三层机制维护节点可用性。

支持三种健康检查方式：

- `cli`：调用 Claude Code CLI，最贴近真实使用链路
- `api`：向上游发送 `/v1/messages` 请求
- `head`：仅检查基础连通性

当前默认全局模式：`cli`

## 机制分层

### 1. 请求失败检测

- 在真实代理请求完成后统计失败
- 连续失败次数达到阈值后，将节点标记为失败
- 失败节点会退出正常选择流程

关键配置：

- `PROXY_FAIL_THRESHOLD`
- `PROXY_RETRY_MAX`

### 2. 失败节点探活

- 定时遍历失败节点
- 按节点配置或全局配置选择 `cli/api/head`
- 探活成功后恢复节点
- 若恢复节点优先级更高，可重新成为活跃节点

关键配置：

- `PROXY_HEALTH_INTERVAL_SEC`
- `PROXY_HEALTH_CHECK_MODE`

### 3. 全量健康检查

- 周期性检查全部节点，不只检查失败节点
- 避免健康状态长期停留在旧值
- 支持并发 worker，CLI 和非 CLI 并发数分别控制

关键配置：

- `PROXY_HEALTH_CHECK_ALL_INTERVAL`
- `HEALTH_ALL_INTERVAL_MIN`：旧格式备选值
- `HEALTH_CHECK_CONCURRENCY`
- `HEALTH_CHECK_CONCURRENCY_CLI`

## 三种检查方式

### `cli`

- 最贴近真实调用链路
- 需要本地 CLI 运行环境
- 默认模型为轻量模型，可按节点覆盖
- 适合生产环境验证“代理 + CLI”完整路径

### `api`

- 直接调用 `/v1/messages`
- 适合确认 API 写入能力
- 成本高于 `head`

### `head`

- 只验证连通性
- 成本最低
- 不验证真正的消息调用能力

## 配置表

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PROXY_HEALTH_CHECK_MODE` | 全局健康检查方式：`cli/api/head` | `cli` |
| `PROXY_FAIL_THRESHOLD` | 连续失败阈值 | `3` |
| `PROXY_HEALTH_INTERVAL_SEC` | 失败节点探活间隔（秒） | `30` |
| `PROXY_HEALTH_CHECK_ALL_INTERVAL` | 全量健康检查主配置，使用 `time.ParseDuration` 格式 | `10m` |
| `HEALTH_ALL_INTERVAL_MIN` | 全量健康检查旧配置，分钟制备选值 | `10` |
| `HEALTH_CHECK_CONCURRENCY` | 全量健康检查并发数 | `2` |
| `HEALTH_CHECK_CONCURRENCY_CLI` | CLI 健康检查并发数 | `1` |
| `HEALTH_MODEL_AWARE` | 是否按节点模型做健康检查 | `0` |
| `HEALTH_VALIDATE_USAGE` | 是否校验 usage 字段 | `1` |
| `HEALTH_VALIDATE_CONTENT` | 是否校验 content 字段 | `0` |

## 行为说明

### 节点状态

- `healthy`：参与调度
- `failed`：暂不参与调度，等待探活恢复
- `disabled`：人工禁用，不参与调度

### 切换逻辑

- 优先选择权重更小的健康节点
- 恢复后的高优先级节点可自动重新接管
- 熔断器、失败状态、禁用状态会共同参与决策

## 常见建议

### 需要最真实验证

```bash
PROXY_HEALTH_CHECK_MODE=cli
```

### 需要更低探活成本

```bash
PROXY_HEALTH_CHECK_MODE=head
```

### 需要更快发现故障

```bash
PROXY_FAIL_THRESHOLD=1
PROXY_HEALTH_INTERVAL_SEC=10
PROXY_HEALTH_CHECK_ALL_INTERVAL=5m
```

## 相关实现

- `internal/proxy/health.go`
- `internal/proxy/health_scheduler.go`
- `internal/proxy/builder.go`
- `internal/proxy/envvars.go`

## 相关文档

- [多租户架构](./multi-tenant-architecture.md)
- [监控数据持久化](./monitoring-data-persistence.md)
- [API 索引](./api/INDEX.md)
