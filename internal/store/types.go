package store

import (
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
	ID          string
	Name        string
	BaseURL     string
	APIKey      string
	AccountID   string
	Weight      int
	Failed      bool
	Disabled    bool
	LastError   string
	CreatedAt   time.Time
	Requests    int64
	FailCount   int64
	FailStreak  int64
	TotalBytes  int64
	TotalInput  int64
	TotalOutput int64
	StreamDurMs int64
	FirstByteMs int64
	LastPingMs  int64
	LastPingErr string
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
