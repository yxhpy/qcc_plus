package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	if failed {
		node.Failed = true
		if acc != nil {
			acc.FailedSet[nodeID] = struct{}{}
		}
	}
	p.mu.Unlock()

	if failed {
		p.logger.Printf("node %s marked failed: %s", node.Name, errMsg)
		p.selectBestAndActivate(acc)
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

	// 读锁保护节点查找，复制必要字段后立即解锁，避免与删除竞争。
	p.mu.RLock()
	node := acc.Nodes[id]
	if node == nil {
		p.mu.RUnlock()
		return
	}
	apiKey := node.APIKey
	nodeURL := node.URL.String()
	p.mu.RUnlock()

	var (
		ok      bool
		pingErr string
	)

	if apiKey != "" {
		payload := map[string]interface{}{
			"model":      "claude-3-5-haiku-20241022",
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		apiURL := strings.TrimSuffix(nodeURL, "/") + "/v1/messages"
		req, _ := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Transport: p.healthRT, Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			pingErr = err.Error()
		} else {
			defer resp.Body.Close()
			ok = resp.StatusCode >= 200 && resp.StatusCode < 300
			if !ok {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
				pingErr = fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))
			}
		}
	} else {
		client := &http.Client{Transport: p.healthRT, Timeout: 5 * time.Second}
		req, _ := http.NewRequest(http.MethodHead, nodeURL, nil)
		resp, err := client.Do(req)
		if err != nil {
			pingErr = err.Error()
		} else {
			defer resp.Body.Close()
			ok = resp.StatusCode == http.StatusOK
			if !ok {
				pingErr = fmt.Sprintf("status %d", resp.StatusCode)
			}
		}
	}

	p.mu.Lock()
	n := p.nodeIndex[id]
	if n != nil {
		acc := p.nodeAccount[id]
		if ok {
			n.Failed = false
			n.LastError = ""
			n.Metrics.FailStreak = 0
			n.Metrics.LastPingErr = ""
			if acc != nil {
				delete(acc.FailedSet, id)
			}
			if p.store != nil {
				_ = p.store.UpsertNode(context.Background(), toRecord(n))
			}
		} else if pingErr != "" {
			n.Metrics.LastPingErr = pingErr
		}
	}
	p.mu.Unlock()
	if ok {
		// 如果是更高优先级（权重更小）的节点且当前节点优先级更低，则自动切回。
		p.maybePromoteRecovered(n)
	}
}

func (p *Server) maybePromoteRecovered(n *Node) {
	if n == nil {
		return
	}
	acc := p.nodeAccount[n.ID]
	cur, _ := p.getActiveNodeForAccount(acc)
	if cur == nil || cur.Failed || n.Weight < cur.Weight {
		p.mu.Lock()
		if acc != nil {
			acc.ActiveID = n.ID
		}
		if p.store != nil && acc != nil {
			_ = p.store.SetActive(context.Background(), acc.ID, n.ID)
		}
		p.mu.Unlock()
		p.logger.Printf("auto-switch to recovered node %s (weight %d)", n.Name, n.Weight)
	}
}
