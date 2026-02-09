package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestBuildChannel(t *testing.T) {
	tests := []struct {
		name    string
		record  store.NotificationChannelRecord
		wantErr bool
		errMsg  string
	}{
		{
			name: "wechat_work channel success",
			record: store.NotificationChannelRecord{
				ID:          "ch1",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Wechat Work",
				Config:      json.RawMessage(`{"webhook_url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test"}`),
				Enabled:     true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "wechat_personal channel success",
			record: store.NotificationChannelRecord{
				ID:          "ch2",
				AccountID:   "acc1",
				ChannelType: ChannelWechatPersonal,
				Name:        "Wechat Personal",
				Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
				Enabled:     true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "unsupported channel type",
			record: store.NotificationChannelRecord{
				ID:          "ch3",
				AccountID:   "acc1",
				ChannelType: ChannelEmail,
				Name:        "Email",
				Config:      json.RawMessage(`{"smtp":"smtp.example.com"}`),
				Enabled:     true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: true,
			errMsg:  "unsupported channel type",
		},
		{
			name: "invalid channel type",
			record: store.NotificationChannelRecord{
				ID:          "ch4",
				AccountID:   "acc1",
				ChannelType: "invalid_type",
				Name:        "Invalid",
				Config:      json.RawMessage(`{}`),
				Enabled:     true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: true,
			errMsg:  "unsupported channel type",
		},
		{
			name: "wechat without webhook_url",
			record: store.NotificationChannelRecord{
				ID:          "ch5",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Wechat No URL",
				Config:      json.RawMessage(`{}`),
				Enabled:     true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: true,
			errMsg:  "webhook_url required",
		},
		{
			name: "wechat with invalid json",
			record: store.NotificationChannelRecord{
				ID:          "ch6",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Wechat Invalid JSON",
				Config:      json.RawMessage(`{invalid json}`),
				Enabled:     true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: true,
			errMsg:  "parse wechat config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := buildChannel(tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildChannel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err == nil {
					t.Error("buildChannel() expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("buildChannel() error = %v, should contain %s", err, tt.errMsg)
				}
			} else {
				if ch == nil {
					t.Error("buildChannel() should not return nil channel on success")
				}
			}
		})
	}
}

func TestBuildChannelPublic(t *testing.T) {
	// Test the public BuildChannel function
	record := store.NotificationChannelRecord{
		ID:          "ch1",
		AccountID:   "acc1",
		ChannelType: ChannelWechatWork,
		Name:        "Test",
		Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ch, err := BuildChannel(record)
	if err != nil {
		t.Errorf("BuildChannel() unexpected error: %v", err)
	}
	if ch == nil {
		t.Error("BuildChannel() should not return nil")
	}
}

func TestNotificationMessage(t *testing.T) {
	now := time.Now()
	msg := NotificationMessage{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Node Failed",
		Content:    "Node 1 has failed",
		OccurredAt: now,
	}

	if msg.AccountID != "acc1" {
		t.Errorf("expected AccountID acc1, got %s", msg.AccountID)
	}
	if msg.EventType != EventNodeFailed {
		t.Errorf("expected EventType %s, got %s", EventNodeFailed, msg.EventType)
	}
	if msg.Title != "Node Failed" {
		t.Errorf("expected Title 'Node Failed', got %s", msg.Title)
	}
	if msg.Content != "Node 1 has failed" {
		t.Errorf("expected Content 'Node 1 has failed', got %s", msg.Content)
	}
	if !msg.OccurredAt.Equal(now) {
		t.Errorf("expected OccurredAt %v, got %v", now, msg.OccurredAt)
	}
}

// mockChannel implements NotificationChannel for testing
type mockChannel struct {
	sendErr    error
	sendCalls  []NotificationMessage
	sendCalled bool
}

func (m *mockChannel) Send(ctx context.Context, msg NotificationMessage) error {
	m.sendCalled = true
	m.sendCalls = append(m.sendCalls, msg)
	return m.sendErr
}

func TestNotificationChannelInterface(t *testing.T) {
	var _ NotificationChannel = (*mockChannel)(nil)

	mock := &mockChannel{}
	msg := NotificationMessage{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test content",
		OccurredAt: time.Now(),
	}

	err := mock.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("Send() unexpected error: %v", err)
	}
	if !mock.sendCalled {
		t.Error("Send() should have been called")
	}
	if len(mock.sendCalls) != 1 {
		t.Errorf("expected 1 send call, got %d", len(mock.sendCalls))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
