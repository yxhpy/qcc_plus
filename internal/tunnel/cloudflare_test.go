package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		apiToken string
	}{
		{
			name:     "有效token",
			apiToken: "test-token-123",
		},
		{
			name:     "空token",
			apiToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.apiToken)
			if client == nil {
				t.Fatal("NewClient() returned nil")
			}
			if client.apiToken != tt.apiToken {
				t.Errorf("apiToken = %v, want %v", client.apiToken, tt.apiToken)
			}
			if client.baseURL != cfBaseURL {
				t.Errorf("baseURL = %v, want %v", client.baseURL, cfBaseURL)
			}
			if client.httpClient == nil {
				t.Error("httpClient is nil")
			}
			if client.httpClient.Timeout != 15*time.Second {
				t.Errorf("httpClient.Timeout = %v, want %v", client.httpClient.Timeout, 15*time.Second)
			}
		})
	}
}

func TestClient_doRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name:   "成功GET请求",
			method: http.MethodGet,
			path:   "/test",
			body:   nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Method = %v, want %v", r.Method, http.MethodGet)
				}
				if !strings.Contains(r.Header.Get("Authorization"), "Bearer") {
					t.Error("Missing Bearer token in Authorization header")
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"success": true})
			},
			wantErr: false,
		},
		{
			name:   "成功POST请求带body",
			method: http.MethodPost,
			path:   "/test",
			body:   map[string]string{"key": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Method = %v, want %v", r.Method, http.MethodPost)
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("Failed to decode body: %v", err)
				}
				if body["key"] != "value" {
					t.Errorf("body[key] = %v, want value", body["key"])
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"success": true})
			},
			wantErr: false,
		},
		{
			name:   "HTTP 400错误",
			method: http.MethodGet,
			path:   "/test",
			body:   nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("bad request"))
			},
			wantErr:    true,
			errContain: "cloudflare api error",
		},
		{
			name:   "HTTP 500错误",
			method: http.MethodGet,
			path:   "/test",
			body:   nil,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			wantErr:    true,
			errContain: "cloudflare api error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			var result map[string]any
			err := client.doRequest(context.Background(), tt.method, tt.path, tt.body, &result)

			if (err != nil) != tt.wantErr {
				t.Errorf("doRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
			}
		})
	}
}

func TestClient_doRequest_NilClient(t *testing.T) {
	var client *Client
	err := client.doRequest(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Error("doRequest() with nil client should return error")
	}
	if !strings.Contains(err.Error(), "nil cloudflare client") {
		t.Errorf("error = %v, want to contain 'nil cloudflare client'", err)
	}
}

func TestClient_doRequest_NilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	// nil context should be handled gracefully
	err := client.doRequest(nil, http.MethodGet, "/test", nil, nil)
	if err != nil {
		t.Errorf("doRequest() with nil context should not error, got %v", err)
	}
}

func TestGatherErrors(t *testing.T) {
	tests := []struct {
		name       string
		resp       cfResponse[any]
		wantErr    bool
		errContain string
	}{
		{
			name: "成功响应",
			resp: cfResponse[any]{
				Success: true,
				Errors:  nil,
			},
			wantErr: false,
		},
		{
			name: "单个错误",
			resp: cfResponse[any]{
				Success: false,
				Errors: []cfError{
					{Message: "error 1"},
				},
			},
			wantErr:    true,
			errContain: "error 1",
		},
		{
			name: "多个错误",
			resp: cfResponse[any]{
				Success: false,
				Errors: []cfError{
					{Message: "error 1"},
					{Message: "error 2"},
				},
			},
			wantErr:    true,
			errContain: "error 1; error 2",
		},
		{
			name: "失败但无错误信息",
			resp: cfResponse[any]{
				Success: false,
				Errors:  []cfError{},
			},
			wantErr:    true,
			errContain: "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gatherErrors(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("gatherErrors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
			}
		})
	}
}

func TestClient_ListZones(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantZones  int
		wantErr    bool
		errContain string
	}{
		{
			name: "成功获取zones",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}]{
					Success: true,
					Result: []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{
						{ID: "zone1", Name: "example.com"},
						{ID: "zone2", Name: "test.com"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantZones: 2,
			wantErr:   false,
		},
		{
			name: "空zones列表",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}]{
					Success: true,
					Result:  []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantZones: 0,
			wantErr:   false,
		},
		{
			name: "API错误",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}]{
					Success: false,
					Errors: []cfError{
						{Message: "authentication failed"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			zones, err := client.ListZones(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListZones() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
				return
			}
			if !tt.wantErr && len(zones) != tt.wantZones {
				t.Errorf("ListZones() returned %d zones, want %d", len(zones), tt.wantZones)
			}
		})
	}
}

