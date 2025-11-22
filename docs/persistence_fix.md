# 持久化数据丢失问题修复

## 问题描述

每次重启代理服务后，节点的统计数据（请求数、Token 用量、延时等）会被重置为 0。

## 根本原因

在 `internal/store/store.go` 的 `UpsertNode` 方法中，原本使用的是 `REPLACE INTO` SQL 语句：

```go
// 旧代码（有问题）
func (s *Store) UpsertNode(ctx context.Context, r NodeRecord) error {
    _, err := s.db.ExecContext(ctx, `REPLACE INTO nodes (...) VALUES (...)`, ...)
    return err
}
```

**`REPLACE INTO` 的行为**：
1. 如果主键已存在，先删除整行
2. 然后插入新行

这导致每次调用 `UpsertNode` 时，即使只是想更新配置（如 Name、BaseURL、APIKey），也会删除包含所有统计数据的旧记录。

## 触发场景

在 `internal/proxy/proxy.go:369-384` 中，每次服务启动时，如果没有活跃节点，会创建一个 "default" 节点：

```go
if srv.activeID == "" {
    node := &Node{
        ID:        "default",
        Name:      b.upstreamName,
        URL:       parsed,
        APIKey:    b.upstreamKey,
        Weight:    1,
        // 统计字段都是零值
    }
    srv.nodes[node.ID] = node
    srv.activeID = node.ID
    if st != nil {
        _ = st.UpsertNode(context.Background(), store.NodeRecord{...})  // ⚠️ 这里会重置统计数据
    }
}
```

由于传入的 `NodeRecord` 只包含配置字段，统计字段都是零值，`REPLACE INTO` 会用这些零值覆盖数据库中的历史统计数据。

## 解决方案

将 `UpsertNode` 改为使用 `INSERT ... ON DUPLICATE KEY UPDATE` 语法：

```go
// 新代码（已修复）
func (s *Store) UpsertNode(ctx context.Context, r NodeRecord) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO nodes (id, name, base_url, api_key, weight, failed, last_error, created_at,
                           requests, fail_count, fail_streak, total_bytes, total_input, total_output,
                           stream_dur_ms, first_byte_ms, last_ping_ms, last_ping_err)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
            name = VALUES(name),
            base_url = VALUES(base_url),
            api_key = VALUES(api_key),
            weight = VALUES(weight),
            failed = VALUES(failed),
            last_error = VALUES(last_error)`,
        r.ID, r.Name, r.BaseURL, r.APIKey, r.Weight, r.Failed, r.LastError, r.CreatedAt,
        r.Requests, r.FailCount, r.FailStreak, r.TotalBytes, r.TotalInput, r.TotalOutput,
        r.StreamDurMs, r.FirstByteMs, r.LastPingMs, r.LastPingErr)
    return err
}
```

**新行为**：
- **插入新节点**：如果 ID 不存在，插入完整记录（包括统计数据）
- **更新已有节点**：如果 ID 已存在，**只更新配置字段**（name、base_url、api_key、weight、failed、last_error），**保留统计字段不变**

## 更新的字段 vs 保留的字段

### 更新的字段（配置类）
- `name` - 节点名称
- `base_url` - API 地址
- `api_key` - API 密钥
- `weight` - 权重
- `failed` - 失败状态
- `last_error` - 最后错误信息

### 保留的字段（统计类）
- `created_at` - 创建时间
- `requests` - 总请求数
- `fail_count` - 失败总数
- `fail_streak` - 连续失败次数
- `total_bytes` - 总字节数
- `total_input` - 输入 Token 总数
- `total_output` - 输出 Token 总数
- `stream_dur_ms` - 流传输总时长
- `first_byte_ms` - 首字节延时总和
- `last_ping_ms` - 最后一次 Ping 延时
- `last_ping_err` - 最后一次 Ping 错误

## 验证方法

### 方法 1：运行验证脚本

启动 MySQL（如果尚未启动）：
```bash
./scripts/start_proxy_docker.sh
```

运行验证脚本：
```bash
go run ./verify/persistence/verify_upsert_preserves_stats.go
```

预期输出：
```
✅ 第一次插入成功：Requests=100, FailCount=5, TotalInput=5000, TotalOutput=3000
✅ 第二次更新成功（传入零值统计）
✅ 配置字段已正确更新：Name=Updated Node Name, Weight=20
✅ 统计字段保持不变：Requests=100, FailCount=5, TotalInput=5000, TotalOutput=3000

🎉 持久化验证通过！统计数据在更新配置时被正确保留。
```

### 方法 2：手动测试

1. 启动代理服务并添加几个节点
2. 发送一些请求，产生统计数据
3. 停止服务并重启
4. 访问管理页面 http://localhost:8000/admin
5. 确认统计数据（请求数、Token 数）仍然存在

## 影响范围

- ✅ 修复后，重启服务不会丢失节点统计数据
- ✅ 向后兼容，不影响现有功能
- ✅ 所有单元测试通过
- ✅ 不改变 API 接口

## 相关文件

- `internal/store/store.go` - UpsertNode 方法实现
- `internal/proxy/proxy.go` - 节点初始化逻辑
- `verify/persistence/verify_upsert_preserves_stats.go` - 验证脚本

## 版本

- 修复前版本：v2.0.2
- 修复后版本：v2.0.3
