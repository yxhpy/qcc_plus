package proxy

import (
	"log"
	"sync"
	"time"
)

const (
	defaultHealthAllInterval      = 10 * time.Minute
	defaultHealthCheckConcurrency = 10
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
	if workers <= 0 {
		workers = defaultHealthCheckConcurrency
	}
	return &HealthScheduler{
		server:   server,
		logger:   logger,
		stopCh:   make(chan struct{}),
		interval: interval,
		workers:  workers,
	}
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
		for id := range acc.Nodes {
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