func TestClient_GetAccountID(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantID     string
		wantErr    bool
		errContain string
	}{
		{
			name: "成功获取账号ID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID string `json:"id"`
				}]{
					Success: true,
					Result: []struct {
						ID string `json:"id"`
					}{
						{ID: "account-123"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantID:  "account-123",
			wantErr: false,
		},
		{
			name: "无账号",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID string `json:"id"`
				}]{
					Success: true,
					Result:  []struct {
						ID string `json:"id"`
					}{},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "no cloudflare account found",
		},
		{
			name: "API错误",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID string `json:"id"`
				}]{
					Success: false,
					Errors: []cfError{
						{Message: "invalid token"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			id, err := client.GetAccountID(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAccountID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
				return
			}
			if !tt.wantErr && id != tt.wantID {
				t.Errorf("GetAccountID() = %v, want %v", id, tt.wantID)
			}
		})
	}
}

func TestClient_CreateTunnel(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name: "成功创建隧道",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Method = %v, want POST", r.Method)
				}
				resp := cfResponse[struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Token string `json:"tunnel_token"`
				}]{
					Success: true,
					Result: struct {
						ID    string `json:"id"`
						Name  string `json:"name"`
						Token string `json:"tunnel_token"`
					}{
						ID:    "tunnel-123",
						Name:  "test-tunnel",
						Token: "token-abc",
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "API错误",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Token string `json:"tunnel_token"`
				}]{
					Success: false,
					Errors: []cfError{
						{Message: "tunnel already exists"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "tunnel already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			tunnel, err := client.CreateTunnel(context.Background(), "account-123", "test-tunnel", "secret")

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTunnel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
				return
			}
			if !tt.wantErr {
				if tunnel == nil {
					t.Error("CreateTunnel() returned nil tunnel")
					return
				}
				if tunnel.ID == "" {
					t.Error("tunnel.ID is empty")
				}
				if tunnel.Name == "" {
					t.Error("tunnel.Name is empty")
				}
				if tunnel.Secret != "secret" {
					t.Errorf("tunnel.Secret = %v, want secret", tunnel.Secret)
				}
			}
		})
	}
}

func TestClient_DeleteTunnel(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name: "成功删除隧道",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("Method = %v, want DELETE", r.Method)
				}
				resp := cfResponse[struct{}]{
					Success: true,
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "隧道不存在",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct{}]{
					Success: false,
					Errors: []cfError{
						{Message: "tunnel not found"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "tunnel not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			err := client.DeleteTunnel(context.Background(), "account-123", "tunnel-123")

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteTunnel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
			}
		})
	}
}

