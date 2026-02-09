package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"qcc_plus/internal/store"
)

func TestHandleMonitorWebSocket(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*Server) string
		queryParams    string
		expectedStatus int
	}{
		{
			name: "connect with session cookie",
			setupFunc: func(srv *Server) string {
				adminAcc := getAdminAccount(t, srv)
				sess := srv.sessionMgr.Create(adminAcc.ID, true)
				return sess.Token
			},
			queryParams:    "",
			expectedStatus: http.StatusSwitchingProtocols,
		},
		{
			name: "connect with share token",
			setupFunc: func(srv *Server) string {
				adminAcc := getAdminAccount(t, srv)
				// Create a share
				now := time.Now().UTC()
				shareRec := store.MonitorShareRecord{
					ID:        "share-ws-test",
					AccountID: adminAcc.ID,
					Token:     "ws-test-token",
					CreatedBy: adminAcc.Name,
					CreatedAt: now,
					ExpireAt:  now.Add(time.Hour),
				}
				if err := srv.store.CreateMonitorShare(context.Background(), shareRec); err != nil {
					t.Fatalf("failed to create share: %v", err)
				}
				return ""
			},
			queryParams:    "?token=ws-test-token",
			expectedStatus: http.StatusSwitchingProtocols,
		},
		{
			name:           "unauthorized without credentials",
			setupFunc:      func(srv *Server) string { return "" },
			queryParams:    "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "unauthorized with invalid share token",
			setupFunc:      func(srv *Server) string { return "" },
			queryParams:    "?token=invalid-token",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithUpstream("http://example.com")
			srv := buildServerNoWarmup(t, b)

			sessionToken := tt.setupFunc(srv)

			// Create test server
			testServer := httptest.NewServer(srv.Handler())
			defer testServer.Close()

			// Convert http:// to ws://
			wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/monitor/ws" + tt.queryParams

			// Create WebSocket connection
			header := http.Header{}
			if sessionToken != "" {
				header.Set("Cookie", "session_token="+sessionToken)
			}

			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)

			if tt.expectedStatus == http.StatusSwitchingProtocols {
				if err != nil {
					t.Fatalf("failed to connect: %v", err)
				}
				defer conn.Close()

				if resp.StatusCode != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}
			} else {
				if err == nil {
					conn.Close()
					t.Error("expected connection to fail but it succeeded")
				}
				if resp != nil && resp.StatusCode != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}
			}
		})
	}
}

func TestAuthenticateWSRequest(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a share
	now := time.Now().UTC()
	shareRec := store.MonitorShareRecord{
		ID:        "share-auth-test",
		AccountID: adminAcc.ID,
		Token:     "auth-test-token",
		CreatedBy: adminAcc.Name,
		CreatedAt: now,
		ExpireAt:  now.Add(time.Hour),
	}
	if err := srv.store.CreateMonitorShare(context.Background(), shareRec); err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	tests := []struct {
		name        string
		setupReq    func() *http.Request
		expectError bool
		expectedID  string
	}{
		{
			name: "authenticate with session cookie",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/monitor/ws", nil)
				req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
				return req
			},
			expectError: false,
			expectedID:  adminAcc.ID,
		},
		{
			name: "authenticate with share token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/monitor/ws?token=auth-test-token", nil)
				return req
			},
			expectError: false,
			expectedID:  adminAcc.ID,
		},
		{
			name: "fail without credentials",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/monitor/ws", nil)
			},
			expectError: true,
		},
		{
			name: "fail with invalid share token",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/monitor/ws?token=invalid", nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			accountID, err := srv.authenticateWSRequest(req)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && accountID != tt.expectedID {
				t.Errorf("expected account ID %s, got %s", tt.expectedID, accountID)
			}
		})
	}
}

func TestGetSessionFromCookie(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	validSess := srv.sessionMgr.Create(adminAcc.ID, true)

	tests := []struct {
		name        string
		setupReq    func() *http.Request
		expectNil   bool
		expectedID  string
	}{
		{
			name: "get valid session",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{Name: "session_token", Value: validSess.Token})
				return req
			},
			expectNil:  false,
			expectedID: adminAcc.ID,
		},
		{
			name: "no cookie",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectNil: true,
		},
		{
			name: "invalid session token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{Name: "session_token", Value: "invalid"})
				return req
			},
			expectNil: true,
		},
		{
			name: "empty session token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{Name: "session_token", Value: ""})
				return req
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			sess := getSessionFromCookie(srv.sessionMgr, req)

			if tt.expectNil && sess != nil {
				t.Error("expected nil session but got one")
			}
			if !tt.expectNil && sess == nil {
				t.Error("expected session but got nil")
			}
			if !tt.expectNil && sess != nil && sess.AccountID != tt.expectedID {
				t.Errorf("expected account ID %s, got %s", tt.expectedID, sess.AccountID)
			}
		})
	}
}

func TestGetSessionFromCookieNilManager(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := getSessionFromCookie(nil, req)

	if sess != nil {
		t.Error("expected nil session with nil manager")
	}
}

func TestGetSessionFromCookieNilRequest(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	sess := getSessionFromCookie(srv.sessionMgr, nil)

	if sess != nil {
		t.Error("expected nil session with nil request")
	}
}

func TestHandleMonitorWebSocketWithoutHub(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Disable WebSocket hub
	srv.wsHub = nil

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	req := httptest.NewRequest(http.MethodGet, "/api/monitor/ws", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
	w := httptest.NewRecorder()

	srv.handleMonitorWebSocket(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestAuthenticateWSRequestWithoutStore(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	req := httptest.NewRequest(http.MethodGet, "/api/monitor/ws?token=test-token", nil)

	_, err := srv.authenticateWSRequest(req)

	if err == nil {
		t.Error("expected error when using share token without store")
	}
}

func TestWebSocketUpgradeFailure(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	adminAcc := getAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	// Create a request without proper WebSocket upgrade headers
	req := httptest.NewRequest(http.MethodGet, "/api/monitor/ws", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
	// Don't set WebSocket upgrade headers

	w := httptest.NewRecorder()

	srv.handleMonitorWebSocket(w, req)

	// The upgrade should fail, but we can't easily test the exact behavior
	// since httptest.ResponseRecorder doesn't support WebSocket upgrades
	// This test mainly ensures no panic occurs
}
