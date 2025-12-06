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
	return p.addNodeWithMethod(p.defaultAccount, name, rawURL, apiKey, weight, "", "")
}

// 添加指定账号的节点。
func (p *Server) addNodeToAccount(acc *Account, name, rawURL, apiKey string, weight int) (*Node, error) {
	return p.addNodeWithMethod(acc, name, rawURL, apiKey, weight, "", "")
}

// 添加指定账号的节点并自定义健康检查方式。
func (p *Server) addNodeWithMethod(acc *Account, name, rawURL, apiKey string, weight int, healthMethod string, healthModel string) (*Node, error) {
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
	model := chooseNonEmpty(healthModel, defaultHealthCheckModel)
	if healthMethodRequiresAPIKey(healthMethod) && apiKey == "" {
		// CLI/API 探活都需要密钥，缺失时统一降级到 HEAD，保证可用性。
		p.logger.Printf("health check mode %s requires api key, fallback to head for node %s", healthMethod, name)
		healthMethod = HealthCheckMethodHEAD
	}
	id := fmt.Sprintf("n-%d", time.Now().UnixNano())
	node := &Node{ID: id, Name: name, URL: u, APIKey: apiKey, HealthCheckMethod: healthMethod, HealthCheckModel: model, AccountID: acc.ID, CreatedAt: time.Now(), Weight: weight}

	p.mu.Lock()
	acc.Nodes[id] = node
	p.nodeIndex[id] = node
	p.nodeAccount[id] = acc
	cur := acc.Nodes[acc.ActiveID]
	curFailed := cur != nil && (cur.Failed || cur.Disabled)
	needSwitch := cur == nil || curFailed || node.Weight < cur.Weight
	var rec store.NodeRecord
	if p.store != nil {
		rec = store.NodeRecord{ID: id, Name: name, BaseURL: rawURL, APIKey: apiKey, HealthCheckMethod: healthMethod, HealthCheckModel: model, AccountID: acc.ID, Weight: weight, CreatedAt: node.CreatedAt}
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

func (p *Server) updateNode(id, name, rawURL string, apiKey *string, weight int, healthMethod *string, healthModel *string) error {
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
	desiredModel := chooseNonEmpty(n.HealthCheckModel, defaultHealthCheckModel)
	if healthMethod != nil {
		desiredMethod = *healthMethod
	}
	if healthModel != nil {
		desiredModel = chooseNonEmpty(*healthModel, defaultHealthCheckModel)
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
	n.HealthCheckModel = desiredModel
	acc := p.nodeAccount[id]
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

// selectHealthyNodeExcluding 选择健康节点，排除 skipNodes
func (p *Server) selectHealthyNodeExcluding(acc *Account, skipNodes map[string]bool) *Node {
	if acc == nil {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	var bestNode *Node
	for id, n := range acc.Nodes {
		if n.Failed || n.Disabled || p.isInFailedSet(acc, id) || skipNodes[id] {
			continue
		}

		// 不在选择阶段过滤熔断器状态，交由请求阶段的 AllowRequest() 控制
		// 这样熔断器可以在冷却后进入 Half-Open 状态进行试探

		if bestNode == nil || n.Weight < bestNode.Weight {
			bestNode = n
		}
	}
	return bestNode
}

// isInFailedSet 判断节点是否在失败集合中。调用方需确保并发安全（外部加锁或只读场景）。
func (p *Server) isInFailedSet(acc *Account, nodeID string) bool {
	if acc == nil {
		return false
	}
	_, ok := acc.FailedSet[nodeID]
	return ok
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
	for id, n := range acc.Nodes {
		if n.Failed || n.Disabled || p.isInFailedSet(acc, id) {
			continue
		}
		if bestNode == nil || n.Weight < bestNode.Weight || (n.Weight == bestNode.Weight && n.CreatedAt.Before(bestNode.CreatedAt)) {
			bestNode = n
			bestID = id
		}
	}
	if bestNode == nil {
		p.mu.Unlock()
		return nil, ErrNoActiveNode
	}

	if p.warmupConfig.Enabled {
		p.mu.Unlock()

		p.logger.Printf("warming up node %s before activation", bestNode.Name)
		successCount, err := p.warmupNode(bestNode)
		if err != nil && p.logger != nil {
			p.logger.Printf("warmup error for node %s: %v", bestNode.Name, err)
		}

		if !isNodeWarmedUp(successCount, p.warmupConfig) {
			p.logger.Printf("node %s warmup failed (%d/%d success), trying next node", bestNode.Name, successCount, p.warmupConfig.Attempts)

			skipNodes := make(map[string]bool)
			skipNodes[bestID] = true
			return p.selectBestAndActivateExcluding(acc, skipNodes, reason...)
		}

		p.logger.Printf("node %s warmed up successfully (%d/%d)", bestNode.Name, successCount, p.warmupConfig.Attempts)
		p.mu.Lock()
	}

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

	// 推送节点切换事件到 WebSocket
	if p.wsHub != nil && acc != nil && prevID != bestID {
		ts := timeutil.FormatBeijingTime(time.Now())
		// 旧节点变为非激活
		if prevNode != nil {
			p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
				"node_id":   prevID,
				"node_name": prevNode.Name,
				"status":    p.resolveNodeStatus(prevNode),
				"active":    false,
				"timestamp": ts,
			})
		}
		// 新节点变为激活
		p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
			"node_id":   bestID,
			"node_name": bestNode.Name,
			"status":    "online",
			"active":    true,
			"timestamp": ts,
		})
	}

	return bestNode, nil
}

// selectBestAndActivateExcluding 选择最佳节点并激活（排除指定节点）
// 用于预热失败后尝试下一个节点
func (p *Server) selectBestAndActivateExcluding(acc *Account, skipNodes map[string]bool, reason ...string) (*Node, error) {
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

	for id, n := range acc.Nodes {
		if n.Failed || n.Disabled || p.isInFailedSet(acc, id) || (skipNodes != nil && skipNodes[id]) {
			continue
		}
		if bestNode == nil || n.Weight < bestNode.Weight || (n.Weight == bestNode.Weight && n.CreatedAt.Before(bestNode.CreatedAt)) {
			bestNode = n
			bestID = id
		}
	}

	if bestNode == nil {
		p.mu.Unlock()
		return nil, ErrNoActiveNode
	}

	if p.warmupConfig.Enabled {
		p.mu.Unlock()

		p.logger.Printf("warming up node %s before activation", bestNode.Name)
		successCount, _ := p.warmupNode(bestNode)

		if !isNodeWarmedUp(successCount, p.warmupConfig) {
			p.logger.Printf("node %s warmup failed (%d/%d success), trying next node", bestNode.Name, successCount, p.warmupConfig.Attempts)

			if skipNodes == nil {
				skipNodes = make(map[string]bool)
			}
			skipNodes[bestID] = true
			return p.selectBestAndActivateExcluding(acc, skipNodes, reason...)
		}

		p.logger.Printf("node %s warmed up successfully", bestNode.Name)
		p.mu.Lock()
	}

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

	// 推送节点切换事件到 WebSocket
	if p.wsHub != nil && acc != nil && prevID != bestID {
		ts := timeutil.FormatBeijingTime(time.Now())
		if prevNode != nil {
			p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
				"node_id":   prevID,
				"node_name": prevNode.Name,
				"status":    p.resolveNodeStatus(prevNode),
				"active":    false,
				"timestamp": ts,
			})
		}
		p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
			"node_id":   bestID,
			"node_name": bestNode.Name,
			"status":    "online",
			"active":    true,
			"timestamp": ts,
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

	// 推送节点禁用状态到 WebSocket
	if p.wsHub != nil && acc != nil {
		p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
			"node_id":   id,
			"node_name": n.Name,
			"status":    "disabled",
			"timestamp": timeutil.FormatBeijingTime(time.Now()),
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

	// 推送节点启用状态到 WebSocket
	if p.wsHub != nil && acc != nil {
		p.wsHub.Broadcast(acc.ID, "node_status", map[string]interface{}{
			"node_id":   id,
			"node_name": n.Name,
			"status":    "online",
			"timestamp": timeutil.FormatBeijingTime(time.Now()),
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

// resolveNodeStatus 根据节点状态返回对应的状态字符串
func (p *Server) resolveNodeStatus(n *Node) string {
	if n == nil {
		return "unknown"
	}
	if n.Disabled {
		return "disabled"
	}
	if n.Failed {
		return "offline"
	}
	return "online"
}