func TestClient_GetTunnelToken(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantToken  string
		wantErr    bool
		errContain string
	}{
		{
			name: "成功获取token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[string]{
					Success: true,
					Result:  "token-xyz-123",
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantToken: "token-xyz-123",
			wantErr:   false,
		},
		{
			name: "空token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[string]{
					Success: true,
					Result:  "",
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "empty tunnel token",
		},
		{
			name: "API错误",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[string]{
					Success: false,
					Errors: []cfError{
						{Message: "unauthorized"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			token, err := client.GetTunnelToken(context.Background(), "account-123", "tunnel-123")

			if (err != nil) != tt.wantErr {
				t.Errorf("GetTunnelToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
				return
			}
			if !tt.wantErr && token != tt.wantToken {
				t.Errorf("GetTunnelToken() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestClient_CreateDNSRecord(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name: "成功创建DNS记录",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Method = %v, want POST", r.Method)
				}
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				if body["type"] != "CNAME" {
					t.Errorf("type = %v, want CNAME", body["type"])
				}
				resp := cfResponse[struct {
					ID string `json:"id"`
				}]{
					Success: true,
					Result: struct {
						ID string `json:"id"`
					}{ID: "record-123"},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "DNS记录已存在",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct {
					ID string `json:"id"`
				}]{
					Success: false,
					Errors: []cfError{
						{Message: "record already exists"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "record already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			err := client.CreateDNSRecord(context.Background(), "zone-123", "test.example.com", "tunnel-123")

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDNSRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
			}
		})
	}
}

func TestClient_DeleteDNSRecord(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name: "成功删除DNS记录",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("Method = %v, want DELETE", r.Method)
				}
				resp := cfResponse[struct{}]{
					Success: true,
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "记录不存在",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct{}]{
					Success: false,
					Errors: []cfError{
						{Message: "record not found"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "record not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			err := client.DeleteDNSRecord(context.Background(), "zone-123", "record-123")

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteDNSRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
			}
		})
	}
}

func TestClient_FindDNSRecord(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantRecord bool
		wantErr    bool
		errContain string
	}{
		{
			name: "找到DNS记录",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID      string `json:"id"`
					Type    string `json:"type"`
					Name    string `json:"name"`
					Content string `json:"content"`
				}]{
					Success: true,
					Result: []struct {
						ID      string `json:"id"`
						Type    string `json:"type"`
						Name    string `json:"name"`
						Content string `json:"content"`
					}{
						{
							ID:      "record-123",
							Type:    "CNAME",
							Name:    "test.example.com",
							Content: "tunnel.cfargotunnel.com",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantRecord: true,
			wantErr:    false,
		},
		{
			name: "未找到记录",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID      string `json:"id"`
					Type    string `json:"type"`
					Name    string `json:"name"`
					Content string `json:"content"`
				}]{
					Success: true,
					Result:  []struct {
						ID      string `json:"id"`
						Type    string `json:"type"`
						Name    string `json:"name"`
						Content string `json:"content"`
					}{},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantRecord: false,
			wantErr:    false,
		},
		{
			name: "API错误",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[[]struct {
					ID      string `json:"id"`
					Type    string `json:"type"`
					Name    string `json:"name"`
					Content string `json:"content"`
				}]{
					Success: false,
					Errors: []cfError{
						{Message: "zone not found"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "zone not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			record, err := client.FindDNSRecord(context.Background(), "zone-123", "test.example.com")

			if (err != nil) != tt.wantErr {
				t.Errorf("FindDNSRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
				return
			}
			if !tt.wantErr {
				if tt.wantRecord && record == nil {
					t.Error("FindDNSRecord() returned nil, want record")
				}
				if !tt.wantRecord && record != nil {
					t.Error("FindDNSRecord() returned record, want nil")
				}
			}
		})
	}
}

func TestClient_updateTunnelConfig(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name: "成功更新配置",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Method = %v, want PUT", r.Method)
				}
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				if body["config"] == nil {
					t.Error("config is nil")
				}
				resp := cfResponse[struct{}]{
					Success: true,
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "配置错误",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct{}]{
					Success: false,
					Errors: []cfError{
						{Message: "invalid config"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "invalid config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient("test-token")
			client.baseURL = server.URL

			err := client.updateTunnelConfig(context.Background(), "account-123", "tunnel-123", "test.example.com", "http://localhost:8000")

			if (err != nil) != tt.wantErr {
				t.Errorf("updateTunnelConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("error = %v, want to contain %v", err, tt.errContain)
			}
		})
	}
}

func TestClient_doRequest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	var result map[string]any
	err := client.doRequest(context.Background(), http.MethodGet, "/test", nil, &result)
	if err == nil {
		t.Error("doRequest() should return error for invalid JSON")
	}
}

func TestClient_doRequest_NilResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	// Pass nil for response, should not error
	err := client.doRequest(context.Background(), http.MethodGet, "/test", nil, nil)
	if err != nil {
		t.Errorf("doRequest() with nil response should not error, got %v", err)
	}
}

func TestClient_ListZones_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	_, err := client.ListZones(context.Background())
	if err == nil {
		t.Error("ListZones() should return error for HTTP error")
	}
}

func TestClient_CreateTunnel_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	_, err := client.CreateTunnel(context.Background(), "account-123", "test", "secret")
	if err == nil {
		t.Error("CreateTunnel() should return error for HTTP error")
	}
}

func TestClient_CreateDNSRecord_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	err := client.CreateDNSRecord(context.Background(), "zone-123", "test.example.com", "tunnel-123")
	if err == nil {
		t.Error("CreateDNSRecord() should return error for HTTP error")
	}
}

func TestClient_FindDNSRecord_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	_, err := client.FindDNSRecord(context.Background(), "zone-123", "test.example.com")
	if err == nil {
		t.Error("FindDNSRecord() should return error for HTTP error")
	}
}

func TestClient_updateTunnelConfig_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	err := client.updateTunnelConfig(context.Background(), "account-123", "tunnel-123", "test.example.com", "http://localhost:8000")
	if err == nil {
		t.Error("updateTunnelConfig() should return error for HTTP error")
	}
}

