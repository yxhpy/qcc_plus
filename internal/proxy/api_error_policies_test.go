package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleErrorPolicies(t *testing.T) {
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
	if adminAcc == nil {
		t.Fatal("admin account not found")
	}
	sess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("list policies", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/error-policies", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("toggle observed policy", func(t *testing.T) {
		srv.errorPolicy.Record(400, "packy_api_error", "gateway error")
		snap := srv.errorPolicy.Snapshot()
		if len(snap.Observed) == 0 {
			t.Fatal("expected observed entries")
		}
		id := snap.Observed[0].ID

		body := strings.NewReader(`{"type":"observed","id":"` + id + `","enabled":true}`)
		req := httptest.NewRequest(http.MethodPost, "/api/error-policies/toggle", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}
