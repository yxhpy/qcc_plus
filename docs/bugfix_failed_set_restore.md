# 失败节点恢复 Bug 修复

## 问题描述
重启后，失败状态的节点不会被自动进行定时健康检查。

## 根本原因
1. 节点的 `Failed` 状态被正确持久化到数据库
2. 重启时从数据库加载节点，`Failed=true` 被正确加载
3. **但是**：失败节点没有被添加到内存中的 `FailedSet` map
4. 健康检查的 `checkFailedNodes()` 方法遍历 `FailedSet`，找不到失败节点
5. 结果：重启后失败节点永远不会被探活

## 代码位置
`internal/proxy/server.go` 第 193-226 行，`loadAccountsFromStore()` 方法中：

```go
// 加载节点
n := &Node{
    ...
    Failed: r.Failed,  // ✅ Failed 状态被加载
    ...
}
acc.Nodes[n.ID] = n    // ✅ 节点被添加到 Nodes map

// ❌ 缺失：如果节点失败，应该添加到 FailedSet
```

## 修复方案
在加载节点后，检查 `Failed` 状态，如果为 `true` 则添加到 `FailedSet`：

```go
acc.Nodes[n.ID] = n
// 重启后恢复失败节点到 FailedSet，确保健康检查能够探活这些节点
if n.Failed {
    acc.FailedSet[n.ID] = struct{}{}
}
```

## 影响范围
- **影响版本**：所有版本
- **影响场景**：任何重启操作
- **严重程度**：高 - 失败节点永远不会恢复，导致高可用性失效

## 验证方法

### 手动验证
1. 创建一个节点并让它失败（连续 3 次请求失败）
2. 确认节点状态变为 `Failed=true`
3. 重启服务器
4. 观察日志，应该看到健康检查开始探活该失败节点
5. 如果节点恢复，应该看到 "auto-switch to recovered node" 日志

### 自动化测试
现有的健康检查测试已经覆盖了基本逻辑：
```bash
cd tests/health_check_cli
go test -v
```

## 修复日期
2025-11-24

## 相关文件
- `internal/proxy/server.go` - 修复位置
- `internal/proxy/health.go` - 健康检查逻辑
- `docs/health_check_mechanism.md` - 健康检查文档
