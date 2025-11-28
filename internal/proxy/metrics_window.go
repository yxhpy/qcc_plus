package proxy

import (
	"math"
	"sort"
	"sync"
)

// metricsEntry 保存一次请求结果与延迟。
type metricsEntry struct {
	success   bool
	latencyMS int64
}

// MetricsWindow 维护最近 N 次请求的滑动窗口。
type MetricsWindow struct {
	mu      sync.Mutex
	size    int
	entries []metricsEntry
	count   int // 已写入的有效条目数，<= size
	pos     int // 下一个写入位置
}

// NewMetricsWindow 构建固定大小的滑动窗口。
func NewMetricsWindow(size int) *MetricsWindow {
	if size <= 0 {
		return &MetricsWindow{size: 0}
	}
	return &MetricsWindow{
		size:    size,
		entries: make([]metricsEntry, size),
	}
}

// Record 记录一次请求结果。
func (w *MetricsWindow) Record(success bool, latencyMS int64) {
	if w == nil || w.size == 0 {
		return
	}
	w.mu.Lock()
	w.entries[w.pos] = metricsEntry{success: success, latencyMS: latencyMS}
	if w.count < w.size {
		w.count++
	}
	w.pos = (w.pos + 1) % w.size
	w.mu.Unlock()
}

// SuccessRate 返回窗口内请求的成功率；空窗口时返回 1 以避免过早惩罚。
func (w *MetricsWindow) SuccessRate() float64 {
	if w == nil || w.size == 0 {
		return 1
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.count == 0 {
		return 1
	}
	successes := 0
	for i := 0; i < w.count; i++ {
		if w.entries[i].success {
			successes++
		}
	}
	return float64(successes) / float64(w.count)
}

// P95Latency 返回 P95 延迟（毫秒）。
func (w *MetricsWindow) P95Latency() float64 {
	return w.percentile(0.95)
}

// P99Latency 返回 P99 延迟（毫秒）。
func (w *MetricsWindow) P99Latency() float64 {
	return w.percentile(0.99)
}

func (w *MetricsWindow) percentile(p float64) float64 {
	if w == nil || w.size == 0 {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.count == 0 {
		return 0
	}
	latencies := make([]int64, w.count)
	for i := 0; i < w.count; i++ {
		latencies[i] = w.entries[i].latencyMS
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	idx := int(math.Ceil(p*float64(len(latencies)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}
	return float64(latencies[idx])
}
