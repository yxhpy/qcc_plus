package proxy

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	// LBStrategyPriority 严格优先级（当前默认行为：最低权重优先）
	LBStrategyPriority LoadBalanceStrategy = "priority"
	// LBStrategyWeightedRandom 同优先级内加权随机
	LBStrategyWeightedRandom LoadBalanceStrategy = "weighted_random"
	// LBStrategyRoundRobin 同优先级内轮询
	LBStrategyRoundRobin LoadBalanceStrategy = "round_robin"
	// LBStrategyLeastConn 同优先级内最少连接
	LBStrategyLeastConn LoadBalanceStrategy = "least_conn"
)

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Strategy LoadBalanceStrategy // 负载均衡策略，默认 weighted_random

	// 慢节点降级
	SlowThresholdMs    int64   // 慢节点阈值（毫秒），首字节延迟超过此值视为慢节点，默认 5000
	SlowDegradeWeight  int     // 慢节点降级后的权重增量，默认 10
	SlowWindowSize     int     // 慢节点检测滑动窗口大小（请求数），默认 10
	SlowRecoverAfterMs int64   // 慢节点恢复检测间隔（毫秒），默认 60000
	SlowRateThreshold  float64 // 慢请求比例阈值（0-1），超过此比例触发降级，默认 0.5

	// 预连接保活
	PreconnectEnabled  bool          // 是否启用备用节点预连接保活，默认 true
	PreconnectInterval time.Duration // 预连接保活间隔，默认 30s
}

func loadLoadBalancerConfig() LoadBalancerConfig {
	cfg := LoadBalancerConfig{
		Strategy:           LBStrategyWeightedRandom,
		SlowThresholdMs:    5000,
		SlowDegradeWeight:  10,
		SlowWindowSize:     10,
		SlowRecoverAfterMs: 60000,
		SlowRateThreshold:  0.5,
		PreconnectEnabled:  true,
		PreconnectInterval: 30 * time.Second,
	}

	switch GetEnvString("LB_STRATEGY", "weighted_random") {
	case "priority":
		cfg.Strategy = LBStrategyPriority
	case "weighted_random":
		cfg.Strategy = LBStrategyWeightedRandom
	case "round_robin":
		cfg.Strategy = LBStrategyRoundRobin
	case "least_conn":
		cfg.Strategy = LBStrategyLeastConn
	}

	if v := GetEnvInt("LB_SLOW_THRESHOLD_MS", 5000); v > 0 {
		cfg.SlowThresholdMs = int64(v)
	}
	if v := GetEnvInt("LB_SLOW_DEGRADE_WEIGHT", 10); v > 0 {
		cfg.SlowDegradeWeight = v
	}
	if v := GetEnvInt("LB_SLOW_WINDOW_SIZE", 10); v > 0 {
		cfg.SlowWindowSize = v
	}
	if v := GetEnvInt("LB_SLOW_RECOVER_AFTER_MS", 60000); v > 0 {
		cfg.SlowRecoverAfterMs = int64(v)
	}
	cfg.PreconnectEnabled = GetEnvBool("LB_PRECONNECT_ENABLED", true)
	if v := GetEnvInt("LB_PRECONNECT_INTERVAL_SEC", 30); v > 0 {
		cfg.PreconnectInterval = time.Duration(v) * time.Second
	}

	return cfg
}

// NodeScorer 节点评分器，追踪每个节点的实时性能指标
type NodeScorer struct {
	mu sync.RWMutex

	// 每个节点的响应时间滑动窗口
	latencyWindows map[string]*latencyWindow

	// 每个节点的活跃连接数
	activeConns map[string]*int64

	// 每个节点的有效权重（可能因慢节点降级而调整）
	effectiveWeights map[string]int

	// 慢节点降级时间戳
	degradedAt map[string]time.Time

	// 轮询计数器
	rrCounter uint64

	config LoadBalancerConfig
}

// latencyWindow 响应时间滑动窗口
type latencyWindow struct {
	samples    []int64 // 首字节延迟（毫秒）
	idx        int
	full       bool
	windowSize int
}

func newLatencyWindow(size int) *latencyWindow {
	if size <= 0 {
		size = 10
	}
	return &latencyWindow{
		samples:    make([]int64, size),
		windowSize: size,
	}
}

func (w *latencyWindow) add(latencyMs int64) {
	w.samples[w.idx] = latencyMs
	w.idx = (w.idx + 1) % w.windowSize
	if w.idx == 0 {
		w.full = true
	}
}

