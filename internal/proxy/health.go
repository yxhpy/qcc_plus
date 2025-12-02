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
	HealthCheckMethodAPI   = "api"   // POST /v1/messages
	HealthCheckMethodHEAD  = "head"  // HEAD 请求
	HealthCheckMethodCLI   = "cli"   // Claude Code CLI 无头模式
	HealthCheckMethodProxy = "proxy" // 通过代理请求的健康信号
)

// 默认 CLI 健康检查模型（低成本）。
const defaultHealthCheckModel = "claude-haiku-4-5-20251001"

const (
	CheckSourceScheduled = "scheduled"
	CheckSourceRecovery  = "recovery"
	CheckSourceProxyFail = "proxy_fail"
)

// 相同状态的健康检查最小持久化间隔，避免历史记录过于冗余。
const sameStateRecordGap = 5 * time.Minute

// 默认健康检查方式（可被环境变量覆盖）；从 API 变更为 CLI，以便在无 HTTP 端点时也能探活。
var defaultHealthCheckMethod = HealthCheckMethodCLI

// shouldFail increments fail streak and returns true when the threshold is reached.
func (p *Server) shouldFail(nodeID, errMsg string) bool {
	if errMsg == "" {
		errMsg = "unknown error"
	}

	p.mu.Lock()
	node, ok := p.nodeIndex[nodeID]
	if !ok {
		p.mu.Unlock()
		return false
	}

	node.Metrics.FailStreak++
	node.LastError = errMsg

	failStreak := node.Metrics.FailStreak
	failLimit := int64(p.failLimit)
	if failLimit < 1 {
		failLimit = 1
	}
	nodeName := node.Name
	p.mu.Unlock()

	if failStreak < failLimit {
		p.logger.Printf("[health] node %s failed (%d/%d): %s", nodeName, failStreak, failLimit, errMsg)
		return false
	}

	p.logger.Printf("[health] node %s reached failure threshold (%d/%d), switching...", nodeName, failStreak, failLimit)
	return true
}

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
	node.LastError = errMsg
	if node.Metrics.FailStreak == 0 {
		node.Metrics.FailStreak = 1
	}
	node.Failed = true
	if acc != nil {
		acc.FailedSet[nodeID] = struct{}{}
	}

	var rec store.NodeRecord
	if p.store != nil {
		rec = toRecord(node)
	}
	failStreak := node.Metrics.FailStreak
	failLimit := p.failLimit
	nodeName := node.Name
	p.mu.Unlock()

	if p.store != nil {
		// 同步持久化，确保状态一致
		_ = p.store.UpsertNode(context.Background(), rec)
	}

	p.logger.Printf("node %s marked failed (fail_streak=%d, fail_limit=%d): %s", nodeName, failStreak, failLimit, errMsg)
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
	// 立即触发节点切换，避免继续使用故障节点
	p.selectBestAndActivate(acc, "节点故障")
}

// 定时探活失败节点。
func (p *Server) healthLoop() {
	for {
		interval := p.healthInterval()
		if interval <= 0 {
			return
		}
		time.Sleep(interval)
		p.checkFailedNodes()
	}
}

func (p *Server) healthInterval() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.accountByID) == 0 {
		return 0
	}
	min := time.Duration(0)
	for _, acc := range p.accountByID {
		if acc.Config.HealthEvery <= 0 {
			continue
		}
		if min == 0 || acc.Config.HealthEvery < min {
			min = acc.Config.HealthEvery
		}
	}
	return min
}

func (p *Server) checkFailedNodes() {
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
	for _, acc := range accs {
		p.mu.RLock()
		ids := make([]string, 0, len(acc.FailedSet))
		for id := range acc.FailedSet {
			// 如果启用了跳过禁用节点，检查节点是否禁用
			if skipDisabled {
				if node := acc.Nodes[id]; node != nil && node.Disabled {
					continue
				}
			}
			ids = append(ids, id)
		}
		p.mu.RUnlock()
		for _, id := range ids {
			p.checkNodeHealth(acc, id, CheckSourceRecovery)
		}
	}
}

