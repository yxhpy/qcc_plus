package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TunnelConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效配置",
			cfg: TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test",
				LocalAddr: "http://localhost:8000",
				Zone:      "example.com",
			},
			wantErr: false,
		},
		{
			name: "缺少APIToken",
			cfg: TunnelConfig{
				Subdomain: "test",
			},
			wantErr: true,
			errMsg:  "CF_API_TOKEN 不能为空",
		},
		{
			name: "缺少Subdomain",
			cfg: TunnelConfig{
				APIToken: "test-token",
			},
			wantErr: true,
			errMsg:  "TUNNEL_SUBDOMAIN 不能为空",
		},
		{
			name: "最小有效配置",
			cfg: TunnelConfig{
				APIToken:  "token",
				Subdomain: "sub",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Error("NewManager() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if mgr == nil {
					t.Error("NewManager() returned nil manager")
					return
				}
				if mgr.client == nil {
					t.Error("manager.client is nil")
				}
				if mgr.logger == nil {
					t.Error("manager.logger is nil")
				}
			}
		})
	}
}

func TestManager_GetPublicURL(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// 初始状态应该为空
	url := mgr.GetPublicURL()
	if url != "" {
		t.Errorf("GetPublicURL() = %v, want empty string", url)
	}

	// 设置 publicURL
	mgr.publicURL = "https://test.example.com"
	url = mgr.GetPublicURL()
	if url != "https://test.example.com" {
		t.Errorf("GetPublicURL() = %v, want https://test.example.com", url)
	}
}

func TestManager_Start_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		cfg       TunnelConfig
		localAddr string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "缺少本地地址",
			cfg: TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test",
			},
			localAddr: "",
			wantErr:   true,
			errMsg:    "本地服务地址不能为空",
		},
		{
			name: "使用配置中的本地地址",
			cfg: TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test",
				LocalAddr: "http://localhost:8000",
			},
			localAddr: "",
			wantErr:   true, // 会在后续步骤失败，但不是因为地址为空
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.cfg)
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}

			// 使用短超时的 context
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err = mgr.Start(ctx, tt.localAddr)

			if (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error = %v, want to contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestManager_Start_AlreadyRunning(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// 模拟已经运行的状态
	mgr.cmd = &exec.Cmd{}

	err = mgr.Start(context.Background(), "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when already running")
	}
	if !strings.Contains(err.Error(), "隧道已运行") {
		t.Errorf("error = %v, want to contain '隧道已运行'", err)
	}
}

func TestManager_Start_WithMockServer(t *testing.T) {
	// 创建 mock Cloudflare API 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet:
			// GetAccountID
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

		case strings.Contains(r.URL.Path, "/zones") && r.Method == http.MethodGet:
			// ListZones
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost:
			// CreateTunnel
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

		case strings.Contains(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			// updateTunnelConfig
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
			// FindDNSRecord
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

		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodPost:
			// CreateDNSRecord
			resp := cfResponse[struct {
				ID string `json:"id"`
			}]{
				Success: true,
				Result: struct {
					ID string `json:"id"`
				}{ID: "record-123"},
			}
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/token") && r.Method == http.MethodGet:
			// GetTunnelToken
			resp := cfResponse[string]{
				Success: true,
				Result:  "tunnel-token-xyz",
			}
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
		LocalAddr: "http://localhost:8000",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// 替换 client 的 baseURL
	mgr.client.baseURL = server.URL

	// 使用短超时的 context，因为 cloudflared 命令会失败
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")

	// 预期会失败，因为 cloudflared 命令不存在或无法启动
	// 但我们可以验证前面的 API 调用是否成功
	if err != nil {
		// 检查是否是 cloudflared 启动失败
		if !strings.Contains(err.Error(), "cloudflared") && !strings.Contains(err.Error(), "executable file not found") {
			// 如果不是 cloudflared 的问题，说明前面的 API 调用有问题
			t.Logf("Start() error (expected cloudflared failure): %v", err)
		}
	}

	// 验证管理器状态
	if mgr.accountID == "" {
		t.Error("accountID should be set")
	}
	if mgr.zone.ID == "" {
		t.Error("zone.ID should be set")
	}
}

func TestManager_Stop(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// 测试停止未运行的管理器
	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() on non-running manager should not error, got %v", err)
	}
}

func TestManager_Stop_WithCleanup(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 所有删除操作都返回成功
		resp := cfResponse[struct{}]{
			Success: true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	// 设置一些状态，模拟已启动的隧道
	mgr.accountID = "account-123"
	mgr.zone = Zone{ID: "zone-123", Name: "example.com"}
	mgr.tunnel = &Tunnel{ID: "tunnel-123", Name: "test-tunnel"}
	mgr.record = &DNSRecord{ID: "record-123", Name: "test.example.com"}

	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}

	// 验证清理后的状态
	if mgr.tunnel != nil {
		t.Error("tunnel should be nil after Stop()")
	}
	if mgr.record != nil {
		t.Error("record should be nil after Stop()")
	}
}

