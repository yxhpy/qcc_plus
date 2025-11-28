package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
)

const (
	HealthCheckMethodAPI  = "api"  // POST /v1/messages
	HealthCheckMethodHEAD = "head" // HEAD 请求
	HealthCheckMethodCLI  = "cli"  // Claude Code CLI 无头模式
)

const (
	CheckSourceScheduled = "scheduled"
	CheckSourceRecovery  = "recovery"
	CheckSourceProxyFail = "proxy_fail"
)

type healthJob struct {
	acc    *Account
	nodeID string
}

// 默认健康检查方式（可被环境变量覆盖）；从 API 变更为 CLI，以便在无 HTTP 端点时也能探活。
var defaultHealthCheckMethod = HealthCheckMethodCLI

// 处理失败：计数、记录错误、熔断并尝试切换。
func (p *Server) handleFailure(nodeID string, errMsg string) {
	if errMsg == "" {
		errMsg = "unknown error"
	}
	p.mu.Lock()
	node, ok := p.nodeIndex[nodeID]
	if !ok {
		p.mu.Unlock()
		return
	}
	acc := p.nodeAccount[nodeID]
	failLimit := 3
	if acc != nil && acc.Config.FailLimit > 0 {
		failLimit = acc.Config.FailLimit
	}
	node.LastError = errMsg
	failed := node.Metrics.FailStreak >= int64(failLimit)
	failStreak := node.Metrics.FailStreak
	nodeName := node.Name
	nodeScore := node.Score
	if failed {
		node.Failed = true
		node.StableSince = time.Time{}
		if acc != nil {
			acc.FailedSet[nodeID] = struct{}{}
		}
	}
	p.mu.Unlock()

	if failed {
		p.logger.Printf("node %s marked failed: %s", nodeName, errMsg)

		if p.audit != nil && acc != nil && node != nil {
			p.audit.Add(AuditEvent{
				Ts:       time.Now(),
				Tenant:   acc.ID,
				NodeID:   nodeID,
				NodeName: nodeName,
				Type:     EvNodeFail,
				Detail:   errMsg,
				Meta: map[string]interface{}{
					"fail_streak": failStreak,
					"score":       nodeScore,
				},
			})
		}

		if p.notifyMgr != nil && acc != nil {
			p.notifyMgr.Publish(notify.Event{
				AccountID:  acc.ID,
				EventType:  notify.EventNodeFailed,
				Title:      "节点故障告警",
				Content:    fmt.Sprintf("**节点名称**: %s\n**错误信息**: %s\n**失败次数**: %d\n**时间**: %s", nodeName, errMsg, failStreak, timeutil.FormatBeijingTime(time.Now())),
				DedupKey:   node.ID,
				OccurredAt: time.Now(),
			})
		}
		// 向该账号所有 WebSocket 连接推送离线事件。
		if p.wsHub != nil && acc != nil {
			p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
				"node_id":   nodeID,
				"node_name": nodeName,
				"status":    "offline",
				"error":     errMsg,
				"timestamp": timeutil.FormatBeijingTime(time.Now()),
			})
		}
		p.selectBestAndActivate(acc, "节点故障")
	}
}

// 定时探活失败节点，采用并发 worker + 自适应回退。
func (p *Server) healthLoop() {
	if p.healthQueue == nil {
		p.healthQueue = make(chan healthJob, 100)
	}
	if p.healthStop == nil {
		p.healthStop = make(chan struct{})
	}

	concurrency := 4
	p.mu.RLock()
	if p.defaultAccount != nil && p.defaultAccount.Config.HealthConcurrency > 0 {
		concurrency = p.defaultAccount.Config.HealthConcurrency
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if p.healthWorkers > 0 {
		concurrency = p.healthWorkers
	} else {
		p.healthWorkers = concurrency
	}
	p.mu.Unlock()

	for i := 0; i < concurrency; i++ {
		go p.healthWorker()
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.enqueueDueHealthChecks()
		case <-p.healthStop:
			close(p.healthQueue)
			return
		}
	}
}

