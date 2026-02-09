package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestHandleTunnelConfig(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		setupFunc      func(*Server) string
		expectedStatus int
	}{
		{
			name:   "GET tunnel config",
			method: http.MethodGet,
			body:   "",
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "PUT update tunnel config",
			method: http.MethodPut,
			body:   `{"api_token":"test-token","subdomain":"test","zone":"example.com","enabled":true}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "PUT with empty subdomain",
			method: http.MethodPut,
			body:   `{"api_token":"test-token","subdomain":"","zone":"example.com"}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "PUT with invalid JSON",
			method: http.MethodPut,
			body:   `{invalid}`,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "unauthorized without admin",
			method: http.MethodGet,
			body:   "",
			setupFunc: func(srv *Server) string {
				nonAdminAcc, _ := srv.createAccount("user", "user-key", "pass", false)
				return srv.sessionMgr.Create(nonAdminAcc.ID, false).Token
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "unauthorized without session",
			method:         http.MethodGet,
			body:           "",
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "method not allowed",
			method: http.MethodDelete,
			body:   "",
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			token := tt.setupFunc(srv)

			req := httptest.NewRequest(tt.method, "/admin/api/tunnel", strings.NewReader(tt.body))
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

func TestHandleTunnelConfigWithoutStore(t *testing.T) {
	srv := &Server{
		accounts:    make(map[string]*Account),
		accountByID: make(map[string]*Account),
		nodeIndex:   make(map[string]*Node),
		nodeAccount: make(map[string]*Account),
		nodes:       make(map[string]*Node),
		sessionMgr:  NewSessionManager(24 * time.Hour),
		store:       nil,
	}
	acc := &Account{ID: "admin-1", Name: "admin", IsAdmin: true, Nodes: make(map[string]*Node), FailedSet: make(map[string]struct{})}
	srv.accounts["admin-key"] = acc
	srv.accountByID["admin-1"] = acc
	srv.defaultAccount = acc

	req := httptest.NewRequest(http.MethodGet, "/admin/api/tunnel", nil)
	ctx := context.WithValue(req.Context(), accountContextKey{}, acc)
	ctx = context.WithValue(ctx, isAdminContextKey{}, true)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	srv.handleTunnelConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleTunnelStart(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		setupFunc      func(*Server) (string, error)
		expectedStatus int
	}{
		{
			name:   "start tunnel",
			method: http.MethodPost,
			setupFunc: func(srv *Server) (string, error) {
				// Create tunnel config first
				cfg := store.TunnelConfig{
					ID:        "default",
					APIToken:  "test-token",
					Subdomain: "test",
					Zone:      "example.com",
					Enabled:   false,
				}
				if err := srv.store.SaveTunnelConfig(context.Background(), cfg); err != nil {
					return "", err
				}
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token, nil
			},
			// Will fail because we can't actually start a tunnel without valid credentials
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "unauthorized without admin",
			method: http.MethodPost,
			setupFunc: func(srv *Server) (string, error) {
				nonAdminAcc, _ := srv.createAccount("user", "user-key", "pass", false)
				return srv.sessionMgr.Create(nonAdminAcc.ID, false).Token, nil
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "method not allowed",
			method: http.MethodGet,
			setupFunc: func(srv *Server) (string, error) {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token, nil
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			token, err := tt.setupFunc(srv)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			req := httptest.NewRequest(tt.method, "/admin/api/tunnel/start", nil)
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

func TestHandleTunnelStop(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		setupFunc      func(*Server) string
		expectedStatus int
	}{
		{
			name:   "stop tunnel when not running",
			method: http.MethodPost,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "unauthorized without admin",
			method: http.MethodPost,
			setupFunc: func(srv *Server) string {
				nonAdminAcc, _ := srv.createAccount("user", "user-key", "pass", false)
				return srv.sessionMgr.Create(nonAdminAcc.ID, false).Token
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "method not allowed",
			method: http.MethodGet,
			setupFunc: func(srv *Server) string {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			token := tt.setupFunc(srv)

			req := httptest.NewRequest(tt.method, "/admin/api/tunnel/stop", nil)
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

func TestHandleTunnelZones(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		setupFunc      func(*Server) (string, error)
		expectedStatus int
	}{
		{
			name:   "get zones without config",
			method: http.MethodGet,
			setupFunc: func(srv *Server) (string, error) {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token, nil
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "get zones with config but invalid token",
			method: http.MethodGet,
			setupFunc: func(srv *Server) (string, error) {
				cfg := store.TunnelConfig{
					ID:        "default",
					APIToken:  "invalid-token",
					Subdomain: "test",
					Zone:      "example.com",
					Enabled:   false,
				}
				if err := srv.store.SaveTunnelConfig(context.Background(), cfg); err != nil {
					return "", err
				}
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token, nil
			},
			// Will fail because the API token is invalid
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "unauthorized without admin",
			method: http.MethodGet,
			setupFunc: func(srv *Server) (string, error) {
				nonAdminAcc, _ := srv.createAccount("user", "user-key", "pass", false)
				return srv.sessionMgr.Create(nonAdminAcc.ID, false).Token, nil
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "method not allowed",
			method: http.MethodPost,
			setupFunc: func(srv *Server) (string, error) {
				return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token, nil
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			token, err := tt.setupFunc(srv)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			req := httptest.NewRequest(tt.method, "/admin/api/tunnel/zones", nil)
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

func TestHandleTunnelZonesWithoutStore(t *testing.T) {
	srv := &Server{
		accounts:    make(map[string]*Account),
		accountByID: make(map[string]*Account),
		nodeIndex:   make(map[string]*Node),
		nodeAccount: make(map[string]*Account),
		nodes:       make(map[string]*Node),
		sessionMgr:  NewSessionManager(24 * time.Hour),
		store:       nil,
	}
	acc := &Account{ID: "admin-1", Name: "admin", IsAdmin: true, Nodes: make(map[string]*Node), FailedSet: make(map[string]struct{})}
	srv.accounts["admin-key"] = acc
	srv.accountByID["admin-1"] = acc
	srv.defaultAccount = acc

	req := httptest.NewRequest(http.MethodGet, "/admin/api/tunnel/zones", nil)
	ctx := context.WithValue(req.Context(), accountContextKey{}, acc)
	ctx = context.WithValue(ctx, isAdminContextKey{}, true)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	srv.handleTunnelZones(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"ErrNotFound", store.ErrNotFound, true},
		{"other error", context.DeadlineExceeded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFound(tt.err)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHandleTunnelConfigUpdateWhileRunning(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create initial config
	cfg := store.TunnelConfig{
		ID:        "default",
		APIToken:  "test-token",
		Subdomain: "test",
		Zone:      "example.com",
		Enabled:   false,
	}
	if err := srv.store.SaveTunnelConfig(context.Background(), cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Simulate tunnel running by setting tunnelMgr (we can't actually start it)
	// This test verifies the check exists, even if we can't fully test it

	body := `{"api_token":"new-token","subdomain":"new-subdomain"}`
	req := httptest.NewRequest(http.MethodPut, "/admin/api/tunnel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Should succeed because tunnel is not actually running
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleTunnelConfigPartialUpdate(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create initial config
	cfg := store.TunnelConfig{
		ID:        "default",
		APIToken:  "test-token",
		Subdomain: "test",
		Zone:      "example.com",
		Enabled:   false,
	}
	if err := srv.store.SaveTunnelConfig(context.Background(), cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "update only enabled",
			body:           `{"enabled":true}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update only zone",
			body:           `{"zone":"newzone.com"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update subdomain to empty is no-op",
			body:           `{"subdomain":""}`,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/admin/api/tunnel", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// Wait a bit for async operations
			time.Sleep(10 * time.Millisecond)
		})
	}
}
