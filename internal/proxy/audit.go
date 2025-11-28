package proxy

import (
	"sync"
	"time"
)

type EventType string

const (
	EvNodeFail    EventType = "node_fail"    // 节点失败
	EvNodeRecover EventType = "node_recover" // 节点恢复
	EvSwitch      EventType = "switch"       // 节点切换
	EvHealth      EventType = "health"       // 健康检查结果
)

type AuditEvent struct {
	Ts       time.Time              `json:"ts"`
	Tenant   string                 `json:"tenant"`
	NodeID   string                 `json:"node_id"`
	NodeName string                 `json:"node_name"`
	Type     EventType              `json:"type"`
	Detail   string                 `json:"detail"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

// AuditLog implements a simple fixed-size ring buffer for audit events.
type AuditLog struct {
	mu     sync.RWMutex
	events []AuditEvent
	cursor int
	size   int // 当前已写入的事件数
	cap    int // 容量
}

func NewAuditLog(capacity int) *AuditLog {
	if capacity <= 0 {
		capacity = 1000
	}
	return &AuditLog{
		events: make([]AuditEvent, capacity),
		cap:    capacity,
	}
}

func (a *AuditLog) Add(event AuditEvent) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	a.events[a.cursor] = event
	a.cursor = (a.cursor + 1) % a.cap
	if a.size < a.cap {
		a.size++
	}
}

func (a *AuditLog) ListRecent(limit int) []AuditEvent {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > a.size {
		limit = a.size
	}

	result := make([]AuditEvent, 0, limit)
	// 从最新的开始读取
	for i := 0; i < limit; i++ {
		idx := (a.cursor - 1 - i + a.cap) % a.cap
		if idx < 0 || idx >= a.size {
			break
		}
		result = append(result, a.events[idx])
	}
	return result
}
