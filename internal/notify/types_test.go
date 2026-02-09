package notify

import (
	"testing"
	"time"
)

func TestEventConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"NodeStatusChanged", EventNodeStatusChanged, "node.status_changed"},
		{"NodeSwitched", EventNodeSwitched, "node.switched"},
		{"NodeFailed", EventNodeFailed, "node.failed"},
		{"NodeRecovered", EventNodeRecovered, "node.recovered"},
		{"NodeAdded", EventNodeAdded, "node.added"},
		{"NodeDeleted", EventNodeDeleted, "node.deleted"},
		{"NodeUpdated", EventNodeUpdated, "node.updated"},
		{"NodeEnabled", EventNodeEnabled, "node.enabled"},
		{"NodeDisabled", EventNodeDisabled, "node.disabled"},
		{"NodeHealthCheckError", EventNodeHealthCheckError, "node.health_check_failed"},
		{"RequestFailed", EventRequestFailed, "request.failed"},
		{"RequestUpstreamErr", EventRequestUpstreamErr, "request.upstream_error"},
		{"RequestProxyError", EventRequestProxyError, "request.proxy_error"},
		{"AccountQuotaWarning", EventAccountQuotaWarning, "account.quota_warning"},
		{"AccountAuthFailed", EventAccountAuthFailed, "account.auth_failed"},
		{"SystemTunnelStarted", EventSystemTunnelStarted, "system.tunnel_started"},
		{"SystemTunnelStopped", EventSystemTunnelStopped, "system.tunnel_stopped"},
		{"SystemTunnelError", EventSystemTunnelError, "system.tunnel_error"},
		{"SystemError", EventSystemError, "system.error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.constant)
			}
		})
	}
}

func TestChannelConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"WechatWork", ChannelWechatWork, "wechat_work"},
		{"WechatPersonal", ChannelWechatPersonal, "wechat_personal"},
		{"Email", ChannelEmail, "email"},
		{"DingTalk", ChannelDingTalk, "dingtalk"},
		{"Slack", ChannelSlack, "slack"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.constant)
			}
		})
	}
}

func TestEventStruct(t *testing.T) {
	now := time.Now()
	evt := Event{
		AccountID:  "acc123",
		EventType:  EventNodeFailed,
		Title:      "Node Failed",
		Content:    "Node 1 has failed",
		DedupKey:   "node1",
		OccurredAt: now,
	}

	if evt.AccountID != "acc123" {
		t.Errorf("expected AccountID acc123, got %s", evt.AccountID)
	}
	if evt.EventType != EventNodeFailed {
		t.Errorf("expected EventType %s, got %s", EventNodeFailed, evt.EventType)
	}
	if evt.Title != "Node Failed" {
		t.Errorf("expected Title 'Node Failed', got %s", evt.Title)
	}
	if evt.Content != "Node 1 has failed" {
		t.Errorf("expected Content 'Node 1 has failed', got %s", evt.Content)
	}
	if evt.DedupKey != "node1" {
		t.Errorf("expected DedupKey 'node1', got %s", evt.DedupKey)
	}
	if !evt.OccurredAt.Equal(now) {
		t.Errorf("expected OccurredAt %v, got %v", now, evt.OccurredAt)
	}
}

func TestManagerConfig(t *testing.T) {
	cfg := ManagerConfig{
		QueueSize:   256,
		WorkerCount: 4,
		DedupWindow: 10 * time.Minute,
		SendTimeout: 5 * time.Second,
	}

	if cfg.QueueSize != 256 {
		t.Errorf("expected QueueSize 256, got %d", cfg.QueueSize)
	}
	if cfg.WorkerCount != 4 {
		t.Errorf("expected WorkerCount 4, got %d", cfg.WorkerCount)
	}
	if cfg.DedupWindow != 10*time.Minute {
		t.Errorf("expected DedupWindow 10m, got %v", cfg.DedupWindow)
	}
	if cfg.SendTimeout != 5*time.Second {
		t.Errorf("expected SendTimeout 5s, got %v", cfg.SendTimeout)
	}
}

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	// Store formatted message for testing
	m.messages = append(m.messages, format)
}

func TestLoggerInterface(t *testing.T) {
	logger := &mockLogger{}
	logger.Printf("test message: %s", "hello")

	if len(logger.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(logger.messages))
	}
	if logger.messages[0] != "test message: %s" {
		t.Errorf("expected 'test message: %%s', got %s", logger.messages[0])
	}
}
