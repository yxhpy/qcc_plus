package proxy

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuilderMissingUpstream(t *testing.T) {
	_, err := NewBuilder().Build()
	if err == nil {
		t.Fatalf("expected error for missing upstream")
	}
	if err != ErrUpstreamMissing {
		t.Fatalf("expected ErrUpstreamMissing, got %v", err)
	}
}

func TestProxyForwardsRequests(t *testing.T) {
	// Upstream echo server capturing Host and path.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "" {
			t.Fatalf("empty Host header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok:" + r.URL.Path))
	}))
	defer upstream.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv, err := NewBuilder().
		WithUpstream(upstream.URL).
		WithAPIKey("test-proxy").
		WithListenAddr(listener.Addr().String()).
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	go http.Serve(listener, srv.Handler())

	req, _ := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/hello", nil)
	req.Header.Set("x-api-key", "test-proxy")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get through proxy: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if got, want := string(b), "ok:/hello"; got != want {
		t.Fatalf("unexpected body: %s want %s", got, want)
	}
}

func TestProxySwitchActiveNode(t *testing.T) {
	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("A:" + r.Host))
	}))
	defer upA.Close()

	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "kB" {
			t.Fatalf("expected injected key, got %s", got)
		}
		w.Write([]byte("B:" + r.Host))
	}))
	defer upB.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv, err := NewBuilder().
		WithUpstream(upA.URL).
		WithAPIKey("client-key").
		WithNodeName("A").
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	if _, err := srv.addNode("B", upB.URL, "kB", 1); err != nil {
		t.Fatalf("add node: %v", err)
	}
	if err := srv.activate("n-"); err == nil {
		t.Fatalf("activate should fail on bad id")
	}
	// 激活 B
	for id, node := range srv.defaultAccount.Nodes {
		if node.Name == "B" {
			srv.activate(id)
		}
	}

	go http.Serve(listener, srv.Handler())

	req, _ := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/hi", nil)
	req.Header.Set("x-api-key", "client-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get through proxy: %v", err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(buf), "B:") {
		t.Fatalf("expected upstream B, got %s", string(buf))
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestRetryOnNon200(t *testing.T) {
	tries := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tries++
		if tries < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer up.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv, err := NewBuilder().
		WithUpstream(up.URL).
		WithAPIKey("client-key").
		WithRetry(3).
		WithListenAddr(listener.Addr().String()).
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	go http.Serve(listener, srv.Handler())

	req, _ := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/ok", nil)
	req.Header.Set("x-api-key", "client-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retries, got %d", resp.StatusCode)
	}
	if tries != 3 {
		t.Fatalf("expected 3 attempts, got %d", tries)
	}
}

func TestHandleConfigGetAndPut(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer up.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv, err := NewBuilder().
		WithUpstream(up.URL).
		WithRetry(2).
		WithFailLimit(2).
		WithHealthEvery(2 * time.Second).
		WithListenAddr(listener.Addr().String()).
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	go http.Serve(listener, srv.Handler())
	sess := srv.sessionMgr.Create(srv.defaultAccount.ID, true)

	// GET config
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/config", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	srv.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("config GET status %d", resp.Code)
	}
	var cfgResp map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&cfgResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfgResp["retries"] != 2 || cfgResp["fail_limit"] != 2 || cfgResp["health_interval_sec"] != 2 {
		t.Fatalf("unexpected config payload: %+v", cfgResp)
	}

	// PUT update
	updateReq := httptest.NewRequest(http.MethodPut, "/admin/api/config", strings.NewReader(`{"retries":4,"fail_limit":5,"health_interval_sec":9}`))
	updateReq.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	updateRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config PUT status %d", updateRec.Code)
	}

	newCfg := srv.getConfig()
	if newCfg.Retries != 4 || newCfg.FailLimit != 5 || newCfg.HealthEvery != 9*time.Second {
		t.Fatalf("config not updated: %+v", newCfg)
	}
	if rt, ok := srv.transport.(*retryTransport); ok {
		if rt.attempts != 4 {
			t.Fatalf("retry transport not updated, attempts=%d", rt.attempts)
		}
	}

	// invalid values
	badReq := httptest.NewRequest(http.MethodPut, "/admin/api/config", strings.NewReader(`{"retries":0,"fail_limit":0,"health_interval_sec":0}`))
	badReq.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	badRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid config, got %d", badRec.Code)
	}
}

