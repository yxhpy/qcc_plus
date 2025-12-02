package notify

import "time"

// 事件类型常量。
const (
	// 节点相关
	EventNodeStatusChanged    = "node.status_changed"
	EventNodeSwitched         = "node.switched"
	EventNodeFailed           = "node.failed"
	EventNodeRecovered        = "node.recovered"
	EventNodeAdded            = "node.added"
	EventNodeDeleted          = "node.deleted"
	EventNodeUpdated          = "node.updated"
	EventNodeEnabled          = "node.enabled"
	EventNodeDisabled         = "node.disabled"
	EventNodeHealthCheckError = "node.health_check_failed"

	// 请求相关
	EventRequestFailed      = "request.failed"
	EventRequestUpstreamErr = "request.upstream_error"
	EventRequestProxyError  = "request.proxy_error"

	// 账号相关
	EventAccountQuotaWarning = "account.quota_warning"
	EventAccountAuthFailed   = "account.auth_failed"

	// 系统相关
	EventSystemTunnelStarted = "system.tunnel_started"
	EventSystemTunnelStopped = "system.tunnel_stopped"
	EventSystemTunnelError   = "system.tunnel_error"
	EventSystemError         = "system.error"
)

// 渠道类型。
const (
	ChannelWechatWork     = "wechat_work"
	ChannelWechatPersonal = "wechat_personal"
	ChannelEmail          = "email"
	ChannelDingTalk       = "dingtalk"
	ChannelSlack          = "slack"
)

// Event 表示一条需要发送的通知事件。
type Event struct {
	AccountID  string
	EventType  string
	Title      string
	Content    string
	DedupKey   string
	OccurredAt time.Time
}

// ManagerConfig 控制通知管理器的运行参数。
type ManagerConfig struct {
	QueueSize   int
	WorkerCount int
	DedupWindow time.Duration
	Logger      Logger
	SendTimeout time.Duration
}

// Logger 抽象日志接口，兼容标准 log.Logger。
type Logger interface {
	Printf(format string, v ...interface{})
}
