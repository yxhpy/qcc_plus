package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestHandleMonitorShares(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{"POST method", http.MethodPost, http.StatusCreated},
		{"GET method", http.MethodGet, http.StatusOK},
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
				body = `{"expire_in":"1h"}`
			}

			req := httptest.NewRequest(tt.method, "/api/monitor/shares", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandleCreateMonitorShare(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		setupFunc      func(*Server) string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "create share with 1h expiry",
			body: `{"expire_in":"1h"}`,
			setupFunc: func(srv *Server) string {
				adminAcc := getAdminAccount(t, srv)
				return srv.sessionMgr.Create(adminAcc.ID, true).Token
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["id"]; !ok {
					t.Error("expected 'id' field")
				}
				if _, ok := resp["token"]; !ok {
					t.Error("expected 'token' field")
				}
				if _, ok := resp["share_url"]; !ok {
					t.Error("expected 'share_url' field")
				}
				if _, ok := resp["expire_at"]; !ok {
					t.Error("expected 'expire_at' field")
				}
			},
		},
		{
			name: "create permanent share",
			body: `{"expire_in":"permanent"}`,
			setupFunc: func(srv *Server) string {
				adminAcc := getAdminAccount(t, srv)
				return srv.sessionMgr.Create(adminAcc.ID, true).Token
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["expire_at"] != nil {
					t.Error("expected null expire_at for permanent share")
				}
			},
		},
		{
			name: "create share with 24h expiry",
			body: `{"expire_in":"24h"}`,
			setupFunc: func(srv *Server) string {
				adminAcc := getAdminAccount(t, srv)
				return srv.sessionMgr.Create(adminAcc.ID, true).Token
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create share with 168h expiry",
			body: `{"expire_in":"168h"}`,
			setupFunc: func(srv *Server) string {
				adminAcc := getAdminAccount(t, srv)
				return srv.sessionMgr.Create(adminAcc.ID, true).Token
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
			name:           "invalid expire_in",
			body:           `{"expire_in":"invalid"}`,
			setupFunc:      func(srv *Server) string { return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty expire_in",
			body:           `{"expire_in":""}`,
			setupFunc:      func(srv *Server) string { return srv.sessionMgr.Create(getAdminAccount(t, srv).ID, true).Token },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized without session",
			body:           `{"expire_in":"1h"}`,
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			token := tt.setupFunc(srv)

			req := httptest.NewRequest(http.MethodPost, "/api/monitor/shares", strings.NewReader(tt.body))
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

func TestHandleCreateMonitorShareAdminForOtherAccount(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a non-admin account
	nonAdminAcc, err := srv.createAccount("user1", "user1-key", "pass", false)
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	t.Run("admin can create share for other account", func(t *testing.T) {
		body := `{"account_id":"` + nonAdminAcc.ID + `","expire_in":"1h"}`
		req := httptest.NewRequest(http.MethodPost, "/api/monitor/shares", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-admin cannot create share for other account", func(t *testing.T) {
		userSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)
		body := `{"account_id":"` + adminAcc.ID + `","expire_in":"1h"}`
		req := httptest.NewRequest(http.MethodPost, "/api/monitor/shares", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: userSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("admin creating share for non-existent account", func(t *testing.T) {
		body := `{"account_id":"non-existent","expire_in":"1h"}`
		req := httptest.NewRequest(http.MethodPost, "/api/monitor/shares", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestHandleListMonitorShares(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a share first
	createBody := `{"expire_in":"1h"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/monitor/shares", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
	createW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createW, createReq)

	tests := []struct {
		name           string
		queryParams    string
		setupFunc      func(*Server) string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:        "list shares for admin",
			queryParams: "",
			setupFunc: func(srv *Server) string {
				return adminSess.Token
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				shares, ok := resp["shares"].([]interface{})
				if !ok {
					t.Error("expected 'shares' array")
				}
				if len(shares) == 0 {
					t.Error("expected at least one share")
				}
			},
		},
		{
			name:           "list shares with limit",
			queryParams:    "?limit=5",
			setupFunc:      func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:           "list shares with offset",
			queryParams:    "?offset=0",
			setupFunc:      func(srv *Server) string { return adminSess.Token },
			expectedStatus: http.StatusOK,
		},
		{
			name:           "list shares with include_revoked",
			queryParams:    "?include_revoked=true",
			setupFunc:      func(srv *Server) string { return adminSess.Token },
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

			req := httptest.NewRequest(http.MethodGet, "/api/monitor/shares"+tt.queryParams, nil)
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

func TestHandleRevokeMonitorShare(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a share to revoke
	now := time.Now().UTC()
	shareRec := store.MonitorShareRecord{
		ID:        "share-test-123",
		AccountID: adminAcc.ID,
		Token:     "test-token-123",
		CreatedBy: adminAcc.Name,
		CreatedAt: now,
		ExpireAt:  now.Add(time.Hour),
	}
	if err := srv.store.CreateMonitorShare(context.Background(), shareRec); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	tests := []struct {
		name           string
		shareID        string
		method         string
		setupFunc      func(*Server) string
		expectedStatus int
	}{
		{
			name:    "revoke existing share",
			shareID: "share-test-123",
			method:  http.MethodDelete,
			setupFunc: func(srv *Server) string {
				return adminSess.Token
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:    "revoke non-existent share",
			shareID: "non-existent",
			method:  http.MethodDelete,
			setupFunc: func(srv *Server) string {
				return adminSess.Token
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized without session",
			shareID:        "share-test-123",
			method:         http.MethodDelete,
			setupFunc:      func(srv *Server) string { return "" },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:    "method not allowed",
			shareID: "share-test-123",
			method:  http.MethodGet,
			setupFunc: func(srv *Server) string {
				return adminSess.Token
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setupFunc(srv)

			req := httptest.NewRequest(tt.method, "/api/monitor/shares/"+tt.shareID, nil)
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

func TestHandleAccessMonitorShare(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)

	// Create a valid share
	now := time.Now().UTC()
	validShare := store.MonitorShareRecord{
		ID:        "share-valid",
		AccountID: adminAcc.ID,
		Token:     "valid-token",
		CreatedBy: adminAcc.Name,
		CreatedAt: now,
		ExpireAt:  now.Add(time.Hour),
	}
	if err := srv.store.CreateMonitorShare(context.Background(), validShare); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	tests := []struct {
		name           string
		token          string
		method         string
		expectedStatus int
	}{
		{
			name:           "access valid share",
			token:          "valid-token",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "access non-existent share",
			token:          "non-existent",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "empty token",
			token:          "",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "method not allowed",
			token:          "valid-token",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/monitor/share/"+tt.token, nil)
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestParseShareExpire(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		checkResult func(*testing.T, time.Time)
	}{
		{
			name:        "1h expiry",
			input:       "1h",
			expectError: false,
			checkResult: func(t *testing.T, result time.Time) {
				if result.IsZero() {
					t.Error("expected non-zero time")
				}
			},
		},
		{
			name:        "24h expiry",
			input:       "24h",
			expectError: false,
		},
		{
			name:        "168h expiry",
			input:       "168h",
			expectError: false,
		},
		{
			name:        "permanent",
			input:       "permanent",
			expectError: false,
			checkResult: func(t *testing.T, result time.Time) {
				if !result.IsZero() {
					t.Error("expected zero time for permanent")
				}
			},
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid value",
			input:       "invalid",
			expectError: true,
		},
		{
			name:        "case insensitive",
			input:       "PERMANENT",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseShareExpire(tt.input)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.checkResult != nil && err == nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestHandleMonitorSharesWithoutStore(t *testing.T) {
	// Build a minimal server without using buildServerNoWarmup to avoid background goroutines
	srv := &Server{
		accounts:    make(map[string]*Account),
		accountByID: make(map[string]*Account),
		nodeIndex:   make(map[string]*Node),
		nodeAccount: make(map[string]*Account),
		nodes:       make(map[string]*Node),
		sessionMgr:  NewSessionManager(24 * time.Hour),
		store:       nil, // explicitly no store
	}
	acc := &Account{ID: "admin-1", Name: "admin", IsAdmin: true, Nodes: make(map[string]*Node), FailedSet: make(map[string]struct{})}
	srv.accounts["admin-key"] = acc
	srv.accountByID["admin-1"] = acc
	srv.defaultAccount = acc

	req := httptest.NewRequest(http.MethodPost, "/api/monitor/shares", strings.NewReader(`{"expire_in":"1h"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), accountContextKey{}, acc)
	ctx = context.WithValue(ctx, isAdminContextKey{}, true)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	srv.handleMonitorShares(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// Helper function to get admin account
func getAdminAccount(t *testing.T, srv *Server) *Account {
	t.Helper()
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for _, acc := range srv.accountByID {
		if acc.IsAdmin {
			return acc
		}
	}
	t.Fatal("admin account not found")
	return nil
}
