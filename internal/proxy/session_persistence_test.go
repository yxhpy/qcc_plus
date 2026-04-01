package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestSessionManagerPersistsAcrossInstances(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")

	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	mgr := NewSessionManager(time.Hour, st)
	sess := mgr.Create("account-1", true)
	if sess == nil {
		t.Fatal("expected session to be created")
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	st2, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer st2.Close()

	reloaded := NewSessionManager(time.Hour, st2).Get(sess.Token)
	if reloaded == nil {
		t.Fatal("expected session to survive manager rebuild")
	}
	if reloaded.AccountID != sess.AccountID {
		t.Fatalf("account_id=%s want=%s", reloaded.AccountID, sess.AccountID)
	}
	if !reloaded.IsAdmin {
		t.Fatalf("is_admin=%v want=true", reloaded.IsAdmin)
	}
}

func TestSessionManagerDeletesExpiredPersistentSession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()

	now := time.Now().UTC().Truncate(time.Second)
	record := store.SessionRecord{
		Token:     "expired-token",
		AccountID: "account-1",
		IsAdmin:   false,
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}
	if err := st.UpsertSession(context.Background(), record); err != nil {
		t.Fatalf("UpsertSession failed: %v", err)
	}

	mgr := NewSessionManager(time.Hour, st)
	if got := mgr.Get(record.Token); got != nil {
		t.Fatal("expected expired session to be invalid")
	}
	if _, err := st.GetSessionByToken(context.Background(), record.Token); err != store.ErrNotFound {
		t.Fatalf("expected expired session to be removed from store, got %v", err)
	}
}

func TestLoginSessionCookiePersistsAcrossServerRebuild(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	dbPath := filepath.Join(t.TempDir(), "proxy.db")
	oldPath := os.Getenv("PROXY_SQLITE_PATH")
	oldSkipTunnel := os.Getenv("QCC_SKIP_TUNNEL_AUTOSTART")
	os.Setenv("PROXY_SQLITE_PATH", dbPath)
	os.Setenv("QCC_SKIP_TUNNEL_AUTOSTART", "1")
	t.Cleanup(func() {
		if oldPath != "" {
			os.Setenv("PROXY_SQLITE_PATH", oldPath)
		} else {
			os.Unsetenv("PROXY_SQLITE_PATH")
		}
		if oldSkipTunnel != "" {
			os.Setenv("QCC_SKIP_TUNNEL_AUTOSTART", oldSkipTunnel)
		} else {
			os.Unsetenv("QCC_SKIP_TUNNEL_AUTOSTART")
		}
	})

	buildServer := func(t *testing.T) *Server {
		t.Helper()
		srv, err := NewBuilder().WithUpstream(upstream.URL).Build()
		if err != nil {
			t.Fatalf("build server: %v", err)
		}
		srv.warmupConfig.Enabled = false
		return srv
	}

	srv1 := buildServer(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=admin&password=admin123"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	srv1.Handler().ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusFound {
		t.Fatalf("login status=%d want=%d", loginRec.Code, http.StatusFound)
	}

	var sessionCookie *http.Cookie
	for _, cookie := range loginRec.Result().Cookies() {
		if cookie.Name == "session_token" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie")
	}
	if sessionCookie.MaxAge != int(defaultSessionTTL/time.Second) {
		t.Fatalf("cookie MaxAge=%d want=%d", sessionCookie.MaxAge, int(defaultSessionTTL/time.Second))
	}
	if !sessionCookie.Expires.After(time.Now().Add(29 * 24 * time.Hour)) {
		t.Fatalf("cookie Expires=%v is too short", sessionCookie.Expires)
	}

	srv1.Stop()
	if srv1.store != nil {
		if err := srv1.store.Close(); err != nil {
			t.Fatalf("close first store: %v", err)
		}
	}

	srv2 := buildServer(t)
	defer srv2.Stop()
	if srv2.store != nil {
		defer srv2.store.Close()
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	sessionReq.AddCookie(sessionCookie)
	sessionRec := httptest.NewRecorder()
	srv2.Handler().ServeHTTP(sessionRec, sessionReq)

	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status=%d want=%d", sessionRec.Code, http.StatusOK)
	}

	var response struct {
		Account map[string]any `json:"account"`
	}
	if err := json.NewDecoder(sessionRec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Account == nil {
		t.Fatal("expected account after server rebuild")
	}
	if response.Account["name"] != "admin" {
		t.Fatalf("account name=%v want=admin", response.Account["name"])
	}
}
