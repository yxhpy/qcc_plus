package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
)

const (
	HealthCheckMethodAPI  = "api"  // POST /v1/messages
	HealthCheckMethodHEAD = "head" // HEAD 请求
	HealthCheckMethodCLI  = "cli"  // Claude Code CLI 无头模式
)

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
	if failed {
		node.Failed = true
		if acc != nil {
			acc.FailedSet[nodeID] = struct{}{}
		}
	}
	p.mu.Unlock()

	if failed {
		p.logger.Printf("node %s marked failed: %s", nodeName, errMsg)
		if p.notifyMgr != nil && acc != nil {
			p.notifyMgr.Publish(notify.Event{
				AccountID:  acc.ID,
				EventType:  notify.EventNodeFailed,
				Title:      "节点故障告警",
				Content:    fmt.Sprintf("**节点名称**: %s\n**错误信息**: %s\n**失败次数**: %d\n**时间**: %s", nodeName, errMsg, failStreak, time.Now().Format("2006-01-02 15:04:05")),
				DedupKey:   node.ID,
				OccurredAt: time.Now(),
			})
		}
		p.selectBestAndActivate(acc, "节点故障")
	}
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
	p.mu.RLock()
	accs := make([]*Account, 0, len(p.accountByID))
	for _, acc := range p.accountByID {
		accs = append(accs, acc)
	}
	p.mu.RUnlock()
	for _, acc := range accs {
		for id := range acc.FailedSet {
			p.checkNodeHealth(acc, id)
		}
	}
}

func (p *Server) checkNodeHealth(acc *Account, id string) {
	if acc == nil {
		return
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	method := normalizeHealthCheckMethod(nodeCopy.HealthCheckMethod)

	var (
		ok       bool
		pingErr  string
		latency  time.Duration
		fallback bool
	)

	switch method {
	case HealthCheckMethodAPI:
		ok, pingErr, latency = p.healthCheckViaAPI(ctx, nodeCopy)
	case HealthCheckMethodHEAD:
		ok, pingErr, latency = p.healthCheckViaHEAD(ctx, nodeCopy)
	case HealthCheckMethodCLI:
		ok, pingErr, latency, fallback = p.healthCheckViaCLI(ctx, nodeCopy)
		if !ok && fallback && nodeCopy.APIKey != "" {
			p.logger.Printf("cli health check failed for node %s, fallback to api: %s", nodeCopy.Name, pingErr)
			ok, pingErr, latency = p.healthCheckViaAPI(ctx, nodeCopy)
		}
	default:
		ok, pingErr, latency = p.healthCheckViaAPI(ctx, nodeCopy)
	}

	var (
		rec           store.NodeRecord
		shouldPersist bool
	)

	p.mu.Lock()
	n := p.nodeIndex[id]
	if n != nil {
		acc := p.nodeAccount[id]
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
			if p.store != nil {
				rec = toRecord(n)
				shouldPersist = true
			}
		} else if pingErr != "" {
			n.Metrics.LastPingErr = pingErr
			if p.store != nil {
				rec = toRecord(n)
				shouldPersist = true
			}
		}
	}
	p.mu.Unlock()
	if shouldPersist {
		_ = p.store.UpsertNode(context.Background(), rec)
	}
	if ok {
		// 恢复后重新在健康节点中选择最优的一个。
		if p.notifyMgr != nil && acc != nil && n != nil {
			p.notifyMgr.Publish(notify.Event{
				AccountID:  acc.ID,
				EventType:  notify.EventNodeRecovered,
				Title:      "节点已恢复",
				Content:    fmt.Sprintf("**节点名称**: %s\n**恢复时间**: %s", n.Name, time.Now().Format("2006-01-02 15:04:05")),
				DedupKey:   n.ID,
				OccurredAt: time.Now(),
			})
		}
		p.maybePromoteRecovered(n)
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
		return HealthCheckMethodAPI
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

func (p *Server) healthCheckViaCLI(ctx context.Context, node Node) (bool, string, time.Duration, bool) {
	runner := p.cliRunner
	if runner == nil {
		runner = defaultCLIRunner
	}
	if node.APIKey == "" {
		return false, "cli health check requires api key", 0, false
	}
	if node.URL == nil {
		return false, "cli health check requires valid base url", 0, false
	}
	env := map[string]string{
		"ANTHROPIC_API_KEY":    node.APIKey,
		"ANTHROPIC_AUTH_TOKEN": chooseNonEmpty(os.Getenv("ANTHROPIC_AUTH_TOKEN"), node.APIKey),
		"ANTHROPIC_BASE_URL":   node.URL.String(),
	}

	start := time.Now()
	out, err := runner(ctx, "claude-code-cli-verify", env, "hi")
	latency := time.Since(start)
	if err != nil {
		return false, err.Error(), latency, isDockerUnavailable(err)
	}
	if strings.TrimSpace(out) == "" {
		return false, "cli health check returned empty output", latency, false
	}
	return true, "", latency, false
}

type CliRunner func(ctx context.Context, image string, env map[string]string, prompt string) (string, error)

func defaultCLIRunner(ctx context.Context, image string, env map[string]string, prompt string) (string, error) {
	args := []string{"run", "--rm"}
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, image, "-p", prompt)

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: stdout=%s stderr=%s", err, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "docker daemon") || strings.Contains(lower, "cannot connect to the docker daemon") || strings.Contains(lower, "permission denied while trying to connect to the docker daemon")
}
