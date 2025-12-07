package store

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	// DefaultAccountID 用于向后兼容的默认账号。
	DefaultAccountID        = "default"
	defaultTimeout          = 5 * time.Second
	defaultHealthCheckModel = "claude-haiku-4-5-20251001"
)

// NodeRecord mirrors persistent fields for a proxy node.
type NodeRecord struct {
	ID                string
	Name              string
	BaseURL           string
	APIKey            string
	HealthCheckMethod string
	HealthCheckModel  string
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

// HealthCheckRecord 健康检查历史记录
type HealthCheckRecord struct {
	ID             int64
	AccountID      string
	NodeID         string
	CheckTime      time.Time
	Success        bool
	ResponseTimeMs int
	ErrorMessage   string
	CheckMethod    string
	CheckSource    string
	CreatedAt      time.Time
}

// QueryHealthCheckParams 查询参数
type QueryHealthCheckParams struct {
	AccountID   string
	NodeID      string
	From        time.Time
	To          time.Time
	Limit       int
	Offset      int
	CheckSource string
}

// MetricsGranularity 描述查询或聚合的时间粒度。
type MetricsGranularity string

const (
	MetricsGranularityRaw     MetricsGranularity = "raw"
	MetricsGranularityHourly  MetricsGranularity = "hour"
	MetricsGranularityDaily   MetricsGranularity = "day"
	MetricsGranularityMonthly MetricsGranularity = "month"
)

// MetricsRecord 表示单次请求或采样点的原始监控数据。
// 所有时间相关字段均使用 UTC 存储，便于跨地域查询。
type MetricsRecord struct {
	ID                  int64
	AccountID           string
	NodeID              string
	Timestamp           time.Time
	RequestsTotal       int64
	RequestsSuccess     int64
	RequestsFailed      int64
	RetryAttemptsTotal  int64
	RetrySuccess        int64
	ResponseTimeSumMs   int64 // 总响应耗时（毫秒），配合 ResponseTimeCount 计算平均值
	ResponseTimeCount   int64
	BytesTotal          int64
	InputTokensTotal    int64
	OutputTokensTotal   int64
	FirstByteTimeSumMs  int64 // 首字节时间总和（毫秒）
	StreamDurationSumMs int64 // 流式持续时间总和（毫秒）
	CreatedAt           time.Time
}

// MetricsHourly 表示小时级聚合数据（半开区间 [BucketStart, BucketStart+1h)）。
type MetricsHourly struct {
	AccountID           string
	NodeID              string
	BucketStart         time.Time
	RequestsTotal       int64
	RequestsSuccess     int64
	RequestsFailed      int64
	RetryAttemptsTotal  int64
	RetrySuccess        int64
	ResponseTimeSumMs   int64
	ResponseTimeCount   int64
	BytesTotal          int64
	InputTokensTotal    int64
	OutputTokensTotal   int64
	FirstByteTimeSumMs  int64
	StreamDurationSumMs int64
}

// MetricsDaily 表示天级聚合数据（UTC 零点对齐）。
type MetricsDaily struct {
	AccountID           string
	NodeID              string
	BucketStart         time.Time
	RequestsTotal       int64
	RequestsSuccess     int64
	RequestsFailed      int64
	RetryAttemptsTotal  int64
	RetrySuccess        int64
	ResponseTimeSumMs   int64
	ResponseTimeCount   int64
	BytesTotal          int64
	InputTokensTotal    int64
	OutputTokensTotal   int64
	FirstByteTimeSumMs  int64
	StreamDurationSumMs int64
}

// MetricsMonthly 表示月级聚合数据（UTC 月初对齐）。
type MetricsMonthly struct {
	AccountID           string
	NodeID              string
	BucketStart         time.Time
	RequestsTotal       int64
	RequestsSuccess     int64
	RequestsFailed      int64
	RetryAttemptsTotal  int64
	RetrySuccess        int64
	ResponseTimeSumMs   int64
	ResponseTimeCount   int64
	BytesTotal          int64
	InputTokensTotal    int64
	OutputTokensTotal   int64
	FirstByteTimeSumMs  int64
	StreamDurationSumMs int64
}

// MetricsQuery 描述监控数据查询参数。
type MetricsQuery struct {
	AccountID   string
	NodeID      string
	From        time.Time
	To          time.Time
	Granularity MetricsGranularity
	Limit       int
	Offset      int
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

var (
	ErrNotFound        = errors.New("not found")
	ErrVersionConflict = errors.New("version conflict")
)

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

type MonitorShareRecord struct {
	ID        string     `json:"id"`
	AccountID string     `json:"account_id"`
	Token     string     `json:"token"` // UUID token
	ExpireAt  time.Time  `json:"expire_at"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	Revoked   bool       `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// QueryMonitorSharesParams 查询参数
type QueryMonitorSharesParams struct {
	AccountID      string
	IncludeRevoked bool
	Limit          int
	Offset         int
}

// ModelPricingRecord 模型定价记录（按 MTok 计费，美元）
type ModelPricingRecord struct {
	ID              string    `json:"id"`
	ModelID         string    `json:"model_id"`          // 模型 ID（如 claude-opus-4-5-20251101）
	ModelName       string    `json:"model_name"`        // 显示名称（如 Claude Opus 4.5）
	InputPriceMTok  float64   `json:"input_price_mtok"`  // 输入价格 $/MTok
	OutputPriceMTok float64   `json:"output_price_mtok"` // 输出价格 $/MTok
	IsActive        bool      `json:"is_active"`         // 是否启用
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UsageLogRecord 使用日志记录
type UsageLogRecord struct {
	ID           int64     `json:"id"`
	AccountID    string    `json:"account_id"`
	NodeID       string    `json:"node_id"`
	ModelID      string    `json:"model_id"`       // 使用的模型
	InputTokens  int64     `json:"input_tokens"`   // 输入 tokens
	OutputTokens int64     `json:"output_tokens"`  // 输出 tokens
	CostUSD      float64   `json:"cost_usd"`       // 费用（美元）
	RequestID    string    `json:"request_id"`     // 请求 ID（可选，用于追踪）
	Success      bool      `json:"success"`        // 请求是否成功
	CreatedAt    time.Time `json:"created_at"`
}

// UsageSummary 使用汇总统计
type UsageSummary struct {
	AccountID         string  `json:"account_id"`
	NodeID            string  `json:"node_id,omitempty"`
	ModelID           string  `json:"model_id,omitempty"`
	TotalRequests     int64   `json:"total_requests"`
	SuccessRequests   int64   `json:"success_requests"`
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
}

// QueryUsageParams 查询使用日志参数
type QueryUsageParams struct {
	AccountID string
	NodeID    string
	ModelID   string
	From      time.Time
	To        time.Time
	Limit     int
	Offset    int
}