func (p *Server) enqueueDueHealthChecks() {
	if p == nil || p.healthQueue == nil {
		return
	}
	now := time.Now()

	p.mu.RLock()
	accs := make([]*Account, 0, len(p.accountByID))
	for _, acc := range p.accountByID {
		accs = append(accs, acc)
	}
	p.mu.RUnlock()

	for _, acc := range accs {
		minBackoff := 5 * time.Second
		maxBackoff := 60 * time.Second
		if acc.Config.HealthBackoffMin > 0 {
			minBackoff = acc.Config.HealthBackoffMin
		}
		if acc.Config.HealthBackoffMax > 0 {
			maxBackoff = acc.Config.HealthBackoffMax
		}

		for id := range acc.FailedSet {
			p.mu.RLock()
			n := acc.Nodes[id]
			due := time.Time{}
			if n != nil {
				due = n.LastHealthCheckDue
			}
			p.mu.RUnlock()
			if n == nil {
				continue
			}

			if due.IsZero() || now.After(due) {
				select {
				case p.healthQueue <- healthJob{acc: acc, nodeID: id}:
					p.mu.Lock()
					bo := n.HealthBackoff
					if bo == 0 {
						bo = minBackoff
					} else {
						bo *= 2
						if bo > maxBackoff {
							bo = maxBackoff
						}
					}
					n.HealthBackoff = bo
					n.LastHealthCheckDue = now.Add(bo)
					p.mu.Unlock()
				default:
					// 队列已满，等待下一轮调度
				}
			}
		}
	}
}

func (p *Server) healthWorker() {
	for job := range p.healthQueue {
		p.checkNodeHealth(job.acc, job.nodeID, CheckSourceRecovery)
	}
}