// slowRate 返回超过阈值的请求比例
func (w *latencyWindow) slowRate(thresholdMs int64) float64 {
	count := w.windowSize
	if !w.full {
		count = w.idx
	}
	if count == 0 {
		return 0
	}
	slow := 0
	for i := 0; i < count; i++ {
		if w.samples[i] > thresholdMs {
			slow++
		}
	}
	return float64(slow) / float64(count)
}

// avgLatency 返回平均延迟
func (w *latencyWindow) avgLatency() int64 {
	count := w.windowSize
	if !w.full {
		count = w.idx
	}
	if count == 0 {
		return 0
	}
	var sum int64
	for i := 0; i < count; i++ {
		sum += w.samples[i]
	}
	return sum / int64(count)
}

// hasEnoughSamples 是否有足够的样本进行判断
func (w *latencyWindow) hasEnoughSamples() bool {
	if w.full {
		return true
	}
	// 至少需要窗口大小一半的样本
	return w.idx >= w.windowSize/2
}

// NewNodeScorer 创建节点评分器
func NewNodeScorer(cfg LoadBalancerConfig) *NodeScorer {
	return &NodeScorer{
		latencyWindows:   make(map[string]*latencyWindow),
		activeConns:      make(map[string]*int64),
		effectiveWeights: make(map[string]int),
		degradedAt:       make(map[string]time.Time),
		config:           cfg,
	}
}

// RecordLatency 记录一次请求的首字节延迟
func (ns *NodeScorer) RecordLatency(nodeID string, latencyMs int64) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	w, ok := ns.latencyWindows[nodeID]
	if !ok {
		w = newLatencyWindow(ns.config.SlowWindowSize)
		ns.latencyWindows[nodeID] = w
	}
	w.add(latencyMs)

	// 检查是否需要降级
	if w.hasEnoughSamples() && w.slowRate(ns.config.SlowThresholdMs) >= ns.config.SlowRateThreshold {
		if _, degraded := ns.degradedAt[nodeID]; !degraded {
			ns.degradedAt[nodeID] = time.Now()
		}
	}
}

// IncrActiveConn 增加活跃连接数
func (ns *NodeScorer) IncrActiveConn(nodeID string) {
	ns.mu.Lock()
	c, ok := ns.activeConns[nodeID]
	if !ok {
		var v int64
		c = &v
		ns.activeConns[nodeID] = c
	}
	ns.mu.Unlock()
	atomic.AddInt64(c, 1)
}

// DecrActiveConn 减少活跃连接数
func (ns *NodeScorer) DecrActiveConn(nodeID string) {
	ns.mu.RLock()
	c, ok := ns.activeConns[nodeID]
	ns.mu.RUnlock()
	if ok {
		atomic.AddInt64(c, -1)
	}
}

// GetActiveConns 获取活跃连接数
func (ns *NodeScorer) GetActiveConns(nodeID string) int64 {
	ns.mu.RLock()
	c, ok := ns.activeConns[nodeID]
	ns.mu.RUnlock()
	if !ok {
		return 0
	}
	return atomic.LoadInt64(c)
}

// GetEffectiveWeight 获取节点的有效权重（考虑慢节点降级）
func (ns *NodeScorer) GetEffectiveWeight(nodeID string, baseWeight int) int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	degradedTime, isDegraded := ns.degradedAt[nodeID]
	if !isDegraded {
		return baseWeight
	}

	// 检查是否已过恢复期
	if time.Since(degradedTime).Milliseconds() > ns.config.SlowRecoverAfterMs {
		// 检查最近的延迟是否已恢复
		w, ok := ns.latencyWindows[nodeID]
		if ok && w.hasEnoughSamples() && w.slowRate(ns.config.SlowThresholdMs) < ns.config.SlowRateThreshold {
			// 已恢复，需要在写锁下删除（这里只读，标记后由 RecordLatency 清理）
			return baseWeight
		}
	}

	return baseWeight + ns.config.SlowDegradeWeight
}

// TryRecoverDegraded 尝试恢复已降级的节点（在写锁下调用）
func (ns *NodeScorer) TryRecoverDegraded(nodeID string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	degradedTime, isDegraded := ns.degradedAt[nodeID]
	if !isDegraded {
		return
	}

	if time.Since(degradedTime).Milliseconds() <= ns.config.SlowRecoverAfterMs {
		return
	}

	w, ok := ns.latencyWindows[nodeID]
	if ok && w.hasEnoughSamples() && w.slowRate(ns.config.SlowThresholdMs) < ns.config.SlowRateThreshold {
		delete(ns.degradedAt, nodeID)
	}
}

// IsDegraded 检查节点是否处于降级状态
func (ns *NodeScorer) IsDegraded(nodeID string) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	_, ok := ns.degradedAt[nodeID]
	return ok
}

