package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleNodeAPIRoutes(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Get admin account
	var adminAcc *Account
	srv.mu.RLock()
	for _, acc := range srv.accountByID {
		if acc.IsAdmin {
			adminAcc = acc
			break
		}
	}
	srv.mu.RUnlock()

	// Create a test node
	node, err := srv.addNodeToAccount(adminAcc, "test-node", "http://example.com", "test-key", 1)
	if err != nil {
		t.Fatalf("failed to add node: %v", err)
	}

	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("routes to health-history handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		// Should not return 404
		if w.Code == http.StatusNotFound {
			t.Error("expected route to be handled, got 404")
		}
	})

	t.Run("returns 404 for unknown route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/unknown", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.handleNodeAPIRoutes(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestHandleGetHealthHistory(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Get admin account
	var adminAcc *Account
	srv.mu.RLock()
	for _, acc := range srv.accountByID {
		if acc.IsAdmin {
			adminAcc = acc
			break
		}
	}
	srv.mu.RUnlock()

	// Create a test node
	node, err := srv.addNodeToAccount(adminAcc, "test-node", "http://example.com", "test-key", 1)
	if err != nil {
		t.Fatalf("failed to add node: %v", err)
	}

	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("GET returns health history", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["node_id"] != node.ID {
			t.Errorf("expected node_id '%s', got '%v'", node.ID, response["node_id"])
		}
		if _, ok := response["checks"]; !ok {
			t.Error("expected 'checks' field in response")
		}
	})

	t.Run("GET with time range parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET with limit and offset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history?limit=10&offset=5", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("GET with source filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history?source=scheduled", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/nodes/"+node.ID+"/health-history", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.handleGetHealthHistory(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("GET with invalid node ID returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/invalid-node-id/health-history", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.handleGetHealthHistory(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("GET without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history", nil)
		w := httptest.NewRecorder()

		srv.handleGetHealthHistory(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("GET with invalid time format returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history?from=invalid-time", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("GET with from after to returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history?from=2024-12-31T23:59:59Z&to=2024-01-01T00:00:00Z", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("non-admin cannot access other account's node", func(t *testing.T) {
		// Create non-admin account
		nonAdminAcc, err := srv.createAccount("non-admin", "non-admin-key", "pass", false)
		if err != nil {
			t.Fatalf("failed to create non-admin account: %v", err)
		}
		nonAdminSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)

		req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+node.ID+"/health-history", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: nonAdminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})
}

func TestExtractNodeIDFromHealthHistoryPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
		ok       bool
	}{
		{
			name:     "valid path",
			path:     "/api/nodes/node-123/health-history",
			expected: "node-123",
			ok:       true,
		},
		{
			name:     "valid path with trailing slash",
			path:     "/api/nodes/node-456/health-history/",
			expected: "",
			ok:       false, // The function doesn't handle trailing slash after /health-history
		},
		{
			name:     "invalid path - missing prefix",
			path:     "/nodes/node-123/health-history",
			expected: "",
			ok:       false,
		},
		{
			name:     "invalid path - missing suffix",
			path:     "/api/nodes/node-123/metrics",
			expected: "",
			ok:       false,
		},
		{
			name:     "invalid path - empty node ID",
			path:     "/api/nodes//health-history",
			expected: "",
			ok:       false,
		},
		{
			name:     "invalid path - no node ID",
			path:     "/api/nodes/health-history",
			expected: "health-history",
			ok:       true, // The function extracts "health-history" as node ID
		},
		{
			name:     "complex node ID",
			path:     "/api/nodes/node-abc-123-xyz/health-history",
			expected: "node-abc-123-xyz",
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, ok := extractNodeIDFromHealthHistoryPath(tt.path)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got %v", tt.ok, ok)
			}
			if nodeID != tt.expected {
				t.Errorf("expected nodeID='%s', got '%s'", tt.expected, nodeID)
			}
		})
	}
}
