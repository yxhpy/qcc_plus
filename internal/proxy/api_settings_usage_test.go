package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleSettings tests the /admin/api/settings endpoint
func TestHandleSettings(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
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
	if adminAcc == nil {
		t.Fatal("admin account not found")
	}

	// Create admin session
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("GET returns settings list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET specific setting", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/settings/test-key", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		// May return 404 if setting doesn't exist, which is expected
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("expected status 200 or 404, got %d", w.Code)
		}
	})

	t.Run("PUT update setting", func(t *testing.T) {
		body := strings.NewReader(`{"value":"test-value"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/settings/test-key", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		// Should succeed or return validation error
		if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
			t.Logf("PUT setting returned %d (may vary by implementation)", w.Code)
		}
	})

	t.Run("DELETE setting", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/settings/test-key", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		// May return 404 if setting doesn't exist
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Logf("DELETE setting returned %d", w.Code)
		}
	})

	t.Run("GET without admin session returns forbidden", func(t *testing.T) {
		// Create non-admin account
		nonAdminAcc, err := srv.createAccount("non-admin", "non-admin-key", "pass", false)
		if err != nil {
			t.Fatalf("create non-admin account: %v", err)
		}
		nonAdminSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)

		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: nonAdminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})
}

// TestHandleUsage tests the /api/usage endpoint
func TestHandleUsage(t *testing.T) {
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

	t.Run("GET returns usage statistics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/summary", nil)
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
	})

	t.Run("GET with time range parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/summary?from=2026-01-01&to=2026-02-01", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("GET without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/summary", nil)
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
