# Phase 1.1: 滑窗指标与多维评分机制 - 已完成 ✅

## 任务概述
实现基于滑窗的节点性能指标收集和多维评分机制，优化节点选择策略。

## 实施时间
- 开始时间: 2025-11-28
- 完成时间: 2025-11-28
- 耗时: ~20分钟（使用 Codex Skill）

## 问题分析
### 当前问题
- 节点选择只考虑静态 `Weight` 和 `CreatedAt`
- 不考虑节点实际运行状态（成功率、延迟、负载）
- 短期波动无法反映到选择决策中

### 解决方案
实现滑窗指标收集 + 多维评分机制，让节点选择更加智能化。

## 实现内容

### 1. 新增文件
#### internal/proxy/metrics_window.go
- `MetricsWindow` 结构：环形缓冲，记录最近 N 次请求
- `Record(success bool, latencyMS int64)` - 记录请求结果
- `SuccessRate() float64` - 计算成功率
- `P95Latency() / P99Latency() float64` - 计算百分位延迟
- 线程安全（内置 mutex）

#### internal/proxy/score.go
- `CalculateScore(node, alphaErr, betaLat)` - 评分计算
- 公式：`score = weight + α×(1-成功率) + β×(P95延迟/1000)`
- 向后兼容：Window 为 nil 时回退到 Weight

### 2. 修改文件
#### internal/proxy/types.go
- `Node` 新增字段：
  - `Window *MetricsWindow` - 滑窗指标
  - `Score float64` - 当前评分
  - `windowMu sync.Mutex` - 滑窗操作锁
- `Config` 新增字段：
  - `WindowSize int` - 滑窗大小
  - `AlphaErr float64` - 错误率权重
  - `BetaLatency float64` - 延迟权重

#### internal/proxy/node_manager.go
- `addNodeWithMethod` (L67): 初始化 Window 和 Score
- `updateNode` (L163): 权重变更后重新计算 Score
- `selectBestAndActivate` (L302-324): 使用 Score 排序选择最佳节点
  - 使用 `effectiveScore()` 函数处理向后兼容
  - 评分相同时按创建时间排序

#### internal/proxy/metrics.go
- `recordMetrics` (L126-136): 每次请求后更新滑窗和评分
  - 从 Account.Config 读取滑窗配置
  - 线程安全地更新 Window 和 Score

#### internal/proxy/builder.go
- 新增环境变量读取（L234-262）：
  - `QCC_WINDOW_SIZE` - 默认 200
  - `QCC_SCORE_ALPHA_ERR` - 默认 5.0
  - `QCC_SCORE_BETA_LAT` - 默认 0.5
- 初始化每个账号的 Config

## 测试验证
### 单元测试
```bash
GOCACHE=$(pwd)/.cache/go-build go test ./internal/proxy -v
```
**结果**: ✅ 所有 15 个测试通过

### 测试覆盖
- `TestHandleConfigGetAndPut` - 验证新配置字段正确返回
- `TestAutoFailoverByWeight` - 验证评分机制下的故障切换
- `TestNodeRecoveryAutoSwitch` - 验证节点恢复后的自动切换
- 所有现有测试保持通过，确保向后兼容

## 配置说明

### 环境变量
| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `QCC_WINDOW_SIZE` | 滑窗大小（请求次数） | 200 |
| `QCC_SCORE_ALPHA_ERR` | 错误率权重（0-禁用） | 5.0 |
| `QCC_SCORE_BETA_LAT` | 延迟权重（0-禁用） | 0.5 |

### 评分公式
```
score = weight + α×(1-成功率) + β×(P95延迟/1000)
```
- **评分越低** = 优先级越高
- 设置 `α=0` 可禁用错误率惩罚
- 设置 `β=0` 可禁用延迟惩罚

### 示例场景
假设两个节点：
- **节点 A**: weight=1, 成功率=95%, P95延迟=200ms
  - score = 1 + 5.0×(1-0.95) + 0.5×(200/1000) = 1 + 0.25 + 0.1 = **1.35**

- **节点 B**: weight=2, 成功率=100%, P95延迟=50ms
  - score = 2 + 5.0×(1-1.0) + 0.5×(50/1000) = 2 + 0 + 0.025 = **2.025**

**结果**: 选择节点 A（评分更低）

## 向后兼容性
- ✅ Window 为 nil 时，评分回退到 Weight
- ✅ 所有现有测试通过
- ✅ 不影响现有部署（默认值与原逻辑一致）

## 后续优化
本实现为 **Phase 1.1**，后续优化：
- Phase 1.2: 防抖机制（冷却窗口+最小健康时间）
- Phase 1.3: 并发探活与自适应频率
- Phase 1.4: 节点切换审计日志

## 关联 Issue
- #20 - Phase 1.1: 滑窗指标与多维评分机制
- #27 - 节点切换优化总览（4阶段路线图）

## 文档更新
- ✅ CLAUDE.md - 新增环境变量说明和评分公式
- ✅ 版本更新为 v1.8.0（开发中）
