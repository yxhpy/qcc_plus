package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"qcc_plus/internal/store"
)

const (
	historyStatusSent   = "sent"
	historyStatusFailed = "failed"
)

// Manager 负责异步派发通知。
type Manager struct {
	store      Store
	cfg        ManagerConfig
	queue      chan Event
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopped    chan struct{}
	dedupMu    sync.Mutex
	lastNotify map[string]time.Time
}

// Option 自定义管理器配置。
type Option func(*ManagerConfig)

// WithQueueSize 设置队列长度。
func WithQueueSize(n int) Option {
	return func(c *ManagerConfig) {
		if n > 0 {
			c.QueueSize = n
		}
	}
}

// WithWorkerCount 设置并发消费者数量。
func WithWorkerCount(n int) Option {
	return func(c *ManagerConfig) {
		if n > 0 {
			c.WorkerCount = n
		}
	}
}

// WithDedupWindow 设置去重时间窗口。
func WithDedupWindow(d time.Duration) Option {
	return func(c *ManagerConfig) {
		if d > 0 {
			c.DedupWindow = d
		}
	}
}

// WithLogger 设置自定义日志。
func WithLogger(l Logger) Option {
	return func(c *ManagerConfig) {
		if l != nil {
			c.Logger = l
		}
	}
}

// WithSendTimeout 设置单次发送超时时间。
func WithSendTimeout(d time.Duration) Option {
	return func(c *ManagerConfig) {
		if d > 0 {
			c.SendTimeout = d
		}
	}
}

// NewManager 创建并启动通知管理器。
func NewManager(store Store, opts ...Option) *Manager {
	cfg := ManagerConfig{
		QueueSize:   128,
		WorkerCount: 2,
		DedupWindow: 5 * time.Minute,
		Logger:      log.Default(),
		SendTimeout: 8 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 1
	}
	m := &Manager{
		store:      store,
		cfg:        cfg,
		queue:      make(chan Event, cfg.QueueSize),
		stopped:    make(chan struct{}),
		lastNotify: make(map[string]time.Time),
	}
	for i := 0; i < cfg.WorkerCount; i++ {
		m.wg.Add(1)
		go m.worker()
	}
	return m
}

// Publish 将事件放入队列，若队列满则丢弃并记录日志。
func (m *Manager) Publish(evt Event) {
	if m == nil {
		return
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now()
	}
	select {
	case m.queue <- evt:
	default:
		m.logf("notify queue full, drop event %s for account %s", evt.EventType, evt.AccountID)
	}
}

// Stop 停止后台 worker。
func (m *Manager) Stop() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() {
		close(m.queue)
		m.wg.Wait()
		close(m.stopped)
	})
}

// worker 消费事件并发送通知。
func (m *Manager) worker() {
	defer m.wg.Done()
	for evt := range m.queue {
		m.handleEvent(evt)
	}
}

func (m *Manager) handleEvent(evt Event) {
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.SendTimeout)
	defer cancel()

	subs, err := m.store.ListEnabledSubscriptionsForEvent(ctx, evt.AccountID, evt.EventType)
	if err != nil {
		m.logf("list subscriptions failed: %v", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	for _, sub := range subs {
		key := m.composeDedupKey(evt, sub)
		if !m.shouldSend(key) {
			continue
		}
		ch, err := buildChannel(sub.Channel)
		if err != nil {
			m.logf("build channel %s failed: %v", sub.Channel.ID, err)
			continue
		}
		msg := NotificationMessage{
			AccountID:  evt.AccountID,
			EventType:  evt.EventType,
			Title:      evt.Title,
			Content:    evt.Content,
			OccurredAt: evt.OccurredAt,
		}
		sendErr := ch.Send(ctx, msg)
		status := historyStatusSent
		errText := ""
		var sentAt *time.Time
		if sendErr != nil {
			status = historyStatusFailed
			errText = sendErr.Error()
		} else {
			now := time.Now()
			sentAt = &now
		}
		if err := m.store.InsertNotificationHistory(ctx, store.NotificationHistoryRecord{
			ID:        randomID(),
			AccountID: evt.AccountID,
			ChannelID: sub.Channel.ID,
			EventType: evt.EventType,
			Title:     evt.Title,
			Content:   evt.Content,
			Status:    status,
			Error:     errText,
			SentAt:    sentAt,
			CreatedAt: time.Now(),
		}); err != nil {
			m.logf("insert notification history failed: %v", err)
		}
		if sendErr != nil {
			m.logf("send notification via %s failed: %v", sub.Channel.ChannelType, sendErr)
		}
	}
}

func (m *Manager) composeDedupKey(evt Event, sub store.SubscriptionWithChannel) string {
	key := evt.EventType
	if evt.DedupKey != "" {
		key += ":" + evt.DedupKey
	}
	return sub.Channel.ID + "|" + evt.AccountID + "|" + key
}

func (m *Manager) shouldSend(key string) bool {
	m.dedupMu.Lock()
	defer m.dedupMu.Unlock()
	now := time.Now()
	if last, ok := m.lastNotify[key]; ok {
		if now.Sub(last) < m.cfg.DedupWindow {
			return false
		}
	}
	m.lastNotify[key] = now
	return true
}

func (m *Manager) logf(format string, v ...interface{}) {
	if m.cfg.Logger != nil {
		m.cfg.Logger.Printf(format, v...)
	}
}

func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
