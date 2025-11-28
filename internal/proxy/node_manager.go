package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
)

// 添加新节点（默认账号）。
func (p *Server) addNode(name, rawURL, apiKey string, weight int) (*Node, error) {
	return p.addNodeWithMethod(p.defaultAccount, name, rawURL, apiKey, weight, "")
}

// 添加指定账号的节点。
func (p *Server) addNodeToAccount(acc *Account, name, rawURL, apiKey string, weight int) (*Node, error) {
	return p.addNodeWithMethod(acc, name, rawURL, apiKey, weight, "")
}

// 添加指定账号的节点并自定义健康检查方式。
func (p *Server) addNodeWithMethod(acc *Account, name, rawURL, apiKey string, weight int, healthMethod string) (*Node, error) {
	if acc == nil {
		return nil, errors.New("account required")
	}
	if rawURL == "" {
		return nil, errors.New("base_url required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = u.Host
	}
	if weight <= 0 {
		weight = 1
	}
	// 未指定时使用全局默认健康检查方式（可被环境变量覆盖）
	if healthMethod == "" {
		healthMethod = defaultHealthCheckMethod
	}
	healthMethod = normalizeHealthCheckMethod(healthMethod)
	if healthMethodRequiresAPIKey(healthMethod) && apiKey == "" {
		// CLI/API 探活都需要密钥，缺失时统一降级到 HEAD，保证可用性。
		p.logger.Printf("health check mode %s requires api key, fallback to head for node %s", healthMethod, name)
		healthMethod = HealthCheckMethodHEAD
	}
	id := fmt.Sprintf("n-%d", time.Now().UnixNano())
	windowSize := acc.Config.WindowSize
	if windowSize == 0 {
		windowSize = 200
	}
	alphaErr := acc.Config.AlphaErr
	if alphaErr == 0 {
		alphaErr = 5.0
	}
	betaLat := acc.Config.BetaLatency
	if betaLat == 0 {
		betaLat = 0.5
	}
	node := &Node{ID: id, Name: name, URL: u, APIKey: apiKey, HealthCheckMethod: healthMethod, AccountID: acc.ID, CreatedAt: time.Now(), Weight: weight, Window: NewMetricsWindow(windowSize)}
	node.Score = CalculateScore(node, alphaErr, betaLat)

	p.mu.Lock()
	acc.Nodes[id] = node
	p.nodeIndex[id] = node
	p.nodeAccount[id] = acc
	cur := acc.Nodes[acc.ActiveID]
	curFailed := cur != nil && (cur.Failed || cur.Disabled)
	needSwitch := cur == nil || curFailed || node.Weight < cur.Weight
	var rec store.NodeRecord
	if p.store != nil {
		rec = store.NodeRecord{ID: id, Name: name, BaseURL: rawURL, APIKey: apiKey, HealthCheckMethod: healthMethod, AccountID: acc.ID, Weight: weight, CreatedAt: node.CreatedAt}
	}
	p.mu.Unlock()

	if p.store != nil {
		_ = p.store.UpsertNode(context.Background(), rec)
	}

	if p.notifyMgr != nil {
		p.notifyMgr.Publish(notify.Event{
			AccountID:  acc.ID,
			EventType:  notify.EventNodeAdded,
			Title:      "节点新增",
			Content:    fmt.Sprintf("**节点名称**: %s\n**地址**: %s\n**权重**: %d\n**时间**: %s", node.Name, node.URL.String(), node.Weight, timeutil.FormatBeijingTime(time.Now())),
			DedupKey:   node.ID,
			OccurredAt: time.Now(),
		})
	}

	// 新节点优先级更高或当前无有效节点时，触发一次重选
	if needSwitch {
		p.logger.Printf("auto-switch after adding node %s (weight %d)", node.Name, node.Weight)
		_, _ = p.selectBestAndActivate(acc, "新增节点")
	}
	return node, nil
}

func (p *Server) updateNode(id, name, rawURL string, apiKey *string, weight int, healthMethod *string) error {
	if rawURL == "" {
		return errors.New("base_url required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if weight <= 0 {
		weight = 1
	}
	p.mu.Lock()
	n, ok := p.nodeIndex[id]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("node %s not found", id)
	}
	oldAPIKey := n.APIKey
	newAPIKey := oldAPIKey
	if apiKey != nil {
		newAPIKey = *apiKey
	}
	desiredMethod := n.HealthCheckMethod
	if healthMethod != nil {
		desiredMethod = *healthMethod
	}
	desiredMethod = normalizeHealthCheckMethod(desiredMethod)
	if healthMethodRequiresAPIKey(desiredMethod) && newAPIKey == "" {
		// CLI/API 探活需要密钥，缺失时统一降级为 HEAD。
		p.logger.Printf("health check mode %s requires api key, fallback to head for node %s", desiredMethod, n.Name)
		desiredMethod = HealthCheckMethodHEAD
	}
	oldWeight := n.Weight
	if name != "" {
		n.Name = name
	}
	n.URL = u
	n.APIKey = newAPIKey
	n.Weight = weight
	n.HealthCheckMethod = desiredMethod
	acc := p.nodeAccount[id]
	windowSize := 200
	alphaErr := 5.0
	betaLat := 0.5
	if acc != nil {
		if acc.Config.WindowSize > 0 {
			windowSize = acc.Config.WindowSize
		}
		if acc.Config.AlphaErr != 0 {
			alphaErr = acc.Config.AlphaErr
		}
		if acc.Config.BetaLatency != 0 {
			betaLat = acc.Config.BetaLatency
		}
	}
	if n.Window == nil {
		n.Window = NewMetricsWindow(windowSize)
	}
	n.Score = CalculateScore(n, alphaErr, betaLat)
	p.mu.Unlock()

	if p.store != nil {
		rec := toRecord(n)
		if err := p.store.UpsertNode(context.Background(), rec); err != nil {
			return err
		}
	}

	if p.notifyMgr != nil && acc != nil {
		p.notifyMgr.Publish(notify.Event{
			AccountID:  acc.ID,
			EventType:  notify.EventNodeUpdated,
			Title:      "节点已更新",
			Content:    fmt.Sprintf("**节点名称**: %s\n**地址**: %s\n**权重**: %d", n.Name, n.URL.String(), n.Weight),
			DedupKey:   n.ID,
			OccurredAt: time.Now(),
		})
	}

	// 权重变更可能影响优先级，事件驱动触发一次重选
	if acc != nil && oldWeight != weight {
		p.logger.Printf("node %s weight changed %d -> %d, reselecting active node", n.Name, oldWeight, weight)
		_, _ = p.selectBestAndActivate(acc, "权重调整")
	}
	return nil
}

func (p *Server) deleteNode(id string) error {
	p.mu.Lock()
	n, ok := p.nodeIndex[id]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("node %s not found", id)
	}
	acc := p.nodeAccount[id]
	accID := ""
	if acc != nil {
		accID = acc.ID
		delete(acc.Nodes, id)
		delete(acc.FailedSet, id)
		if acc.ActiveID == id {
			acc.ActiveID = ""
		}
	}
	delete(p.nodeIndex, id)
	delete(p.nodeAccount, id)
	p.mu.Unlock()

	if p.store != nil {
		if err := p.store.DeleteNode(context.Background(), id); err != nil {
			return err
		}
	}

	if p.notifyMgr != nil && acc != nil {
		baseURL := ""
		if n.URL != nil {
			baseURL = n.URL.String()
		}
		p.notifyMgr.Publish(notify.Event{
			AccountID:  accID,
			EventType:  notify.EventNodeDeleted,
			Title:      "节点已删除",
			Content:    fmt.Sprintf("**节点名称**: %s\n**地址**: %s", n.Name, baseURL),
			DedupKey:   n.ID,
			OccurredAt: time.Now(),
		})
	}

	return nil
}

