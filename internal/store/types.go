package store

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	// DefaultAccountID 用于向后兼容的默认账号。
	DefaultAccountID = "default"
	defaultTimeout   = 5 * time.Second
)

// NodeRecord mirrors persistent fields for a proxy node.
type NodeRecord struct {
	ID                string
	Name              string
	BaseURL           string
	APIKey            string
	HealthCheckMethod string
	AccountID         string
	Weight            int
	Failed            bool
	Disabled          bool
	LastError         string
	CreatedAt         time.Time
	Requests          int64
	FailCount         int64
	FailStreak        int64
	TotalBytes        int64
	TotalInput        int64
	TotalOutput       int64
	StreamDurMs       int64
	FirstByteMs       int64
	LastPingMs        int64
	LastPingErr       string
	LastHealthCheckAt time.Time
}

// AccountRecord 账号记录。
type AccountRecord struct {
	ID          string
	Name        string
	Password    string
	ProxyAPIKey string // 用于代理路由识别的 API Key
	IsAdmin     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Config holds runtime tunables persisted in DB.
type Config struct {
	Retries     int
	FailLimit   int
	HealthEvery time.Duration
}

var ErrNotFound = errors.New("not found")

// NotificationChannelRecord 描述通知渠道的持久化结构。
type NotificationChannelRecord struct {
	ID          string
	AccountID   string
	ChannelType string
	Name        string
	Config      json.RawMessage
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NotificationSubscriptionRecord 描述通知订阅。
type NotificationSubscriptionRecord struct {
	ID        string
	AccountID string
	ChannelID string
	EventType string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NotificationHistoryRecord 记录通知发送历史。
type NotificationHistoryRecord struct {
	ID        string
	AccountID string
	ChannelID string
	EventType string
	Title     string
	Content   string
	Status    string
	Error     string
	SentAt    *time.Time
	CreatedAt time.Time
}

// SubscriptionWithChannel 将订阅与渠道信息合并，便于发送层使用。
type SubscriptionWithChannel struct {
	Subscription NotificationSubscriptionRecord
	Channel      NotificationChannelRecord
}
