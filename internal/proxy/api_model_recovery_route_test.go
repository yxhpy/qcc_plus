package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleModelRecoveryListRoute(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/model-recovery", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
