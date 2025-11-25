package proxy

import (
	"net/url"
	"time"
)

// Node 代表一个可切换的上游节点。
type Node struct {
	ID                string
	Name              string
	URL               *url.URL
	APIKey            string
	HealthCheckMethod string
	AccountID         string
	CreatedAt         time.Time
	Metrics           metrics
	Weight            int
	Failed            bool
	Disabled          bool // 用户手动禁用
	LastError         string
}

// metrics 记录节点请求与健康状况统计。
type metrics struct {
	Requests          int64
	StreamDur         time.Duration // 累计（首字节到末字节）
	FirstByteDur      time.Duration // 累计首字节延时
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalBytes        int64
	LastPingMS        int64
	LastPingErr       string
	LastHealthCheckAt time.Time
	FailCount         int64 // 总失败次数（非200）
	FailStreak        int64 // 连续失败次数
}

// usage 描述一次请求的 token 统计。
type usage struct {
	input  int64
	output int64
}

// Config 描述可运行时调整的系统配置。
type Config struct {
	Retries     int
	FailLimit   int
	HealthEvery time.Duration
}

// Account 表示一个租户，持有独立的节点与配置。
type Account struct {
	ID          string
	Name        string
	Password    string
	ProxyAPIKey string
	IsAdmin     bool
	Nodes       map[string]*Node
	ActiveID    string
	Config      Config
	FailedSet   map[string]struct{}
}

// TunnelStatus 返回给前端的隧道状态视图。
type TunnelStatus struct {
	APITokenSet bool   `json:"api_token_set"`
	Subdomain   string `json:"subdomain"`
	Zone        string `json:"zone"`
	Enabled     bool   `json:"enabled"`
	PublicURL   string `json:"public_url"`
	Status      string `json:"status"`
	LastError   string `json:"last_error"`
}

// 用于在上下文中传递 usage。
type usageContextKey struct{}