// RemoveNode 清理节点相关数据
func (ns *NodeScorer) RemoveNode(nodeID string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	delete(ns.latencyWindows, nodeID)
	delete(ns.activeConns, nodeID)
	delete(ns.effectiveWeights, nodeID)
	delete(ns.degradedAt, nodeID)
}

// nodeCandidate 用于负载均衡选择的候选节点
type nodeCandidate struct {
	node            *Node
	effectiveWeight int
}

// SelectNode 根据负载均衡策略从候选节点中选择一个
// candidates 必须是已过滤（非 Failed、非 Disabled、非 FailedSet、非 skipNodes）的健康节点
func (ns *NodeScorer) SelectNode(candidates []*Node, skipNodes map[string]bool) *Node {
	if len(candidates) == 0 {
		return nil
	}

	// 过滤并计算有效权重
	var filtered []nodeCandidate
	for _, n := range candidates {
		if skipNodes != nil && skipNodes[n.ID] {
			continue
		}
		ew := ns.GetEffectiveWeight(n.ID, n.Weight)
		filtered = append(filtered, nodeCandidate{node: n, effectiveWeight: ew})
	}
	if len(filtered) == 0 {
		return nil
	}

	// 按有效权重分组（找到最低有效权重的一组）
	minWeight := filtered[0].effectiveWeight
	for _, c := range filtered[1:] {
		if c.effectiveWeight < minWeight {
			minWeight = c.effectiveWeight
		}
	}

	// 收集同优先级（最低有效权重）的节点
	var samePriority []nodeCandidate
	for _, c := range filtered {
		if c.effectiveWeight == minWeight {
			samePriority = append(samePriority, c)
		}
	}

	if len(samePriority) == 1 {
		return samePriority[0].node
	}

	// 在同优先级节点中应用负载均衡策略
	switch ns.config.Strategy {
	case LBStrategyPriority:
		return ns.selectByPriority(samePriority)
	case LBStrategyRoundRobin:
		return ns.selectByRoundRobin(samePriority)
	case LBStrategyLeastConn:
		return ns.selectByLeastConn(samePriority)
	case LBStrategyWeightedRandom:
		return ns.selectByWeightedRandom(samePriority)
	default:
		return ns.selectByWeightedRandom(samePriority)
	}
}

// selectByPriority 严格优先级：按创建时间选择最早的
func (ns *NodeScorer) selectByPriority(candidates []nodeCandidate) *Node {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.node.CreatedAt.Before(best.node.CreatedAt) {
			best = c
		}
	}
	return best.node
}

// selectByWeightedRandom 加权随机：基于节点的基础权重反比进行随机选择
// 权重越低（优先级越高），被选中的概率越大
func (ns *NodeScorer) selectByWeightedRandom(candidates []nodeCandidate) *Node {
	if len(candidates) == 1 {
		return candidates[0].node
	}

	// 使用权重的倒数作为选择概率
	// 例如：权重1和权重1的两个节点各50%概率
	// 如果有延迟差异，使用延迟的倒数作为额外权重因子
	type weightedNode struct {
		node   *Node
		weight float64
	}

	var nodes []weightedNode
	var totalWeight float64

	for _, c := range candidates {
		w := 1.0 / math.Max(float64(c.effectiveWeight), 1.0)

		// 如果有延迟数据，用延迟的倒数作为额外因子
		ns.mu.RLock()
		lw, hasLatency := ns.latencyWindows[c.node.ID]
		ns.mu.RUnlock()
		if hasLatency && lw.hasEnoughSamples() {
			avg := lw.avgLatency()
			if avg > 0 {
				// 延迟越低，权重越高
				w *= 1000.0 / math.Max(float64(avg), 1.0)
			}
		}

		nodes = append(nodes, weightedNode{node: c.node, weight: w})
		totalWeight += w
	}

	// 加权随机选择
	r := rand.Float64() * totalWeight
	for _, n := range nodes {
		r -= n.weight
		if r <= 0 {
			return n.node
		}
	}
	return nodes[len(nodes)-1].node
}

// selectByRoundRobin 轮询
func (ns *NodeScorer) selectByRoundRobin(candidates []nodeCandidate) *Node {
	idx := atomic.AddUint64(&ns.rrCounter, 1) - 1
	return candidates[idx%uint64(len(candidates))].node
}

// selectByLeastConn 最少连接
func (ns *NodeScorer) selectByLeastConn(candidates []nodeCandidate) *Node {
	best := candidates[0]
	bestConns := ns.GetActiveConns(best.node.ID)

	for _, c := range candidates[1:] {
		conns := ns.GetActiveConns(c.node.ID)
		if conns < bestConns {
			best = c
			bestConns = conns
		}
	}
	return best.node
}