func (p *Server) checkNodeHealth(acc *Account, id string, source string) {
	if acc == nil {
		return
	}
	if strings.TrimSpace(source) == "" {
		source = CheckSourceScheduled
	}

	now := time.Now()

	// 读锁保护节点查找，复制必要字段后立即解锁，避免与删除竞争。
	p.mu.RLock()
	node := acc.Nodes[id]
	if node == nil {
		p.mu.RUnlock()
		return
	}
	nodeCopy := *node
	p.mu.RUnlock()
	if nodeCopy.AccountID == "" && acc != nil {
		nodeCopy.AccountID = acc.ID
	}

	// 根据健康检查方式设置超时时间
	method := normalizeHealthCheckMethod(nodeCopy.HealthCheckMethod)
	if method == HealthCheckMethodCLI && nodeCopy.APIKey == "" {
		// CLI 需要 API Key，缺失时自动降级为 HEAD，避免探活失败卡死。
		p.logger.Printf("health check mode cli requires api key, fallback to head for node %s", nodeCopy.Name)
		method = HealthCheckMethodHEAD
	}

	timeout := 5 * time.Second
	if method == HealthCheckMethodCLI {
		// CLI 方式需要执行外部 CLI，需要更长的超时时间
		timeout = 15 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var (
		ok      bool
		pingErr string
		latency time.Duration
	)

	switch method {
	case HealthCheckMethodAPI:
		ok, pingErr, latency = p.healthCheckViaAPI(ctx, nodeCopy)
	case HealthCheckMethodHEAD:
		ok, pingErr, latency = p.healthCheckViaHEAD(ctx, nodeCopy)
	case HealthCheckMethodCLI:
		ok, pingErr, latency = p.healthCheckViaCLI(ctx, nodeCopy)
		// 不再自动降级，保留 CLI 失败的真实错误信息，便于调试
	default:
		ok, pingErr, latency = p.healthCheckViaAPI(ctx, nodeCopy)
	}
	checkedAt := time.Now().UTC()
	p.recordHealthEvent(nodeCopy.AccountID, nodeCopy.ID, method, source, ok, latency, pingErr, checkedAt)

	var (
		rec           store.NodeRecord
		shouldPersist bool

		metricsSnapshot metrics
		nodeName        string
		nodeID          string
		nodeFailed      bool
		nodeDisabled    bool
		hasNode         bool
		wasFailed       bool
		recovered       bool
	)

	p.mu.Lock()
	n := p.nodeIndex[id]
	if n != nil {
		acc := p.nodeAccount[id]
		wasFailed = n.Failed
		n.Metrics.LastHealthCheckAt = now
		if latency > 0 {
			n.Metrics.LastPingMS = latency.Milliseconds()
		}
		if ok {
			if n.StableSince.IsZero() {
				n.StableSince = now
			}
			minHealthy := 15 * time.Second
			if acc != nil {
				switch {
				case acc.Config.MinHealthy > 0:
					minHealthy = acc.Config.MinHealthy
				case acc.Config.MinHealthy == 0:
					minHealthy = 0
				}
			}
			if minHealthy == 0 || now.Sub(n.StableSince) >= minHealthy {
				n.Failed = false
				n.LastError = ""
				n.Metrics.FailStreak = 0
				n.Metrics.LastPingErr = ""
				n.HealthBackoff = 0
				n.LastHealthCheckDue = time.Time{}
				if acc != nil {
					delete(acc.FailedSet, id)
				}
			}
			if p.store != nil {
				rec = toRecord(n)
				shouldPersist = true
			}
		} else {
			n.StableSince = time.Time{}
			if pingErr != "" {
				n.Metrics.LastPingErr = pingErr
				if p.store != nil {
					rec = toRecord(n)
					shouldPersist = true
				}
			}
		}

		hasNode = true
		nodeName = n.Name
		nodeID = n.ID
		nodeFailed = n.Failed
		nodeDisabled = n.Disabled
		metricsSnapshot = n.Metrics
		recovered = wasFailed && !n.Failed
	}
	p.mu.Unlock()
	if shouldPersist {
		_ = p.store.UpsertNode(context.Background(), rec)
	}
	if recovered {
		if p.audit != nil && acc != nil && n != nil {
			stableDur := time.Duration(0)
			if !n.StableSince.IsZero() {
				stableDur = now.Sub(n.StableSince)
			}
			p.audit.Add(AuditEvent{
				Ts:       now,
				Tenant:   acc.ID,
				NodeID:   n.ID,
				NodeName: n.Name,
				Type:     EvNodeRecover,
				Detail:   "health check passed",
				Meta: map[string]interface{}{
					"stable_duration_sec": stableDur.Seconds(),
					"latency_ms":          latency.Milliseconds(),
				},
			})
		}
		// 恢复后重新在健康节点中选择最优的一个。
		if p.notifyMgr != nil && acc != nil && n != nil {
			p.notifyMgr.Publish(notify.Event{
				AccountID:  acc.ID,
				EventType:  notify.EventNodeRecovered,
				Title:      "节点已恢复",
				Content:    fmt.Sprintf("**节点名称**: %s\n**恢复时间**: %s", n.Name, timeutil.FormatBeijingTime(time.Now())),
				DedupKey:   n.ID,
				OccurredAt: time.Now(),
			})
		}
		if p.wsHub != nil && acc != nil && n != nil {
			p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
				"node_id":   n.ID,
				"node_name": n.Name,
				"status":    "online",
				"timestamp": timeutil.FormatBeijingTime(time.Now()),
			})
		}
		p.maybePromoteRecovered(n)
	}

	if p.wsHub != nil && acc != nil && hasNode {
		healthInterval := acc.Config.HealthEvery
		if healthInterval <= 0 {
			healthInterval = p.healthEvery
		}
		health := summarizeHealth(metricsSnapshot, method, healthInterval, time.Now())
		traffic := summarizeTraffic(metricsSnapshot)

		status := "unknown"
		if nodeDisabled {
			status = "disabled"
		} else if nodeFailed || health.Status == "down" {
			status = "offline"
		} else if health.Status == "stale" {
			status = "degraded"
		} else {
			status = "online"
		}

		timestamp := timeutil.FormatBeijingTime(time.Now())
		if health.LastCheckAt != nil {
			timestamp = *health.LastCheckAt
		}

		p.wsHub.Broadcast(acc.ID, "node_metrics", map[string]interface{}{
			"node_id":   nodeID,
			"node_name": nodeName,
			"status":    status,
			"traffic":   traffic,
			"health":    health,
			"timestamp": timestamp,
		})
	}

	if !ok && p.audit != nil && acc != nil && n != nil {
		p.audit.Add(AuditEvent{
			Ts:       now,
			Tenant:   acc.ID,
			NodeID:   n.ID,
			NodeName: n.Name,
			Type:     EvHealth,
			Detail:   fmt.Sprintf("health check failed: %s", pingErr),
			Meta: map[string]interface{}{
				"method": method,
				"source": source,
			},
		})
	}
}

func (p *Server) maybePromoteRecovered(n *Node) {
	if n == nil {
		return
	}
	acc := p.nodeAccount[n.ID]
	if acc == nil {
		return
	}

	// 重新在所有健康节点中选择最佳节点，确保优先级正确。
	p.mu.RLock()
	prevActive := acc.ActiveID
	p.mu.RUnlock()

	best, err := p.selectBestAndActivate(acc, "节点恢复")
	if err != nil || best == nil {
		return
	}

	if best.ID != prevActive {
		p.logger.Printf("auto-switch to recovered node %s (weight %d)", best.Name, best.Weight)
	}
}

func normalizeHealthCheckMethod(method string) string {
	switch strings.ToLower(method) {
	case HealthCheckMethodAPI:
		return HealthCheckMethodAPI
	case HealthCheckMethodHEAD:
		return HealthCheckMethodHEAD
	case HealthCheckMethodCLI:
		return HealthCheckMethodCLI
	default:
		// 使用全局默认值，支持环境变量覆盖
		return defaultHealthCheckMethod
	}
}

