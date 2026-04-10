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
	APIKeyConfig      string
	APIKeyItems       []NamedAPIKey
	APIKeys           *KeyRotator // 多密钥轮换器（当 APIKey 包含逗号时自动启用）
	HealthCheckMethod string
	HealthCheckModel  string            // CLI 健康检查使用的模型，默认为 claude-haiku-4-5-20251001
	ModelMapping      map[string]string // 模型映射：请求中的模型 -> 转发给上游的模型
	SourceProtocol    string            // 源协议：claude/openai/gemini
	WireAPI           string            // OpenAI/Codex 上游接口：responses/chat_completions
	AuthProfile       string            // JSON 格式鉴权配置
	Capabilities      string            // JSON 格式能力声明
	AccountID         string
	CreatedAt         time.Time
	Metrics           metrics
	Weight            int
	MaxConcurrency    int
	Failed            bool
	Disabled          bool // 用户手动禁用
	LastError         string
}

// MapModel 根据节点的模型映射表转换模型 ID，无映射则返回原值。
func (n *Node) MapModel(model string) string {
	if n.ModelMapping == nil {
		return model
	}
	if mapped, ok := n.ModelMapping[model]; ok && mapped != "" {
		return mapped
	}
	return model
}

func (n *Node) NormalizedWireAPI() string {
	if n == nil {
		return normalizeOpenAIWireAPI("")
	}
	if NormalizedSourceProtocol(n.SourceProtocol) != SourceProtocolOpenAI {
		return ""
	}
	return normalizeOpenAIWireAPI(n.WireAPI)
}

type ActiveAPIKeyInfo struct {
	Key         string
	Name        string
	DisplayName string
}

// GetActiveAPIKey 获取当前应使用的 API Key（支持多密钥轮换）
func (n *Node) GetActiveAPIKey() string {
	return n.GetActiveAPIKeyInfo().Key
}

func (n *Node) GetActiveAPIKeyInfo() ActiveAPIKeyInfo {
	if n == nil {
		return ActiveAPIKeyInfo{}
	}

	key := n.APIKey
	if n.APIKeys != nil && n.APIKeys.KeyCount() > 0 {
		key = n.APIKeys.GetCurrentKey()
	}

	info := ActiveAPIKeyInfo{
		Key:         key,
		DisplayName: n.Name,
	}
	if key == "" {
		return info
	}

	for _, item := range n.APIKeyItems {
		if item.Key == key {
			info.Name = item.Name
			info.DisplayName = displayNodeNameForKey(n.Name, item.Name)
			return info
		}
	}

	return info
}

func (n *Node) DisplayNameForKey(key string) string {
	if n == nil {
		return ""
	}
	if key == "" {
		return n.Name
	}
	for _, item := range n.APIKeyItems {
		if item.Key == key {
			return displayNodeNameForKey(n.Name, item.Name)
		}
	}
	return n.Name
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
	input     int64
	output    int64
	modelID   string // 使用的模型 ID
	requestID string // 请求 ID（用于追踪）
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
