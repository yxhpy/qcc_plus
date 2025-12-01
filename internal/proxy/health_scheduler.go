package proxy

import (
	"log"
	"runtime"
	"sync"
	"time"
)

const (
	defaultHealthAllInterval = 10 * time.Minute
	// 默认并发 2：兼顾 2 核 / 2GB 小机型，避免同时拉起过多 CLI 进程导致 OOM。
	defaultHealthCheckConcurrency = 2
)

// HealthScheduler 定期探活所有节点（包括健康节点），避免状态盲区。
type HealthScheduler struct {
	server   *Server
	logger   *log.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	interval time.Duration
	stopOnce sync.Once
	workers  int
}

// NewHealthScheduler 创建全量健康检查调度器。
func NewHealthScheduler(server *Server, interval time.Duration, workers int, logger *log.Logger) *HealthScheduler {
	if logger == nil {
		logger = log.Default()
	}
	if interval <= 0 {
		interval = defaultHealthAllInterval
	}
	workers = normalizeHealthCheckWorkers(workers, logger)
	return &HealthScheduler{
		server:   server,
		logger:   logger,
		stopCh:   make(chan struct{}),
		interval: interval,
		workers:  workers,
	}
}

// normalizeHealthCheckWorkers 限制健康检查的并发度，避免在小规格机器上把 CLI 健康检查同时拉起过多进程。
// 策略：
//  1. 默认值 fallback 到 defaultHealthCheckConcurrency（2）。
//  2. 上限 = min(4, runtime.NumCPU()*2)。在 2C 机器上最大 4，默认 2；在 1C 上最大 2。
//  3. 低于 1 时修正为 1。
func normalizeHealthCheckWorkers(workers int, logger *log.Logger) int {
	if workers <= 0 {
		workers = defaultHealthCheckConcurrency
	}

	max := runtime.NumCPU() * 2
	if max > 4 {
		max = 4
	}
	if max < 1 {
		max = 1
	}

	if workers > max {
		if logger != nil {
			logger.Printf("[HealthScheduler] reduce health check concurrency from %d to %d to protect low-resource host", workers, max)
		}
		workers = max
	}

	if workers < 1 {
		workers = 1
	}
	return workers
}

// Start 启动定时全量健康检查。
func (h *HealthScheduler) Start() error {
	if h == nil || h.server == nil {
		return nil
	}
	if h.interval <= 0 {
		return nil
	}

	h.logger.Printf("[HealthScheduler] start full health checks every %v", h.interval)
	h.wg.Add(1)
	go h.checkLoop()
	return nil
}

// Stop 发出停止信号并等待退出，最多等待 30 秒。
func (h *HealthScheduler) Stop() {
	if h == nil {
		return
	}
	h.stopOnce.Do(func() {
		close(h.stopCh)
	})

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		h.logger.Printf("[HealthScheduler] stop timeout, exiting forcefully")
	}
}

// checkLoop 以固定间隔对所有节点进行健康检查。
func (h *HealthScheduler) checkLoop() {
	defer h.wg.Done()
	defer h.recoverPanic("check loop")

	// 延迟 30 秒后执行首次检查，避免启动时负载峰值。
	time.Sleep(30 * time.Second)
	h.checkAllNodes()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.checkAllNodes()
		}
	}
}

// checkAllNodes 遍历所有账号的所有节点执行健康检查。
func (h *HealthScheduler) checkAllNodes() {
	if h == nil || h.server == nil {
		return
	}

	start := time.Now()
	h.logger.Printf("[HealthScheduler] checking all nodes...")

	p := h.server

	// 检查是否跳过禁用节点
	skipDisabled := true // 默认跳过
	if p.settingsCache != nil {
		skipDisabled = p.settingsCache.GetBool("health.skip_disabled_nodes", true)
	}

	p.mu.RLock()
	accs := make([]*Account, 0, len(p.accountByID))
	for _, acc := range p.accountByID {
		accs = append(accs, acc)
	}
	p.mu.RUnlock()

	type checkTask struct {
		acc *Account
		id  string
	}

	tasks := make([]checkTask, 0)
	for _, acc := range accs {
		p.mu.RLock()
		ids := make([]string, 0, len(acc.Nodes))
		for id, node := range acc.Nodes {
			// 如果启用了跳过禁用节点，则跳过
			if skipDisabled && node.Disabled {
				continue
			}
			ids = append(ids, id)
		}
		p.mu.RUnlock()
		for _, id := range ids {
			tasks = append(tasks, checkTask{acc: acc, id: id})
		}
	}

	total := len(tasks)
	h.logger.Printf("[HealthScheduler] total nodes to check: %d (concurrency=%d)", total, h.workers)

	if total == 0 {
		h.logger.Printf("[HealthScheduler] full health check finished in %v (nodes=%d)", time.Since(start), total)
		return
	}

	sem := make(chan struct{}, h.workers)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t checkTask) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					h.logger.Printf("[HealthScheduler] panic in health check: %v", r)
				}
			}()

			sem <- struct{}{}
			defer func() { <-sem }()

			p.checkNodeHealth(t.acc, t.id, CheckSourceScheduled)
		}(task)
	}

	wg.Wait()
	h.logger.Printf("[HealthScheduler] full health check finished in %v (nodes=%d)", time.Since(start), total)
}

// recoverPanic 防止调度器因 panic 退出。
func (h *HealthScheduler) recoverPanic(where string) {
	if r := recover(); r != nil {
		h.logger.Printf("[HealthScheduler] panic recovered in %s: %v", where, r)
	}
}
