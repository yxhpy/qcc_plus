package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestNewWechatChannel(t *testing.T) {
	tests := []struct {
		name    string
		record  store.NotificationChannelRecord
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with webhook_url",
			record: store.NotificationChannelRecord{
				ID:          "ch1",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Test Channel",
				Config:      json.RawMessage(`{"webhook_url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test"}`),
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "success with webhook_url and token",
			record: store.NotificationChannelRecord{
				ID:          "ch2",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Test Channel with Token",
				Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook","token":"secret123"}`),
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "error - missing webhook_url",
			record: store.NotificationChannelRecord{
				ID:          "ch3",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "No URL",
				Config:      json.RawMessage(`{}`),
				Enabled:     true,
			},
			wantErr: true,
			errMsg:  "webhook_url required",
		},
		{
			name: "error - empty config",
			record: store.NotificationChannelRecord{
				ID:          "ch4",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Empty Config",
				Config:      json.RawMessage(``),
				Enabled:     true,
			},
			wantErr: true,
			errMsg:  "webhook_url required",
		},
		{
			name: "error - invalid json",
			record: store.NotificationChannelRecord{
				ID:          "ch5",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Invalid JSON",
				Config:      json.RawMessage(`{invalid json}`),
				Enabled:     true,
			},
			wantErr: true,
			errMsg:  "parse wechat config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := newWechatChannel(tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("newWechatChannel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err == nil {
					t.Error("newWechatChannel() expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("newWechatChannel() error = %v, should contain %s", err, tt.errMsg)
				}
			} else {
				if ch == nil {
					t.Error("newWechatChannel() should not return nil on success")
				}
				wc, ok := ch.(*wechatChannel)
				if !ok {
					t.Error("newWechatChannel() should return *wechatChannel")
				}
				if wc.cfg.WebhookURL == "" {
					t.Error("wechatChannel should have webhook_url")
				}
				if wc.client == nil {
					t.Error("wechatChannel should have http client")
				}
			}
		})
	}
}

func TestWechatChannelSend(t *testing.T) {
	tests := []struct {
		name       string
		msg        NotificationMessage
		ctx        context.Context
		serverFunc func(w http.ResponseWriter, r *http.Request)
		wantErr    bool
		errMsg     string
	}{
		{
			name: "success",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Node Failed",
				Content:    "Node 1 has failed",
				OccurredAt: time.Now(),
			},
			ctx: context.Background(),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Read and verify body
				body, _ := io.ReadAll(r.Body)
				var req map[string]interface{}
				if err := json.Unmarshal(body, &req); err != nil {
					t.Errorf("invalid JSON body: %v", err)
				}
				if req["msgtype"] != "markdown" {
					t.Errorf("expected msgtype markdown, got %v", req["msgtype"])
				}

				// Return success
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"errcode": 0,
					"errmsg":  "ok",
				})
			},
			wantErr: false,
		},
		{
			name: "success with nil context",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeRecovered,
				Title:      "Node Recovered",
				Content:    "Node 1 has recovered",
				OccurredAt: time.Now(),
			},
			ctx: nil, // Test nil context handling
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"errcode": 0,
					"errmsg":  "ok",
				})
			},
			wantErr: false,
		},
		{
			name: "error - non-200 status",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Test",
				Content:    "Test",
				OccurredAt: time.Now(),
			},
			ctx: context.Background(),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
			errMsg:  "status 500",
		},
		{
			name: "error - wechat api error",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Test",
				Content:    "Test",
				OccurredAt: time.Now(),
			},
			ctx: context.Background(),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"errcode": 40001,
					"errmsg":  "invalid webhook key",
				})
			},
			wantErr: true,
			errMsg:  "webhook error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			// Create wechat channel
			record := store.NotificationChannelRecord{
				ID:          "ch1",
				AccountID:   "acc1",
				ChannelType: ChannelWechatWork,
				Name:        "Test",
				Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
				Enabled:     true,
			}

			ch, err := newWechatChannel(record)
			if err != nil {
				t.Fatalf("newWechatChannel() unexpected error: %v", err)
			}

			// Send message
			err = ch.Send(tt.ctx, tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil {
					t.Error("Send() expected error but got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Send() error = %v, should contain %s", err, tt.errMsg)
				}
			}
		})
	}
}

func TestWechatChannelSendTimeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errcode": 0,
			"errmsg":  "ok",
		})
	}))
	defer server.Close()

	record := store.NotificationChannelRecord{
		ID:          "ch1",
		AccountID:   "acc1",
		ChannelType: ChannelWechatWork,
		Name:        "Test",
		Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
		Enabled:     true,
	}

	ch, err := newWechatChannel(record)
	if err != nil {
		t.Fatalf("newWechatChannel() unexpected error: %v", err)
	}

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	msg := NotificationMessage{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		OccurredAt: time.Now(),
	}

	err = ch.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should return error on timeout")
	}
}

func TestFormatWechatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		msg      NotificationMessage
		contains []string
	}{
		{
			name: "full message",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Node Failed",
				Content:    "Node 1 has failed",
				OccurredAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			contains: []string{"Node Failed", EventNodeFailed, "Node 1 has failed"},
		},
		{
			name: "empty title uses event type",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeRecovered,
				Title:      "",
				Content:    "Node recovered",
				OccurredAt: time.Now(),
			},
			contains: []string{EventNodeRecovered, "Node recovered"},
		},
		{
			name: "empty content shows placeholder",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Test",
				Content:    "",
				OccurredAt: time.Now(),
			},
			contains: []string{"Test", "_无详细内容_"},
		},
		{
			name: "both empty",
			msg: NotificationMessage{
				AccountID:  "acc1",
				EventType:  EventSystemError,
				Title:      "",
				Content:    "",
				OccurredAt: time.Now(),
			},
			contains: []string{EventSystemError, "_无详细内容_"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWechatMarkdown(tt.msg)
			if result == "" {
				t.Error("formatWechatMarkdown() should not return empty string")
			}
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("formatWechatMarkdown() result should contain %s, got: %s", substr, result)
				}
			}
		})
	}
}

func TestWechatConfig(t *testing.T) {
	cfg := wechatConfig{
		WebhookURL: "https://example.com/webhook",
		Token:      "secret123",
	}

	if cfg.WebhookURL != "https://example.com/webhook" {
		t.Errorf("expected WebhookURL https://example.com/webhook, got %s", cfg.WebhookURL)
	}
	if cfg.Token != "secret123" {
		t.Errorf("expected Token secret123, got %s", cfg.Token)
	}

	// Test JSON marshaling
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Errorf("json.Marshal() unexpected error: %v", err)
	}

	var cfg2 wechatConfig
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Errorf("json.Unmarshal() unexpected error: %v", err)
	}

	if cfg2.WebhookURL != cfg.WebhookURL {
		t.Errorf("expected WebhookURL %s, got %s", cfg.WebhookURL, cfg2.WebhookURL)
	}
	if cfg2.Token != cfg.Token {
		t.Errorf("expected Token %s, got %s", cfg.Token, cfg2.Token)
	}
}

func TestWechatChannelSendInvalidURL(t *testing.T) {
	// Test with invalid URL that will cause NewRequestWithContext to fail
	record := store.NotificationChannelRecord{
		ID:          "ch1",
		AccountID:   "acc1",
		ChannelType: ChannelWechatWork,
		Name:        "Test",
		Config:      json.RawMessage(`{"webhook_url":"ht tp://invalid url with spaces"}`),
		Enabled:     true,
	}

	ch, err := newWechatChannel(record)
	if err != nil {
		t.Fatalf("newWechatChannel() unexpected error: %v", err)
	}

	msg := NotificationMessage{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		OccurredAt: time.Now(),
	}

	err = ch.Send(context.Background(), msg)
	if err == nil {
		t.Error("Send() should return error for invalid URL")
	}
}

func TestWechatChannelSendInvalidJSON(t *testing.T) {
	// Test response with invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	record := store.NotificationChannelRecord{
		ID:          "ch1",
		AccountID:   "acc1",
		ChannelType: ChannelWechatWork,
		Name:        "Test",
		Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
		Enabled:     true,
	}

	ch, err := newWechatChannel(record)
	if err != nil {
		t.Fatalf("newWechatChannel() unexpected error: %v", err)
	}

	msg := NotificationMessage{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		OccurredAt: time.Now(),
	}

	// Should not error even if JSON decode fails (line 77 checks err == nil)
	err = ch.Send(context.Background(), msg)
	if err != nil {
		t.Errorf("Send() unexpected error: %v", err)
	}
}
