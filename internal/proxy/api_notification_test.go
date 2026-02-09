package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
)

func TestHandleNotificationChannels(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{"GET method", http.MethodGet, http.StatusOK},
		{"POST method", http.MethodPost, http.StatusCreated},
		{"PUT method not allowed", http.MethodPut, http.StatusMethodNotAllowed},
		{"DELETE method not allowed", http.MethodDelete, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			adminAcc := getAdminAccount(t, srv)
			adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

			var body string
			if tt.method == http.MethodPost {
				body = `{"name":"test","channel_type":"wechat_work","config":{"webhook_url":"https://example.com/webhook"}}`
			}

			req := httptest.NewRequest(tt.method, "/api/notification/channels", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestListNotificationChannels(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a channel first
	channelRec := store.NotificationChannelRecord{
		ID:          "chn-test-123",
		AccountID:   adminAcc.ID,
		ChannelType: notify.ChannelWechatWork,
		Name:        "Test Channel",
		Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := srv.store.CreateNotificationChannel(context.Background(), channelRec); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	tests := []struct {
		name           string
		queryParams    string
		setupFunc      func(*Server) string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:        "list channels for admin",
			queryParams: "",
			setupFunc:   func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				channels, ok := resp["channels"].([]interface{})
				if !ok {
					t.Error("expected 'channels' array")
				}
				if len(channels) == 0 {
					t.Error("expected at least one channel")
				}
			},
		},
		{
			name:           "unauthorized without session",
			queryParams:    "",
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodGet, "/api/notification/channels"+tt.queryParams, nil)
			if token != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			}
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestCreateNotificationChannel(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		setupFunc      func(*Server) string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "create wechat work channel",
			body: `{"name":"test","channel_type":"wechat_work","config":{"webhook_url":"https://example.com/webhook"}}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["id"]; !ok {
					t.Error("expected 'id' field")
				}
				if resp["name"] != "test" {
					t.Error("expected name 'test'")
				}
			},
		},
		{
			name: "create wechat personal channel",
			body: `{"name":"personal","channel_type":"wechat_personal","config":{"webhook_url":"https://example.com/webhook"}}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create channel with enabled false",
			body: `{"name":"disabled","channel_type":"wechat_work","config":{"webhook_url":"https://example.com/webhook"},"enabled":false}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create channel without name uses channel_type",
			body: `{"channel_type":"wechat_work","config":{"webhook_url":"https://example.com/webhook"}}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid JSON",
			body:           `{invalid}`,
			setupFunc:      func(srv *Server) string { return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unsupported channel type",
			body:           `{"name":"test","channel_type":"invalid","config":{"webhook_url":"https://example.com/webhook"}}`,
			setupFunc:      func(srv *Server) string { return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing webhook_url",
			body:           `{"name":"test","channel_type":"wechat_work","config":{}}`,
			setupFunc:      func(srv *Server) string { return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid webhook_url",
			body:           `{"name":"test","channel_type":"wechat_work","config":{"webhook_url":"invalid-url"}}`,
			setupFunc:      func(srv *Server) string { return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized without session",
			body:           `{"name":"test","channel_type":"wechat_work","config":{"webhook_url":"https://example.com/webhook"}}`,
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodPost, "/api/notification/channels", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			}
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.checkResponse != nil && w.Code == http.StatusCreated {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestUpdateNotificationChannel(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a channel to update
	channelRec := store.NotificationChannelRecord{
		ID:          "chn-update-test",
		AccountID:   adminAcc.ID,
		ChannelType: notify.ChannelWechatWork,
		Name:        "Original Name",
		Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := srv.store.CreateNotificationChannel(context.Background(), channelRec); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	tests := []struct {
		name           string
		channelID      string
		body           string
		setupFunc      func(*Server) string
		expectedStatus int
	}{
		{
			name:      "update channel name",
			channelID: "chn-update-test",
			body:      `{"name":"Updated Name"}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:      "update channel enabled status",
			channelID: "chn-update-test",
			body:      `{"enabled":false}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:      "update channel config",
			channelID: "chn-update-test",
			body:      `{"config":{"webhook_url":"https://new.example.com/webhook"}}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:      "update non-existent channel",
			channelID: "non-existent",
			body:      `{"name":"Updated"}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "invalid JSON",
			channelID: "chn-update-test",
			body:      `{invalid}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "unauthorized without session",
			channelID: "chn-update-test",
			body:      `{"name":"Updated"}`,
			setupFunc: func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodPut, "/api/notification/channels/"+tt.channelID, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			}
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestDeleteNotificationChannel(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a channel to delete
	channelRec := store.NotificationChannelRecord{
		ID:          "chn-delete-test",
		AccountID:   adminAcc.ID,
		ChannelType: notify.ChannelWechatWork,
		Name:        "To Delete",
		Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := srv.store.CreateNotificationChannel(context.Background(), channelRec); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	tests := []struct {
		name           string
		channelID      string
		setupFunc      func(*Server) string
		expectedStatus int
	}{
		{
			name:      "delete existing channel",
			channelID: "chn-delete-test",
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:      "delete non-existent channel",
			channelID: "non-existent",
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "unauthorized without session",
			channelID: "chn-delete-test",
			setupFunc: func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodDelete, "/api/notification/channels/"+tt.channelID, nil)
			if token != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			}
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestListNotificationSubscriptions(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a channel and subscription
	channelRec := store.NotificationChannelRecord{
		ID:          "chn-sub-test",
		AccountID:   adminAcc.ID,
		ChannelType: notify.ChannelWechatWork,
		Name:        "Test Channel",
		Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := srv.store.CreateNotificationChannel(context.Background(), channelRec); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	subRec := store.NotificationSubscriptionRecord{
		ID:        "sub-test-123",
		AccountID: adminAcc.ID,
		ChannelID: "chn-sub-test",
		EventType: notify.EventNodeFailed,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := srv.store.UpsertNotificationSubscription(context.Background(), subRec); err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	tests := []struct {
		name           string
		queryParams    string
		setupFunc      func(*Server) string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:        "list subscriptions",
			queryParams: "",
			setupFunc:   func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				subs, ok := resp["subscriptions"].([]interface{})
				if !ok {
					t.Error("expected 'subscriptions' array")
				}
				if len(subs) == 0 {
					t.Error("expected at least one subscription")
				}
			},
		},
		{
			name:        "list subscriptions by channel",
			queryParams: "?channel_id=chn-sub-test",
			setupFunc:   func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthorized without session",
			queryParams:    "",
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodGet, "/api/notification/subscriptions"+tt.queryParams, nil)
			if token != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			}
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestCreateNotificationSubscription(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a channel
	channelRec := store.NotificationChannelRecord{
		ID:          "chn-create-sub",
		AccountID:   adminAcc.ID,
		ChannelType: notify.ChannelWechatWork,
		Name:        "Test Channel",
		Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := srv.store.CreateNotificationChannel(context.Background(), channelRec); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	tests := []struct {
		name           string
		body           string
		setupFunc      func(*Server) string
		expectedStatus int
	}{
		{
			name: "create subscription",
			body: `{"channel_id":"chn-create-sub","event_types":["node.failed","node.recovered"]}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create subscription with enabled false",
			body: `{"channel_id":"chn-create-sub","event_types":["node.failed"],"enabled":false}`,
			setupFunc: func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing channel_id",
			body:           `{"event_types":["node.failed"]}`,
			setupFunc:      func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing event_types",
			body:           `{"channel_id":"chn-create-sub"}`,
			setupFunc:      func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid event type",
			body:           `{"channel_id":"chn-create-sub","event_types":["invalid.event"]}`,
			setupFunc:      func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-existent channel",
			body:           `{"channel_id":"non-existent","event_types":["node.failed"]}`,
			setupFunc:      func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized without session",
			body:           `{"channel_id":"chn-create-sub","event_types":["node.failed"]}`,
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodPost, "/api/notification/subscriptions", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			}
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestValidateChannelConfig(t *testing.T) {
	tests := []struct {
		name        string
		channelType string
		config      string
		expectError bool
	}{
		{
			name:        "valid wechat work config",
			channelType: notify.ChannelWechatWork,
			config:      `{"webhook_url":"https://example.com/webhook"}`,
			expectError: false,
		},
		{
			name:        "valid wechat personal config",
			channelType: notify.ChannelWechatPersonal,
			config:      `{"webhook_url":"http://example.com/webhook"}`,
			expectError: false,
		},
		{
			name:        "missing webhook_url",
			channelType: notify.ChannelWechatWork,
			config:      `{}`,
			expectError: true,
		},
		{
			name:        "invalid webhook_url",
			channelType: notify.ChannelWechatWork,
			config:      `{"webhook_url":"invalid"}`,
			expectError: true,
		},
		{
			name:        "empty config",
			channelType: notify.ChannelWechatWork,
			config:      ``,
			expectError: true,
		},
		{
			name:        "unsupported channel type",
			channelType: "unsupported",
			config:      `{"webhook_url":"https://example.com/webhook"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateChannelConfig(tt.channelType, json.RawMessage(tt.config))

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{"valid https URL", "https://example.com/webhook", false},
		{"valid http URL", "http://example.com/webhook", false},
		{"empty URL", "", true},
		{"invalid URL", "invalid", true},
		{"ftp URL", "ftp://example.com", true},
		{"URL without scheme", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsValidEventType(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		expected  bool
	}{
		{"valid node failed event", notify.EventNodeFailed, true},
		{"valid node recovered event", notify.EventNodeRecovered, true},
		{"valid request failed event", notify.EventRequestFailed, true},
		{"invalid event type", "invalid.event", false},
		{"empty event type", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEventType(tt.eventType)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
