# 监控数据持久化技术文档

## 概述

本文档描述 qcc_plus 项目的监控数据持久化系统，包括数据存储、多维度统计、自动聚合和清理机制。

**相关 Issue**: [#6](https://github.com/yxhpy/qcc_plus/issues/6)
**实现日期**: 2025-11-25
**版本**: v1.3.0+

## 架构设计

### 数据流程

```
请求 → recordMetrics → node_metrics_raw →
  ↓ (每小时聚合)
node_metrics_hourly →
  ↓ (每天聚合)
node_metrics_daily →
  ↓ (每月聚合)
node_metrics_monthly → (永久保留)
```

### 核心组件

1. **internal/store/metrics.go** - 数据持久化层
2. **internal/proxy/api_metrics.go** - HTTP API 层
3. **internal/proxy/scheduler.go** - 定时任务调度器
4. **internal/proxy/metrics.go** - 实时数据记录

## 数据库表结构

### 1. node_metrics_raw（原始数据表）

存储每次请求的原始监控数据，保留 7 天。

```sql
CREATE TABLE node_metrics_raw (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL,
  node_id VARCHAR(64) NOT NULL,
  ts DATETIME NOT NULL,                  -- 时间戳（UTC）
  requests_total BIGINT DEFAULT 0,       -- 总请求数（通常为1）
  requests_success BIGINT DEFAULT 0,     -- 成功请求数
  requests_failed BIGINT DEFAULT 0,      -- 失败请求数
  response_time_sum_ms BIGINT DEFAULT 0, -- 响应时间总和（毫秒）
  response_time_count BIGINT DEFAULT 0,  -- 响应时间计数
  bytes_total BIGINT DEFAULT 0,          -- 字节总数
  input_tokens_total BIGINT DEFAULT 0,   -- 输入 token 总数
  output_tokens_total BIGINT DEFAULT 0,  -- 输出 token 总数
  first_byte_time_sum_ms BIGINT DEFAULT 0,    -- 首字节时间总和
  stream_duration_sum_ms BIGINT DEFAULT 0,    -- 流持续时间总和
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  KEY idx_metrics_raw_account_node_time (account_id, node_id, ts),
  KEY idx_metrics_raw_time (ts)
);
```

**字段说明**:
- `response_time_sum_ms / response_time_count` = 平均响应时间
- `first_byte_time_sum_ms / response_time_count` = 平均首字节时间
- `stream_duration_sum_ms / response_time_count` = 平均流持续时间

### 2. node_metrics_hourly（小时聚合表）

按小时聚合的监控数据，保留 30 天。

```sql
CREATE TABLE node_metrics_hourly (
  account_id VARCHAR(64) NOT NULL,
  node_id VARCHAR(64) NOT NULL,
  bucket_start DATETIME NOT NULL,        -- 时间桶起始时间（如 2025-11-25 10:00:00）
  requests_total BIGINT DEFAULT 0,
  requests_success BIGINT DEFAULT 0,
  requests_failed BIGINT DEFAULT 0,
  response_time_sum_ms BIGINT DEFAULT 0,
  response_time_count BIGINT DEFAULT 0,
  bytes_total BIGINT DEFAULT 0,
  input_tokens_total BIGINT DEFAULT 0,
  output_tokens_total BIGINT DEFAULT 0,
  first_byte_time_sum_ms BIGINT DEFAULT 0,
  stream_duration_sum_ms BIGINT DEFAULT 0,
  PRIMARY KEY (account_id, node_id, bucket_start),
  KEY idx_metrics_hour_time (bucket_start)
);
```

**聚合规则**: 每小时的原始数据按 `DATE_FORMAT(ts, '%Y-%m-%d %H:00:00')` 分组聚合

### 3. node_metrics_daily（天聚合表）

按天聚合的监控数据，保留 1 年。

```sql
CREATE TABLE node_metrics_daily (
  account_id VARCHAR(64) NOT NULL,
  node_id VARCHAR(64) NOT NULL,
  bucket_start DATETIME NOT NULL,        -- 时间桶起始时间（如 2025-11-25 00:00:00）
  ... (字段同 hourly)
  PRIMARY KEY (account_id, node_id, bucket_start),
  KEY idx_metrics_day_time (bucket_start)
);
```

**聚合规则**: 小时数据按 `DATE(bucket_start)` 分组聚合

### 4. node_metrics_monthly（月聚合表）

按月聚合的监控数据，永久保留。

```sql
CREATE TABLE node_metrics_monthly (
  account_id VARCHAR(64) NOT NULL,
  node_id VARCHAR(64) NOT NULL,
  bucket_start DATETIME NOT NULL,        -- 时间桶起始时间（如 2025-11-01 00:00:00）
  ... (字段同 hourly)
  PRIMARY KEY (account_id, node_id, bucket_start),
  KEY idx_metrics_month_time (bucket_start)
);
```

**聚合规则**: 天数据按 `DATE_FORMAT(bucket_start, '%Y-%m-01 00:00:00')` 分组聚合

## API 接口

### 1. 查询节点监控数据

**接口**: `GET /api/nodes/:id/metrics`

**权限**: 需要登录，只能查询自己账号下的节点（管理员可查询所有节点）

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| granularity | string | 否 | raw | 数据粒度：raw/hour/day/month |
| from | string | 否 | 自动计算 | 开始时间（RFC3339 格式） |
| to | string | 否 | 当前时间 | 结束时间（RFC3339 格式） |
| limit | int | 否 | 100 | 分页限制 |
| offset | int | 否 | 0 | 分页偏移 |

**默认时间窗口**:
- `raw`: 最近 24 小时
- `hour`: 最近 7 天
- `day`: 最近 30 天
- `month`: 最近 12 个月

**示例请求**:
```bash
curl -H "Cookie: session_token=xxx" \
  "http://localhost:8000/api/nodes/n-123/metrics?granularity=hour&from=2025-11-24T00:00:00Z&to=2025-11-25T00:00:00Z"
```

**响应示例**:
```json
{
  "data": [
    {
      "timestamp": "2025-11-25T10:00:00Z",
      "requests_total": 100,
      "requests_success": 95,
      "requests_failed": 5,
      "avg_response_time_ms": 250.5,
      "bytes_total": 1024000,
      "input_tokens": 5000,
      "output_tokens": 8000,
      "avg_first_byte_ms": 50.2,
      "avg_stream_duration_ms": 200.3
    }
  ],
  "granularity": "hour",
  "from": "2025-11-24T00:00:00Z",
  "to": "2025-11-25T00:00:00Z"
}
```

### 2. 查询账号聚合数据

**接口**: `GET /api/accounts/:id/metrics`

**权限**: 需要登录，只能查询自己的账号（管理员可查询所有账号）

**查询参数**: 同节点查询接口

**功能**: 聚合账号下所有节点的监控数据

### 3. 手动触发聚合

**接口**: `POST /api/metrics/aggregate`

**权限**: 仅管理员可用

**请求体**:
```json
{
  "target": "hour",           // 聚合目标：hour/day/month
  "account_id": "account123", // 可选，为空则处理所有账号
  "from": "2025-11-24T00:00:00Z",
  "to": "2025-11-25T00:00:00Z"
}
```

**响应**:
```json
{
  "message": "aggregation completed"
}
```

### 4. 手动触发清理

**接口**: `POST /api/metrics/cleanup`

**权限**: 仅管理员可用

**请求体**:
```json
{
  "account_id": "account123"  // 可选，为空则清理所有账号
}
```

**响应**:
```json
{
  "message": "cleanup completed"
}
```

## 定时任务调度器

### 配置参数

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| METRICS_AGGREGATE_INTERVAL | 1h | 数据聚合间隔 |
| METRICS_CLEANUP_INTERVAL | 24h | 数据清理间隔 |
| METRICS_SCHEDULER_ENABLED | true | 是否启用调度器 |

### 聚合任务（每小时执行）

1. **原始 → 小时**: 聚合过去 2 小时的原始数据
2. **小时 → 天**: 聚合昨天的小时数据
3. **天 → 月**: 聚合上个月的天数据

**日志示例**:
```
[MetricsScheduler] Starting hourly aggregation...
[MetricsScheduler] Aggregation completed in 1.2s
```

### 清理任务（每天凌晨 2:00 UTC 执行）

根据数据保留策略清理过期数据：
- 原始数据：保留 7 天
- 小时数据：保留 30 天
- 天数据：保留 365 天
- 月数据：永久保留

**日志示例**:
```
[MetricsScheduler] Starting daily cleanup...
[MetricsScheduler] Cleanup completed in 0.5s
```

### 优雅关闭

调度器支持优雅关闭，超时时间为 30 秒：
```go
// 停止调度器
server.Stop()
```

## 数据流程示例

### 1. 请求处理流程

```
用户请求 → ReverseProxy → recordMetrics
           ↓
    构建 MetricsRecord {
      AccountID: "account123"
      NodeID: "n-123"
      Timestamp: 2025-11-25 10:15:30 UTC
      RequestsTotal: 1
      RequestsSuccess: 1
      ResponseTimeSumMs: 250
      ResponseTimeCount: 1
      BytesTotal: 2048
      InputTokensTotal: 100
      OutputTokensTotal: 200
    }
           ↓
    store.InsertMetrics(ctx, rec)
           ↓
    INSERT INTO node_metrics_raw ...
```

### 2. 聚合流程（每小时）

```
调度器触发 → runAggregation()
           ↓
    原始 → 小时
    SELECT ... FROM node_metrics_raw
    WHERE ts >= '2025-11-25 09:00:00'
      AND ts < '2025-11-25 11:00:00'
    GROUP BY account_id, node_id, DATE_FORMAT(ts, '%Y-%m-%d %H:00:00')
           ↓
    INSERT INTO node_metrics_hourly ...
    ON DUPLICATE KEY UPDATE ...
           ↓
    小时 → 天
    SELECT ... FROM node_metrics_hourly
    WHERE bucket_start >= '2025-11-24 00:00:00'
      AND bucket_start < '2025-11-25 00:00:00'
    GROUP BY account_id, node_id, DATE(bucket_start)
           ↓
    INSERT INTO node_metrics_daily ...
```

### 3. 查询流程

```
GET /api/nodes/n-123/metrics?granularity=hour
           ↓
    QueryMetrics(ctx, MetricsQuery{
      AccountID: "account123",
      NodeID: "n-123",
      Granularity: "hour",
      From: now - 7 days,
      To: now
    })
           ↓
    SELECT * FROM node_metrics_hourly
    WHERE account_id = 'account123'
      AND node_id = 'n-123'
      AND bucket_start >= '2025-11-18 00:00:00'
      AND bucket_start < '2025-11-25 23:59:59'
    ORDER BY bucket_start ASC
           ↓
    计算平均值并返回 JSON
```

## 性能优化

### 索引设计

1. **复合索引**: `(account_id, node_id, ts/bucket_start)` - 支持按账号和节点查询
2. **时间索引**: `(ts/bucket_start)` - 支持按时间范围查询和清理

### 查询优化

1. **分页查询**: 使用 `LIMIT` 和 `OFFSET` 避免一次返回过多数据
2. **时间窗口**: 默认时间窗口限制查询范围
3. **账号聚合**: 先取全量再在内存中聚合，避免多次数据库查询

### 存储优化

1. **数据聚合**: 原始数据通过聚合减少存储空间
2. **自动清理**: 定期删除过期数据
3. **ON DUPLICATE KEY UPDATE**: 聚合时使用 upsert 避免重复数据

## 多租户隔离

所有监控数据按 `account_id` 隔离：
- 查询时自动过滤当前账号数据
- 管理员可查询所有账号数据
- 聚合和清理支持按账号过滤

## 故障恢复

### Panic 恢复

调度器所有任务使用 `defer/recover` 防止 panic：
```go
defer func() {
  if r := recover(); r != nil {
    m.logger.Printf("[MetricsScheduler] panic in %s: %v", name, r)
  }
}()
```

### 任务失败处理

- 聚合失败：记录日志，继续下一个聚合步骤
- 清理失败：记录日志，等待下次清理

### 数据库连接

所有操作使用 context 超时控制（默认 30 秒）

## 监控指标

可通过日志监控调度器健康状态：
```bash
# 查看聚合任务
grep "MetricsScheduler" logs.txt | grep "aggregation"

# 查看清理任务
grep "MetricsScheduler" logs.txt | grep "cleanup"

# 查看错误
grep "MetricsScheduler" logs.txt | grep "failed\|panic"
```

## 使用建议

### 1. 开发环境

```bash
# 启用调度器（默认）
PROXY_MYSQL_DSN="user:pass@tcp(localhost:3306)/qcc_plus" \
go run ./cmd/cccli proxy
```

### 2. 生产环境

```bash
# 自定义聚合和清理间隔
PROXY_MYSQL_DSN="user:pass@tcp(localhost:3306)/qcc_plus" \
METRICS_AGGREGATE_INTERVAL="30m" \
METRICS_CLEANUP_INTERVAL="12h" \
go run ./cmd/cccli proxy
```

### 3. 禁用调度器

```bash
# 仅在特殊情况下禁用（如数据迁移）
METRICS_SCHEDULER_ENABLED="false" \
go run ./cmd/cccli proxy
```

### 4. 数据迁移

如需手动触发聚合：
```bash
# 聚合特定时间范围的数据
curl -X POST http://localhost:8000/api/metrics/aggregate \
  -H "Cookie: session_token=admin_token" \
  -d '{
    "target": "hour",
    "from": "2025-11-01T00:00:00Z",
    "to": "2025-11-25T00:00:00Z"
  }'
```

## 常见问题

### Q1: 为什么查询结果为空？

**A**: 检查以下几点：
1. 确认时间范围内有数据（查询 `node_metrics_raw`）
2. 检查是否选择了正确的粒度（raw/hour/day/month）
3. 确认聚合任务已执行（非 raw 粒度需要聚合）

### Q2: 聚合任务没有执行？

**A**: 检查以下几点：
1. 查看日志确认调度器已启动
2. 检查 `METRICS_SCHEDULER_ENABLED` 是否为 true
3. 确认数据库连接正常（`PROXY_MYSQL_DSN`）

### Q3: 历史数据丢失？

**A**: 检查数据保留策略：
- 原始数据超过 7 天会被自动清理
- 小时数据超过 30 天会被自动清理
- 天数据超过 365 天会被自动清理

建议定期备份月度数据。

### Q4: 如何查看聚合进度？

**A**: 查看日志或手动查询：
```sql
-- 查看小时数据最新时间
SELECT MAX(bucket_start) FROM node_metrics_hourly;

-- 查看天数据最新时间
SELECT MAX(bucket_start) FROM node_metrics_daily;

-- 查看月数据最新时间
SELECT MAX(bucket_start) FROM node_metrics_monthly;
```

## 技术债务

当前实现的局限性和未来改进方向：

1. **性能优化**
   - [ ] 考虑使用时序数据库（InfluxDB、TimescaleDB）
   - [ ] 实现数据预聚合缓存

2. **功能增强**
   - [ ] 支持 P50/P95/P99 响应时间统计
   - [ ] 支持自定义时间窗口聚合
   - [ ] 支持节点对比分析

3. **运维工具**
   - [ ] 数据备份和恢复工具
   - [ ] 数据迁移脚本
   - [ ] 监控指标可视化

## 相关文档

- [健康检查机制](./health_check_mechanism.md)
- [多租户架构](./multi-tenant-architecture.md)
- [前端技术栈](./frontend-tech-stack.md)
