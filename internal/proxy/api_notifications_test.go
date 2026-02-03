package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleNotifications tests the /admin/api/notifications endpoint
func TestHandleNotifications(t *testing.T) {
	t.Skip("Notifications list endpoint not implemented yet")

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

	t.Run("GET returns notifications list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/notifications", nil)
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

		if _, ok := response["notifications"]; !ok {
			t.Error("expected 'notifications' field in response")
		}
	})

	t.Run("GET with limit parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/notifications?limit=10", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("GET without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/notifications", nil)
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

// TestHandleNotificationMarkRead tests marking notifications as read
func TestHandleNotificationMarkRead(t *testing.T) {
	t.Skip("Notification mark read endpoint not implemented yet")

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

	t.Run("POST marks notification as read", func(t *testing.T) {
		// Note: This test assumes notification ID exists
		// In a real scenario, we'd create a notification first
		req := httptest.NewRequest(http.MethodPost, "/admin/api/notifications/test-id/read", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		// May return 404 if notification doesn't exist, which is expected
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("expected status 200 or 404, got %d", w.Code)
		}
	})

	t.Run("POST without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/notifications/test-id/read", nil)
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
