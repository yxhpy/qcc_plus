package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleActivate tests the /admin/api/nodes/:id/activate endpoint
func TestHandleActivate(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node1, err := srv.addNodeToAccount(acc, "node1", "http://node1.com", "key1", 1)
	if err != nil {
		t.Fatalf("add node1: %v", err)
	}
	node2, err := srv.addNodeToAccount(acc, "node2", "http://node2.com", "key2", 2)
	if err != nil {
		t.Fatalf("add node2: %v", err)
	}

	// Create session
	sess := srv.sessionMgr.Create(acc.ID, false)

	t.Run("POST activates node successfully", func(t *testing.T) {
		body := `{"id":"` + node2.ID + `"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/activate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		// Verify node is activated
		// Need to reload account to get updated ActiveID
		updatedAcc := srv.getAccountByID(acc.ID)
		if updatedAcc == nil || updatedAcc.ActiveID != node2.ID {
			t.Errorf("expected node2 to be active, got %v", updatedAcc)
		}
	})

	t.Run("POST with invalid node ID returns not found", func(t *testing.T) {
		body := `{"id":"invalid-id"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/activate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("POST without session returns unauthorized", func(t *testing.T) {
		body := `{"id":"` + node1.ID + `"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/activate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

// TestHandleDisable tests the /admin/api/nodes/:id/disable endpoint
func TestHandleDisable(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account and node
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node, err := srv.addNodeToAccount(acc, "test-node", "http://test.com", "key", 1)
	if err != nil {
		t.Fatalf("add node: %v", err)
	}

	// Create session
	sess := srv.sessionMgr.Create(acc.ID, false)

	t.Run("POST disables node successfully", func(t *testing.T) {
		body := `{"id":"` + node.ID + `"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/disable", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		// Verify node is disabled
		srv.mu.RLock()
		disabled := srv.nodeIndex[node.ID].Disabled
		srv.mu.RUnlock()
		if !disabled {
			t.Error("expected node to be disabled")
		}
	})

	t.Run("POST with invalid node ID returns not found", func(t *testing.T) {
		body := `{"id":"invalid-id"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/disable", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

// TestHandleEnable tests the /admin/api/nodes/:id/enable endpoint
func TestHandleEnable(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account and node
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node, err := srv.addNodeToAccount(acc, "test-node", "http://test.com", "key", 1)
	if err != nil {
		t.Fatalf("add node: %v", err)
	}

	// Disable the node first
	if err := srv.disableNode(node.ID); err != nil {
		t.Fatalf("disable node: %v", err)
	}

	// Create session
	sess := srv.sessionMgr.Create(acc.ID, false)

	t.Run("POST enables node successfully", func(t *testing.T) {
		body := `{"id":"` + node.ID + `"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/enable", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		// Verify node is enabled
		srv.mu.RLock()
		disabled := srv.nodeIndex[node.ID].Disabled
		srv.mu.RUnlock()
		if disabled {
			t.Error("expected node to be enabled")
		}
	})

	t.Run("POST with invalid node ID returns not found", func(t *testing.T) {
		body := `{"id":"invalid-id"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes/enable", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

// TestHandleLogout tests the /logout endpoint
func TestHandleLogout(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account and session
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	sess := srv.sessionMgr.Create(acc.ID, false)

	t.Run("POST logs out successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("expected status 302, got %d", w.Code)
		}

		// Verify session is deleted
		if srv.sessionMgr.Get(sess.Token) != nil {
			t.Error("expected session to be deleted")
		}
	})
}

// TestHandleDashboardRedirect tests the /admin route
func TestHandleDashboardRedirect(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("GET redirects to dashboard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		// /admin serves the SPA, not a redirect
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}