// 激活指定节点。
func (p *Server) activate(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	acc := p.nodeAccount[id]
	if acc == nil {
		return fmt.Errorf("node %s not found", id)
	}
	if _, ok := acc.Nodes[id]; !ok {
		return fmt.Errorf("node %s not found", id)
	}
	acc.ActiveID = id
	if p.store != nil {
		_ = p.store.SetActive(context.Background(), acc.ID, id)
	}
	return nil
}

// 根据 id 获取节点（只读）。
func (p *Server) getNode(id string) *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeIndex[id]
}

// 获取账号下的当前激活节点，如果失败则自动切换。
func (p *Server) getActiveNodeForAccount(acc *Account) (*Node, error) {
	if acc == nil {
		return nil, ErrNoActiveNode
	}
	p.mu.RLock()
	activeID := acc.ActiveID
	n, ok := acc.Nodes[activeID]
	failed := ok && (n.Failed || n.Disabled)
	p.mu.RUnlock()

	if !ok || failed {
		return p.selectBestAndActivate(acc, "当前节点不可用")
	}
	return n, nil
}

// 兼容旧调用：不传参则返回默认账号的激活节点，传入账号则返回对应激活节点。
func (p *Server) getActiveNode(acc ...*Account) (*Node, error) {
	if len(acc) > 0 && acc[0] != nil {
		return p.getActiveNodeForAccount(acc[0])
	}
	return p.getActiveNodeForAccount(p.defaultAccount)
}

