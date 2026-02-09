package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleAccounts tests the /admin/api/accounts endpoint
func TestHandleAccounts(t *testing.T) {
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

	t.Run("GET returns accounts list for admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/accounts", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["accounts"]; !ok {
			t.Error("expected 'accounts' field in response")
		}
	})

	t.Run("POST creates new account for admin", func(t *testing.T) {
		body := strings.NewReader(`{
			"name": "new-account",
			"proxy_api_key": "new-key",
			"password": "password123",
			"is_admin": false
		}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
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
		body := strings.NewReader(`{invalid}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("POST without admin session returns forbidden", func(t *testing.T) {
		// Create non-admin account
		nonAdminAcc, err := srv.createAccount("non-admin", "non-admin-key", "pass", false)
		if err != nil {
			t.Fatalf("create non-admin account: %v", err)
		}
		nonAdminSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)

		body := strings.NewReader(`{"name":"test","proxy_api_key":"key","password":"pass"}`)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/accounts", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: nonAdminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("GET without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/accounts", nil)
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