func (p *Server) checkNodeHealth(acc *Account, id string, source string) {
	if acc == nil {
		return
	}
	if strings.TrimSpace(source) == "" {
		source = CheckSourceScheduled
	}
	isWarmup := strings.EqualFold(source, "warmup")

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
		// CLI 方式需要执行外部 CLI，需要更长的超时时间（进程+模型加载开销）
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
		activeID        string
		activeWeight    int
	)

	// 默认使用最大 int 作为哨兵值，表示“无活跃节点”
	activeWeight = int(^uint(0) >> 1)

	p.mu.Lock()
	n := p.nodeIndex[id]
	if n != nil {
		acc := p.nodeAccount[id]
		if acc != nil {
			activeID = acc.ActiveID
			if active := acc.Nodes[activeID]; active != nil {
				activeWeight = active.Weight
			}
		}
		wasFailed = n.Failed
		n.Metrics.LastHealthCheckAt = now
		if latency > 0 {
			n.Metrics.LastPingMS = latency.Milliseconds()
		}
		if ok {
			n.Failed = false
			n.LastError = ""
			n.Metrics.FailStreak = 0
			n.Metrics.LastPingErr = ""
			if acc != nil {
				delete(acc.FailedSet, id)
			}
			// 恢复后同步清理熔断器状态，避免 Open/Half-Open 残留
			if p.cbConfig.Enabled {
				if cb := p.getOrCreateCircuitBreaker(id); cb != nil {
					cb.Reset()
				}
			}
			if p.store != nil {
				rec = toRecord(n)
				shouldPersist = true
			}
		} else {
			n.Metrics.LastPingErr = pingErr
			n.LastError = pingErr
			n.Failed = true
			if n.Metrics.FailStreak == 0 {
				n.Metrics.FailStreak = 1
			}
			if acc != nil {
				acc.FailedSet[id] = struct{}{}
			}
			if p.store != nil {
				rec = toRecord(n)
				shouldPersist = true
			}
			// 如果是当前活跃节点且健康检查失败，立即触发节点切换
			if acc != nil && id == acc.ActiveID {
				// 异步触发节点切换（避免死锁，因为当前持有 p.mu 锁）
				if !isWarmup {
					go func(account *Account, reason string) {
						p.selectBestAndActivate(account, reason)
					}(acc, fmt.Sprintf("健康检查失败: %s", pingErr))
				}
				// 发送节点离线通知
				if p.notifyMgr != nil {
					p.notifyMgr.Publish(notify.Event{
						AccountID:  acc.ID,
						EventType:  notify.EventNodeFailed,
						Title:      "节点健康检查失败",
						Content:    fmt.Sprintf("**节点名称**: %s\n**错误信息**: %s\n**检测时间**: %s", n.Name, pingErr, timeutil.FormatBeijingTime(time.Now())),
						DedupKey:   n.ID,
						OccurredAt: time.Now(),
					})
				}
				// 发送 WebSocket 事件
				if p.wsHub != nil {
					p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
						"node_id":   n.ID,
						"node_name": n.Name,
						"status":    "offline",
						"error":     pingErr,
						"timestamp": timeutil.FormatBeijingTime(time.Now()),
					})
				}
			}
		}

		hasNode = true
		nodeName = n.Name
		nodeID = n.ID
		nodeFailed = n.Failed
		nodeDisabled = n.Disabled
		metricsSnapshot = n.Metrics
	}
	p.mu.Unlock()

	if !ok && hasNode {
		p.logger.Printf("health check failed for node %s: %s", nodeName, pingErr)
	}
	if shouldPersist {
		_ = p.store.UpsertNode(context.Background(), rec)
	}
	shouldPromote := ok && n != nil && !nodeDisabled &&
		(wasFailed || activeID == "" || n.Weight < activeWeight)

	if ok && wasFailed && !isWarmup {
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
	}

	if shouldPromote && !isWarmup {
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

	model := node.HealthCheckModel
	if model == "" {
		model = defaultHealthCheckModel
	}
	env := map[string]string{
		"ANTHROPIC_API_KEY":    node.APIKey,
		"ANTHROPIC_AUTH_TOKEN": chooseNonEmpty(os.Getenv("ANTHROPIC_AUTH_TOKEN"), node.APIKey),
		"ANTHROPIC_BASE_URL":   node.URL.String(),
	}

	start := time.Now()
	// 使用简短的 prompt 让模型只回复 "ok"，减少输出 token 数量
	out, err := runner(ctx, "claude", env, "say ok", model)
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

	if p.store != nil {
		go func(rec store.HealthCheckRecord) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if !p.shouldInsertHealthRecord(ctx, rec.AccountID, rec.NodeID, rec.Success, rec.CheckTime) {
				return
			}
			_ = p.store.InsertHealthCheck(ctx, &rec)
		}(rec)
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

func (p *Server) shouldInsertHealthRecord(ctx context.Context, accountID, nodeID string, success bool, checkTime time.Time) bool {
	if p == nil || p.store == nil {
		return false
	}
	if checkTime.IsZero() {
		checkTime = time.Now().UTC()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	last, err := p.store.LatestHealthCheck(ctx, accountID, nodeID)
	if err != nil {
		if p.logger != nil {
			p.logger.Printf("health history lookup failed for node %s: %v", nodeID, err)
		}
		return true // 出错时保守写入，避免数据缺失
	}
	if last == nil {
		return true
	}
	// 如果当前记录早于已存在的最新记录，跳过以避免乱序。
	if checkTime.Before(last.CheckTime) {
		return false
	}
	if last.Success != success {
		return true
	}
	if checkTime.Sub(last.CheckTime) >= sameStateRecordGap {
		return true
	}
	return false
}

type CliRunner func(ctx context.Context, image string, env map[string]string, prompt string, model string) (string, error)

func defaultCLIRunner(ctx context.Context, image string, env map[string]string, prompt string, model string) (string, error) {
	// image 参数保留以兼容旧接口（当前直接调用本地 claude CLI）。
	_ = image

	// 使用 -p/--print 来获取非交互式输出
	// 超时通过 context 控制（在 healthCheckViaCLI 中设置）
	// --tools "" 禁用所有工具，避免加载工具定义，加速响应
	args := []string{"-p", prompt, "--tools", ""}

	// 如果指定了模型，添加 --model 参数
	if model != "" {
		args = append(args, "--model", model)
	}
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
