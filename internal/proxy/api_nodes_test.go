package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleNodes tests the /admin/api/nodes endpoint
func TestHandleNodes(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Create session
	sess := srv.sessionMgr.Create(acc.ID, false)

	t.Run("GET returns nodes list", func(t *testing.T) {
		// Add a test node first
		_, err := srv.addNodeToAccount(acc, "test-node", "http://test.com", "key", 1)
		if err != nil {
			t.Fatalf("add node: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/admin/api/nodes", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["nodes"]; !ok {
			t.Error("expected 'nodes' field in response")
		}
	})

	t.Run("POST creates new node", func(t *testing.T) {
		body := strings.NewReader(`{
			"name": "new-node",
			"base_url": "http://new.com",
			"api_key": "new-key",
			"weight": 2
		}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["id"]; !ok {
			t.Error("expected 'id' field in response")
		}
	})

	t.Run("POST with invalid JSON returns bad request", func(t *testing.T) {
		body := strings.NewReader(`{invalid json}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("POST without session returns unauthorized", func(t *testing.T) {
		body := strings.NewReader(`{"name":"test","base_url":"http://test.com","weight":1}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/nodes", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