func healthMethodRequiresAPIKey(method string) bool {
	m := normalizeHealthCheckMethod(method)
	return m == HealthCheckMethodAPI || m == HealthCheckMethodCLI
}

func (p *Server) healthCheckViaAPI(ctx context.Context, node Node) (bool, string, time.Duration) {
	if node.APIKey == "" {
		return false, "api health check requires api key", 0
	}
	prompt := map[string]interface{}{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}
	bodyBytes, _ := json.Marshal(prompt)
	apiURL := strings.TrimSuffix(node.URL.String(), "/") + "/v1/messages"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", node.APIKey)
	req.Header.Set("Authorization", "Bearer "+node.APIKey)

	client := &http.Client{Transport: p.healthRT, Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return false, err.Error(), latency
	}
	defer resp.Body.Close()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if ok {
		return true, "", latency
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	return false, fmt.Sprintf("status %d: %s", resp.StatusCode, string(body)), latency
}

func (p *Server) healthCheckViaHEAD(ctx context.Context, node Node) (bool, string, time.Duration) {
	client := &http.Client{Transport: p.healthRT, Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, node.URL.String(), nil)
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return false, err.Error(), latency
	}
	defer resp.Body.Close()
	ok := resp.StatusCode == http.StatusOK
	if ok {
		return true, "", latency
	}
	return false, fmt.Sprintf("status %d", resp.StatusCode), latency
}

func (p *Server) healthCheckViaCLI(ctx context.Context, node Node) (bool, string, time.Duration) {
	runner := p.cliRunner
	if runner == nil {
		runner = defaultCLIRunner
	}
	if node.APIKey == "" {
		return false, "cli health check requires api key", 0
	}
	if node.URL == nil {
		return false, "cli health check requires valid base url", 0
	}
	env := map[string]string{
		"ANTHROPIC_API_KEY":    node.APIKey,
		"ANTHROPIC_AUTH_TOKEN": chooseNonEmpty(os.Getenv("ANTHROPIC_AUTH_TOKEN"), node.APIKey),
		"ANTHROPIC_BASE_URL":   node.URL.String(),
	}

	start := time.Now()
	out, err := runner(ctx, "claude", env, "hi")
	latency := time.Since(start)
	if err != nil {
		// 不再返回 fallback 标志，直接返回错误
		return false, err.Error(), latency
	}
	if strings.TrimSpace(out) == "" {
		return false, "cli health check returned empty output", latency
	}
	return true, "", latency
}

func (p *Server) recordHealthEvent(accountID, nodeID, method, source string, success bool, latency time.Duration, errMsg string, checkTime time.Time) {
	if p == nil {
		return
	}
	if checkTime.IsZero() {
		checkTime = time.Now().UTC()
	}
	if accountID == "" {
		accountID = store.DefaultAccountID
	}
	if strings.TrimSpace(source) == "" {
		source = CheckSourceScheduled
	}
	respMs := int(latency.Milliseconds())
	if respMs < 0 {
		respMs = 0
	}

	if p.store != nil {
		rec := store.HealthCheckRecord{
			AccountID:      accountID,
			NodeID:         nodeID,
			CheckTime:      checkTime,
			Success:        success,
			ResponseTimeMs: respMs,
			ErrorMessage:   errMsg,
			CheckMethod:    method,
			CheckSource:    source,
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = p.store.InsertHealthCheck(ctx, &rec)
		}()
	}

	if p.wsHub != nil {
		payload := map[string]interface{}{
			"node_id":          nodeID,
			"check_time":       timeutil.FormatBeijingTime(checkTime),
			"success":          success,
			"response_time_ms": respMs,
			"error_message":    errMsg,
			"check_method":     method,
			"check_source":     source,
		}
		p.wsHub.Broadcast(accountID, "health_check", payload)
	}
}

type CliRunner func(ctx context.Context, image string, env map[string]string, prompt string) (string, error)

func defaultCLIRunner(ctx context.Context, image string, env map[string]string, prompt string) (string, error) {
	// image 参数保留以兼容旧接口（当前直接调用本地 claude CLI）。
	_ = image

	// 使用 -p/--print 来获取非交互式输出
	// 超时通过 context 控制（在 healthCheckViaCLI 中设置）
	args := []string{"-p", prompt}
	cmd := exec.CommandContext(ctx, "claude", args...)

	cmdEnv := os.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = cmdEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude cli failed: %w: stdout=%s stderr=%s", err, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