func TestAutoFailoverByWeight(t *testing.T) {
	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upA.Close()

	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("B"))
	}))
	defer upB.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv, err := NewBuilder().
		WithUpstream(upA.URL).
		WithAPIKey("client-key").
		WithRetry(1).
		WithFailLimit(1).
		WithHealthEvery(200 * time.Millisecond).
		WithListenAddr(listener.Addr().String()).
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	if _, err := srv.addNode("backup", upB.URL, "", 1); err != nil {
		t.Fatalf("add node: %v", err)
	}

	go http.Serve(listener, srv.Handler())

	// 第一次请求失败并熔断 default 节点。
	reqFail, _ := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/fails", nil)
	reqFail.Header.Set("x-api-key", "client-key")
	resp, _ := http.DefaultClient.Do(reqFail)
	if resp == nil || resp.StatusCode == http.StatusOK {
		t.Fatalf("expected failure status")
	}

	// 等待健康检查把 failed 节点保持失败，选择权重最低的健康节点（backup）。
	reqOk, _ := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/ok", nil)
	reqOk.Header.Set("x-api-key", "client-key")
	resp2, err := http.DefaultClient.Do(reqOk)
	if err != nil {
		t.Fatalf("second request err: %v", err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if string(body) != "B" {
		t.Fatalf("expected fallback to B, got %s", string(body))
	}
}

func TestParseUsageFromSSE(t *testing.T) {
	s := []byte("event: message_start\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\"}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\",\"usage\":{\"input_tokens\":11,\"output_tokens\":22}}\n\n")
	in, out := parseUsage(s)
	if in != 11 || out != 22 {
		t.Fatalf("unexpected usage %d %d", in, out)
	}
}

func TestGetActiveSwitchesToLowerWeight(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv, err := NewBuilder().WithUpstream(up.URL).Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	primary := srv.getNode("default")
	if primary == nil {
		t.Fatalf("default node missing")
	}
	if err := srv.updateNode("default", primary.Name, primary.URL.String(), &primary.APIKey, 10); err != nil {
		t.Fatalf("update default weight: %v", err)
	}

	low, err := srv.addNode("low", up.URL, "", 1)
	if err != nil {
		t.Fatalf("add low node: %v", err)
	}

	if srv.defaultAccount.ActiveID != low.ID {
		t.Fatalf("expected auto switch to lowest weight node, got %s", srv.defaultAccount.ActiveID)
	}

	node, err := srv.getActiveNode(srv.defaultAccount)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if node.ID != low.ID {
		t.Fatalf("expected switch to lowest weight node, got %s", node.ID)
	}
	if srv.defaultAccount.ActiveID != low.ID {
		t.Fatalf("activeID not updated, got %s", srv.defaultAccount.ActiveID)
	}
}

func TestDisableActiveTriggersImmediateSwitch(t *testing.T) {
	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upA.Close()
	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upB.Close()

	srv, err := NewBuilder().WithUpstream(upA.URL).Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	def := srv.getNode("default")
	if def == nil {
		t.Fatalf("default node missing")
	}
	if err := srv.updateNode("default", def.Name, def.URL.String(), &def.APIKey, 2); err != nil {
		t.Fatalf("update default weight: %v", err)
	}

	backup, err := srv.addNode("backup", upB.URL, "", 1)
	if err != nil {
		t.Fatalf("add backup: %v", err)
	}
	if err := srv.activate(backup.ID); err != nil {
		t.Fatalf("activate backup: %v", err)
	}

	if err := srv.disableNode(backup.ID); err != nil {
		t.Fatalf("disable active: %v", err)
	}

	active, err := srv.getActiveNode(srv.defaultAccount)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.ID != "default" {
		t.Fatalf("expected switch to default after disabling active, got %s", active.ID)
	}
}

func TestEnableNodeAutoSwitchesByPriority(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv, err := NewBuilder().WithUpstream(up.URL).Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	primary := srv.getNode("default")
	if primary == nil {
		t.Fatalf("default node missing")
	}
	if err := srv.updateNode("default", primary.Name, primary.URL.String(), &primary.APIKey, 5); err != nil {
		t.Fatalf("update default weight: %v", err)
	}

	low, err := srv.addNode("low", up.URL, "", 1)
	if err != nil {
		t.Fatalf("add low: %v", err)
	}
	if err := srv.disableNode(low.ID); err != nil {
		t.Fatalf("pre-disable low: %v", err)
	}

	if srv.defaultAccount.ActiveID != "default" {
		t.Fatalf("expected default active before enable, got %s", srv.defaultAccount.ActiveID)
	}

	if err := srv.enableNode(low.ID); err != nil {
		t.Fatalf("enable low: %v", err)
	}

	if srv.defaultAccount.ActiveID != low.ID {
		t.Fatalf("expected auto switch to enabled higher priority node, got %s", srv.defaultAccount.ActiveID)
	}
}

func TestAccountsCreateStoresPassword(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv, err := NewBuilder().WithUpstream(up.URL).Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

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
		t.Fatalf("admin account missing")
	}
	sess := srv.sessionMgr.Create(adminAcc.ID, true)

	body := strings.NewReader(`{"name":"team-a","password":"secret6","proxy_api_key":"key-team","is_admin":false}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/accounts", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create account status %d", rec.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	id := resp["id"]
	if id == "" {
		t.Fatalf("missing account id in response")
	}

	created := srv.getAccountByID(id)
	if created == nil {
		t.Fatalf("account not registered")
	}
	if created.Password != "secret6" {
		t.Fatalf("password not stored, got %q", created.Password)
	}
}

func TestLoginWithUsernamePassword(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv, err := NewBuilder().WithUpstream(up.URL).Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=admin&password=admin123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	var hasSession bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session_token" && c.Value != "" {
			hasSession = true
			break
		}
	}
	if !hasSession {
		t.Fatalf("session cookie missing")
	}
}

func TestLoginEmptyPasswordShowsError(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv, err := NewBuilder().WithUpstream(up.URL).Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=admin&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "账号名称和密码不能为空") {
		t.Fatalf("expected empty password error, got body: %s", rec.Body.String())
	}
}
