package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"qcc_plus/internal/store"
)

// 添加新节点（默认账号）。
func (p *Server) addNode(name, rawURL, apiKey string, weight int) (*Node, error) {
	return p.addNodeToAccount(p.defaultAccount, name, rawURL, apiKey, weight)
}

// 添加指定账号的节点。
func (p *Server) addNodeToAccount(acc *Account, name, rawURL, apiKey string, weight int) (*Node, error) {
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
	id := fmt.Sprintf("n-%d", time.Now().UnixNano())
	node := &Node{ID: id, Name: name, URL: u, APIKey: apiKey, AccountID: acc.ID, CreatedAt: time.Now(), Weight: weight}

	p.mu.Lock()
	acc.Nodes[id] = node
	p.nodeIndex[id] = node
	p.nodeAccount[id] = acc
	cur := acc.Nodes[acc.ActiveID]
	curFailed := cur != nil && (cur.Failed || cur.Disabled)
	needSwitch := cur == nil || curFailed || node.Weight < cur.Weight
	var rec store.NodeRecord
	if p.store != nil {
		rec = store.NodeRecord{ID: id, Name: name, BaseURL: rawURL, APIKey: apiKey, AccountID: acc.ID, Weight: weight, CreatedAt: node.CreatedAt}
	}
	p.mu.Unlock()

	if p.store != nil {
		_ = p.store.UpsertNode(context.Background(), rec)
	}

	// 新节点优先级更高或当前无有效节点时，触发一次重选
	if needSwitch {
		p.logger.Printf("auto-switch after adding node %s (weight %d)", node.Name, node.Weight)
		_, _ = p.selectBestAndActivate(acc)
	}
	return node, nil
}

func (p *Server) updateNode(id, name, rawURL, apiKey string, weight int) error {
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
	oldWeight := n.Weight
	if name != "" {
		n.Name = name
	}
	n.URL = u
	n.APIKey = apiKey
	n.Weight = weight
	acc := p.nodeAccount[id]
	p.mu.Unlock()

	if p.store != nil {
		rec := toRecord(n)
		if err := p.store.UpsertNode(context.Background(), rec); err != nil {
			return err
		}
	}

	// 权重变更可能影响优先级，事件驱动触发一次重选
	if acc != nil && oldWeight != weight {
		p.logger.Printf("node %s weight changed %d -> %d, reselecting active node", n.Name, oldWeight, weight)
		_, _ = p.selectBestAndActivate(acc)
	}
	return nil
}

func (p *Server) deleteNode(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.nodeIndex[id]; !ok {
		return fmt.Errorf("node %s not found", id)
	}
	acc := p.nodeAccount[id]
	delete(acc.Nodes, id)
	delete(acc.FailedSet, id)
	delete(p.nodeIndex, id)
	delete(p.nodeAccount, id)
	if acc.ActiveID == id {
		acc.ActiveID = ""
	}
	if p.store != nil {
		return p.store.DeleteNode(context.Background(), id)
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
		return p.selectBestAndActivate(acc)
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
func (p *Server) selectBestAndActivate(acc *Account) (*Node, error) {
	if acc == nil {
		return nil, ErrNoActiveNode
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	bestID := ""
	var bestNode *Node
	for id, n := range acc.Nodes {
		if n.Failed || n.Disabled {
			continue
		}
		if bestNode == nil || n.Weight < bestNode.Weight || (n.Weight == bestNode.Weight && n.CreatedAt.Before(bestNode.CreatedAt)) {
			bestNode = n
			bestID = id
		}
	}
	if bestNode == nil {
		return nil, ErrNoActiveNode
	}
	acc.ActiveID = bestID
	if p.store != nil {
		_ = p.store.SetActive(context.Background(), acc.ID, bestID)
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

	// 如果禁用的是当前活跃节点，立即切换到下一个可用节点
	if wasActive {
		p.logger.Printf("disabled active node %s, switching to next available", n.Name)
		p.selectBestAndActivate(acc)
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
