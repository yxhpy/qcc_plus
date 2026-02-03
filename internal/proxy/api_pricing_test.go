package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePricing(t *testing.T) {
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
	if adminAcc == nil {
		t.Fatal("admin account not found")
	}

	// Create admin session
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("GET lists all pricing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
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

		if _, ok := response["pricing"]; !ok {
			t.Error("expected 'pricing' field in response")
		}
	})

	t.Run("POST creates pricing (admin only)", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"model_id": "claude-3-opus",
			"model_name": "Claude 3 Opus",
			"input_price": 15.0,
			"output_price": 75.0,
			"active": true
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pricing", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST without model_id returns bad request", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"model_name": "Test Model"
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pricing", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("POST with invalid JSON returns bad request", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pricing", body)
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
			t.Fatalf("failed to create non-admin account: %v", err)
		}
		nonAdminSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)

		body := bytes.NewBufferString(`{
			"model_id": "test-model",
			"model_name": "Test Model"
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pricing", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: nonAdminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("DELETE removes pricing (admin only)", func(t *testing.T) {
		// First create a pricing
		body := bytes.NewBufferString(`{
			"model_id": "test-delete-model",
			"model_name": "Test Delete Model"
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pricing", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		// Then delete it
		req = httptest.NewRequest(http.MethodDelete, "/api/pricing?id=test-delete-model", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w = httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("DELETE without id returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/pricing", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("DELETE without admin session returns forbidden", func(t *testing.T) {
		nonAdminAcc, _ := srv.createAccount("non-admin2", "non-admin-key2", "pass", false)
		nonAdminSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)

		req := httptest.NewRequest(http.MethodDelete, "/api/pricing?id=test-model", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: nonAdminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("PUT returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/pricing", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestHandleUsageLogs(t *testing.T) {
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

	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("GET returns usage logs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/logs", nil)
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

		if _, ok := response["logs"]; !ok {
			t.Error("expected 'logs' field in response")
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/usage/logs", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestHandleUsageSummary(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	var adminAcc *Account
	srv.mu.RLock()
	for _, acc := range srv.accountByID {
		if acc.IsAdmin {
			adminAcc = acc
			break
		}
	}
	srv.mu.RUnlock()

	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("GET returns overall summary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/summary", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET with group_by=model returns model summaries", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/summary?group_by=model", nil)
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

		if _, ok := response["summaries"]; !ok {
			t.Error("expected 'summaries' field in response")
		}
	})

	t.Run("GET with group_by=node returns node summaries", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/summary?group_by=node", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/usage/summary", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestHandleUsageCleanup(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	var adminAcc *Account
	srv.mu.RLock()
	for _, acc := range srv.accountByID {
		if acc.IsAdmin {
			adminAcc = acc
			break
		}
	}
	srv.mu.RUnlock()

	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("POST cleans up usage logs", func(t *testing.T) {
		body := bytes.NewBufferString(`{"retention_days": 30}`)
		req := httptest.NewRequest(http.MethodPost, "/api/usage/cleanup", body)
		req.Header.Set("Content-Type", "application/json")
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

		if response["retention_days"] != float64(30) {
			t.Errorf("expected retention_days 30, got %v", response["retention_days"])
		}
	})

	t.Run("POST without admin session returns forbidden", func(t *testing.T) {
		nonAdminAcc, _ := srv.createAccount("non-admin3", "non-admin-key3", "pass", false)
		nonAdminSess := srv.sessionMgr.Create(nonAdminAcc.ID, false)

		body := bytes.NewBufferString(`{"retention_days": 30}`)
		req := httptest.NewRequest(http.MethodPost, "/api/usage/cleanup", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: nonAdminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("GET returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/usage/cleanup", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}