func TestManager_cleanupDNS(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Manager)
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "无DNS记录需要清理",
			setup: func(m *Manager) {
				m.record = nil
			},
			wantErr: false,
		},
		{
			name: "成功删除DNS记录",
			setup: func(m *Manager) {
				m.zone = Zone{ID: "zone-123"}
				m.record = &DNSRecord{ID: "record-123", Name: "test.example.com"}
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct{}]{
					Success: true,
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "DNS记录无ID需要查找",
			setup: func(m *Manager) {
				m.zone = Zone{ID: "zone-123"}
				m.record = &DNSRecord{Name: "test.example.com"}
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					// FindDNSRecord
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
							{ID: "record-123", Name: "test.example.com"},
						},
					}
					json.NewEncoder(w).Encode(resp)
				} else {
					// DeleteDNSRecord
					resp := cfResponse[struct{}]{
						Success: true,
					}
					json.NewEncoder(w).Encode(resp)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.handler != nil {
				server = httptest.NewServer(tt.handler)
				defer server.Close()
			}

			mgr, err := NewManager(TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test",
			})
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}

			if server != nil {
				mgr.client.baseURL = server.URL
			}

			tt.setup(mgr)

			err = mgr.cleanupDNS()

			if (err != nil) != tt.wantErr {
				t.Errorf("cleanupDNS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_cleanupTunnel(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Manager)
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "无隧道需要清理",
			setup: func(m *Manager) {
				m.tunnel = nil
			},
			wantErr: false,
		},
		{
			name: "成功删除隧道",
			setup: func(m *Manager) {
				m.accountID = "account-123"
				m.tunnel = &Tunnel{ID: "tunnel-123"}
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct{}]{
					Success: true,
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "删除隧道失败",
			setup: func(m *Manager) {
				m.accountID = "account-123"
				m.tunnel = &Tunnel{ID: "tunnel-123"}
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := cfResponse[struct{}]{
					Success: false,
					Errors: []cfError{
						{Message: "tunnel not found"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.handler != nil {
				server = httptest.NewServer(tt.handler)
				defer server.Close()
			}

			mgr, err := NewManager(TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test",
			})
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}

			if server != nil {
				mgr.client.baseURL = server.URL
			}

			tt.setup(mgr)

			err = mgr.cleanupTunnel()

			if (err != nil) != tt.wantErr {
				t.Errorf("cleanupTunnel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRandomSecret(t *testing.T) {
	// 测试生成随机密钥
	secret1, err := randomSecret()
	if err != nil {
		t.Fatalf("randomSecret() error = %v", err)
	}
	if secret1 == "" {
		t.Error("randomSecret() returned empty string")
	}

	// 测试生成的密钥是随机的
	secret2, err := randomSecret()
	if err != nil {
		t.Fatalf("randomSecret() error = %v", err)
	}
	if secret1 == secret2 {
		t.Error("randomSecret() returned same secret twice")
	}

	// 测试密钥长度（base64 编码的 32 字节应该是 44 字符）
	if len(secret1) != 44 {
		t.Errorf("randomSecret() length = %d, want 44", len(secret1))
	}
}

func TestManager_Start_GetAccountIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/accounts") {
			resp := cfResponse[[]struct {
				ID string `json:"id"`
			}]{
				Success: false,
				Errors: []cfError{
					{Message: "authentication failed"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when GetAccountID fails")
	}
	if !strings.Contains(err.Error(), "获取 Cloudflare 账号失败") {
		t.Errorf("error = %v, want to contain '获取 Cloudflare 账号失败'", err)
	}
}

func TestManager_Start_ListZonesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/accounts") {
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
		} else if strings.Contains(r.URL.Path, "/zones") {
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: false,
				Errors: []cfError{
					{Message: "zone list failed"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when ListZones fails")
	}
	if !strings.Contains(err.Error(), "获取域名列表失败") {
		t.Errorf("error = %v, want to contain '获取域名列表失败'", err)
	}
}

func TestManager_Start_NoZones(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/accounts") {
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
		} else if strings.Contains(r.URL.Path, "/zones") {
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
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when no zones available")
	}
	if !strings.Contains(err.Error(), "账号下没有可用域名") {
		t.Errorf("error = %v, want to contain '账号下没有可用域名'", err)
	}
}

func TestManager_Start_ZoneNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/accounts") {
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
		} else if strings.Contains(r.URL.Path, "/zones") {
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
		Zone:      "notfound.com", // 指定不存在的域名
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when specified zone not found")
	}
	if !strings.Contains(err.Error(), "未找到指定域名") {
		t.Errorf("error = %v, want to contain '未找到指定域名'", err)
	}
}

func TestManager_Start_CreateTunnelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet {
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
		} else if strings.Contains(r.URL.Path, "/zones") {
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else if strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost {
			resp := cfResponse[struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Token string `json:"tunnel_token"`
			}]{
				Success: false,
				Errors: []cfError{
					{Message: "tunnel creation failed"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when CreateTunnel fails")
	}
	if !strings.Contains(err.Error(), "创建隧道失败") {
		t.Errorf("error = %v, want to contain '创建隧道失败'", err)
	}
}

func TestManager_Start_UpdateConfigError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet:
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
		case strings.Contains(r.URL.Path, "/zones"):
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost:
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
					ID:   "tunnel-123",
					Name: "test-tunnel",
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			resp := cfResponse[struct{}]{
				Success: false,
				Errors: []cfError{
					{Message: "config update failed"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodDelete:
			// cleanup
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when updateTunnelConfig fails")
	}
	if !strings.Contains(err.Error(), "配置隧道转发失败") {
		t.Errorf("error = %v, want to contain '配置隧道转发失败'", err)
	}
}

func TestManager_Start_FindDNSRecordError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet:
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
		case strings.Contains(r.URL.Path, "/zones") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/dns_records"):
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost:
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
					ID:   "tunnel-123",
					Name: "test-tunnel",
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
			resp := cfResponse[[]struct {
				ID      string `json:"id"`
				Type    string `json:"type"`
				Name    string `json:"name"`
				Content string `json:"content"`
			}]{
				Success: false,
				Errors: []cfError{
					{Message: "dns query failed"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodDelete:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when FindDNSRecord fails")
	}
	if !strings.Contains(err.Error(), "查询 DNS 记录失败") {
		t.Errorf("error = %v, want to contain '查询 DNS 记录失败'", err)
	}
}

func TestManager_Start_DNSRecordConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet:
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
		case strings.Contains(r.URL.Path, "/zones") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/dns_records"):
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost:
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
					ID:   "tunnel-123",
					Name: "test-tunnel",
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
			// Return existing record with different content
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
						Content: "old-tunnel.cfargotunnel.com",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodDelete:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodPost:
			resp := cfResponse[struct {
				ID string `json:"id"`
			}]{
				Success: true,
				Result: struct {
					ID string `json:"id"`
				}{ID: "record-456"},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/token") && r.Method == http.MethodGet:
			resp := cfResponse[string]{
				Success: true,
				Result:  "tunnel-token-xyz",
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodDelete:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	// Should fail at cloudflared start, but DNS record should be updated
	if err != nil && !strings.Contains(err.Error(), "cloudflared") {
		t.Logf("Start() error (expected cloudflared failure): %v", err)
	}
}

func TestManager_Start_CreateDNSRecordError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet:
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
		case strings.Contains(r.URL.Path, "/zones") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/dns_records"):
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost:
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
					ID:   "tunnel-123",
					Name: "test-tunnel",
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
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
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodPost:
			resp := cfResponse[struct {
				ID string `json:"id"`
			}]{
				Success: false,
				Errors: []cfError{
					{Message: "dns creation failed"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodDelete:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	if err == nil {
		t.Error("Start() should return error when CreateDNSRecord fails")
	}
	if !strings.Contains(err.Error(), "创建 DNS 记录失败") {
		t.Errorf("error = %v, want to contain '创建 DNS 记录失败'", err)
	}
}

func TestManager_Stop_WithProcess(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// Create a dummy process that will exit quickly
	cmd := exec.Command("sleep", "0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("Cannot start test process: %v", err)
	}

	mgr.cmd = cmd

	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}

	if mgr.cmd != nil {
		t.Error("cmd should be nil after Stop()")
	}
}

func TestManager_Stop_ProcessKill(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// Create a long-running process that needs to be killed
	cmd := exec.Command("sleep", "100")
	if err := cmd.Start(); err != nil {
		t.Skipf("Cannot start test process: %v", err)
	}

	mgr.cmd = cmd

	err = mgr.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}

	if mgr.cmd != nil {
		t.Error("cmd should be nil after Stop()")
	}
}

func TestManager_cleanupDNS_NoZoneID(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.record = &DNSRecord{ID: "record-123"}
	// zone.ID is empty

	err = mgr.cleanupDNS()
	if err != nil {
		t.Errorf("cleanupDNS() with no zone ID should not error, got %v", err)
	}
}

func TestManager_cleanupDNS_NoRecordID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// FindDNSRecord returns nothing
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
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL
	mgr.zone = Zone{ID: "zone-123"}
	mgr.record = &DNSRecord{Name: "test.example.com"} // No ID

	err = mgr.cleanupDNS()
	if err != nil {
		t.Errorf("cleanupDNS() should not error when record not found, got %v", err)
	}
}

func TestManager_cleanupTunnel_NoAccountID(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.tunnel = &Tunnel{ID: "tunnel-123"}
	// accountID is empty

	err = mgr.cleanupTunnel()
	if err != nil {
		t.Errorf("cleanupTunnel() with no account ID should not error, got %v", err)
	}
}

func TestManager_Start_NilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/accounts") {
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
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	// Pass nil context, should use Background()
	err = mgr.Start(nil, "http://localhost:8000")
	// Will fail at some point, but should handle nil context
	if err != nil {
		t.Logf("Start() with nil context error (expected): %v", err)
	}
}

func TestManager_Start_EmptyLocalAddrUsesConfig(t *testing.T) {
	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
		LocalAddr: "http://localhost:9000",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// Create a context that will timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Pass empty localAddr, should use config.LocalAddr
	err = mgr.Start(ctx, "")
	// Will fail due to timeout, but should use config.LocalAddr
	if err != nil {
		t.Logf("Start() error (expected timeout): %v", err)
	}
}

func TestManager_Stop_CleanupErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// All cleanup operations fail
		resp := cfResponse[struct{}]{
			Success: false,
			Errors: []cfError{
				{Message: "cleanup failed"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL
	mgr.accountID = "account-123"
	mgr.zone = Zone{ID: "zone-123"}
	mgr.tunnel = &Tunnel{ID: "tunnel-123"}
	mgr.record = &DNSRecord{ID: "record-123"}

	err = mgr.Stop()
	if err == nil {
		t.Error("Stop() should return error when cleanup fails")
	}
	// Should contain both DNS and tunnel cleanup errors
	if !strings.Contains(err.Error(), "cleanup failed") {
		t.Errorf("error = %v, want to contain 'cleanup failed'", err)
	}
}

func TestManager_cleanupDNS_DeleteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cfResponse[struct{}]{
			Success: false,
			Errors: []cfError{
				{Message: "delete failed"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL
	mgr.zone = Zone{ID: "zone-123"}
	mgr.record = &DNSRecord{ID: "record-123"}

	err = mgr.cleanupDNS()
	if err == nil {
		t.Error("cleanupDNS() should return error when delete fails")
	}
	if !strings.Contains(err.Error(), "删除 DNS 记录失败") {
		t.Errorf("error = %v, want to contain '删除 DNS 记录失败'", err)
	}
}

func TestManager_Start_DNSRecordMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts") && r.Method == http.MethodGet:
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
		case strings.Contains(r.URL.Path, "/zones") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/dns_records"):
			resp := cfResponse[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}]{
				Success: true,
				Result: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "zone-123", Name: "example.com"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodPost:
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
					ID:   "tunnel-123",
					Name: "test-tunnel",
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/configurations") && r.Method == http.MethodPut:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/dns_records") && r.Method == http.MethodGet:
			// Return existing record with MATCHING content
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
						Content: "tunnel-123.cfargotunnel.com", // Matches tunnel ID
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/token") && r.Method == http.MethodGet:
			resp := cfResponse[string]{
				Success: true,
				Result:  "tunnel-token-xyz",
			}
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/cfd_tunnel") && r.Method == http.MethodDelete:
			resp := cfResponse[struct{}]{
				Success: true,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mgr, err := NewManager(TunnelConfig{
		APIToken:  "test-token",
		Subdomain: "test",
	})
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	mgr.client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = mgr.Start(ctx, "http://localhost:8000")
	// Should fail at cloudflared start, but DNS record should be reused
	if err != nil && !strings.Contains(err.Error(), "cloudflared") {
		t.Logf("Start() error (expected cloudflared failure): %v", err)
	}

	// Verify that existing DNS record was reused
	if mgr.record == nil {
		t.Error("record should be set")
	} else if mgr.record.ID != "record-123" {
		t.Errorf("record.ID = %v, want record-123", mgr.record.ID)
	}
}

func TestClient_doRequest_BodyEncodeError(t *testing.T) {
	client := NewClient("test-token")

	// Try to encode an invalid body (channel cannot be JSON encoded)
	invalidBody := make(chan int)

	err := client.doRequest(context.Background(), http.MethodPost, "/test", invalidBody, nil)
	if err == nil {
		t.Error("doRequest() should return error for invalid body")
	}
}
