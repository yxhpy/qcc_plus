package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSession(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

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

	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("GET returns current session account", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var response struct {
			Account map[string]any `json:"account"`
		}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if response.Account["id"] != adminAcc.ID {
			t.Fatalf("account id=%v want=%s", response.Account["id"], adminAcc.ID)
		}
		if response.Account["name"] != adminAcc.Name {
			t.Fatalf("account name=%v want=%s", response.Account["name"], adminAcc.Name)
		}
		if response.Account["is_admin"] != true {
			t.Fatalf("is_admin=%v want=true", response.Account["is_admin"])
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/session", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("GET without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var response struct {
			Account any `json:"account"`
		}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.Account != nil {
			t.Fatalf("account=%v want=nil", response.Account)
		}
	})
}