// 选择最低权重（最高优先级）的健康节点并激活。
func (p *Server) selectBestAndActivate(acc *Account, reason ...string) (*Node, error) {
	if acc == nil {
		return nil, ErrNoActiveNode
	}
	switchReason := "自动切换"
	if len(reason) > 0 && reason[0] != "" {
		switchReason = reason[0]
	}

	p.mu.Lock()
	prevID := acc.ActiveID
	prevNode := acc.Nodes[prevID]
	bestID := ""
	var bestNode *Node
	var bestScore float64
	now := time.Now()
	effectiveScore := func(n *Node) float64 {
		if n == nil {
			return 0
		}
		if n.Window == nil {
			return float64(n.Weight)
		}
		if n.Score == 0 {
			return float64(n.Weight)
		}
		return n.Score
	}
	for id, n := range acc.Nodes {
		if n.Failed || n.Disabled {
			continue
		}
		if acc.Config.Cooldown > 0 && !n.LastSwitchAt.IsZero() && now.Sub(n.LastSwitchAt) < acc.Config.Cooldown {
			continue
		}
		score := effectiveScore(n)
		if bestNode == nil || score < bestScore || (score == bestScore && n.CreatedAt.Before(bestNode.CreatedAt)) {
			bestNode = n
			bestID = id
			bestScore = score
		}
	}
	if bestNode == nil {
		p.mu.Unlock()
		return nil, ErrNoActiveNode
	}
	bestNode.LastSwitchAt = now
	acc.ActiveID = bestID
	if p.store != nil {
		_ = p.store.SetActive(context.Background(), acc.ID, bestID)
	}
	p.mu.Unlock()

	if p.notifyMgr != nil && acc != nil && prevID != bestID {
		fromName := "-"
		if prevNode != nil {
			fromName = prevNode.Name
		}
		p.notifyMgr.Publish(notify.Event{
			AccountID:  acc.ID,
			EventType:  notify.EventNodeSwitched,
			Title:      "节点自动切换",
			Content:    fmt.Sprintf("**从节点**: %s\n**到节点**: %s (权重: %d)\n**切换原因**: %s", chooseNonEmpty(fromName, "-"), bestNode.Name, bestNode.Weight, switchReason),
			DedupKey:   fmt.Sprintf("switch:%s", acc.ID),
			OccurredAt: time.Now(),
		})
	}

	if p.audit != nil && acc != nil && prevID != bestID {
		fromName := "-"
		if prevNode != nil {
			fromName = prevNode.Name
		}
		p.audit.Add(AuditEvent{
			Ts:       time.Now(),
			Tenant:   acc.ID,
			NodeID:   bestID,
			NodeName: bestNode.Name,
			Type:     EvSwitch,
			Detail:   fmt.Sprintf("%s → %s (%s)", fromName, bestNode.Name, switchReason),
			Meta: map[string]interface{}{
				"from_id":   prevID,
				"from_name": fromName,
				"to_id":     bestID,
				"to_name":   bestNode.Name,
				"to_score":  bestNode.Score,
				"to_weight": bestNode.Weight,
				"reason":    switchReason,
			},
		})
	}

	return bestNode, nil
}

// disableNode 手动禁用节点，如果是当前活跃节点则立即切换
func (p *Server) disableNode(id string) error {
	p.mu.Lock()
	n, ok := p.nodeIndex[id]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("node %s not found", id)
	}
	acc := p.nodeAccount[id]
	n.Disabled = true
	wasActive := acc != nil && id == acc.ActiveID
	p.mu.Unlock()

	if p.store != nil {
		rec := toRecord(n)
		if err := p.store.UpsertNode(context.Background(), rec); err != nil {
			return err
		}
	}

	if p.notifyMgr != nil && acc != nil {
		p.notifyMgr.Publish(notify.Event{
			AccountID:  acc.ID,
			EventType:  notify.EventNodeDisabled,
			Title:      "节点已禁用",
			Content:    fmt.Sprintf("**节点名称**: %s\n**操作**: 手动禁用", n.Name),
			DedupKey:   n.ID,
			OccurredAt: time.Now(),
		})
	}

	// 如果禁用的是当前活跃节点，立即切换到下一个可用节点
	if wasActive {
		p.logger.Printf("disabled active node %s, switching to next available", n.Name)
		p.selectBestAndActivate(acc, "节点禁用")
	}
	return nil
}

// enableNode 手动启用节点，如果其优先级更高则自动切换
func (p *Server) enableNode(id string) error {
	p.mu.Lock()
	n, ok := p.nodeIndex[id]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("node %s not found", id)
	}
	acc := p.nodeAccount[id]
	n.Disabled = false
	n.Failed = false
	n.Metrics.FailStreak = 0
	if acc != nil {
		delete(acc.FailedSet, id)
	}
	p.mu.Unlock()

	if p.store != nil {
		rec := toRecord(n)
		if err := p.store.UpsertNode(context.Background(), rec); err != nil {
			return err
		}
	}

	if p.notifyMgr != nil && acc != nil {
		p.notifyMgr.Publish(notify.Event{
			AccountID:  acc.ID,
			EventType:  notify.EventNodeEnabled,
			Title:      "节点已启用",
			Content:    fmt.Sprintf("**节点名称**: %s\n**权重**: %d", n.Name, n.Weight),
			DedupKey:   n.ID,
			OccurredAt: time.Now(),
		})
	}

	// 检查是否需要切换到刚启用的节点（如果其优先级更高）
	cur, _ := p.getActiveNodeForAccount(acc)
	if cur == nil || cur.Failed || n.Weight < cur.Weight {
		p.mu.Lock()
		if acc != nil {
			acc.ActiveID = id
		}
		if p.store != nil {
			_ = p.store.SetActive(context.Background(), acc.ID, id)
		}
		p.mu.Unlock()
		p.logger.Printf("auto-switch to enabled node %s (weight %d)", n.Name, n.Weight)
	}
	return nil
}
