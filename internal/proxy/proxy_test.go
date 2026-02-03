package proxy

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// buildServerNoWarmup builds a server for tests and disables warmup to avoid
// spawning external CLI processes or waiting on real network calls.
func buildServerNoWarmup(t *testing.T, b *Builder) *Server {
	t.Helper()
	prevMethod := defaultHealthCheckMethod
	defaultHealthCheckMethod = HealthCheckMethodHEAD
	t.Cleanup(func() { defaultHealthCheckMethod = prevMethod })

	// Use temporary SQLite database for each test
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	// Set PROXY_SQLITE_PATH to override default path
	oldPath := os.Getenv("PROXY_SQLITE_PATH")
	os.Setenv("PROXY_SQLITE_PATH", tmpDB)
	t.Cleanup(func() {
		if oldPath != "" {
			os.Setenv("PROXY_SQLITE_PATH", oldPath)
		} else {
			os.Unsetenv("PROXY_SQLITE_PATH")
		}
	})

	srv, err := b.Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}
	srv.warmupConfig.Enabled = false
	return srv
}

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

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(upstream.URL).
		WithListenAddr(listener.Addr().String()))

	// Create test account and node
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := srv.addNodeToAccount(acc, "test-node", upstream.URL, "", 1); err != nil {
		t.Fatalf("add node: %v", err)
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

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(upA.URL).
		WithListenAddr(listener.Addr().String()))

	// Create test account
	acc, err := srv.createAccount("test-account", "client-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Add node A
	if _, err := srv.addNodeToAccount(acc, "A", upA.URL, "", 1); err != nil {
		t.Fatalf("add node A: %v", err)
	}

	// Add node B
	nodeB, err := srv.addNodeToAccount(acc, "B", upB.URL, "kB", 1)
	if err != nil {
		t.Fatalf("add node B: %v", err)
	}

	if err := srv.activate("n-bad-id"); err == nil {
		t.Fatalf("activate should fail on bad id")
	}
	// 激活 B
	if err := srv.activate(nodeB.ID); err != nil {
		t.Fatalf("activate B: %v", err)
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

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(up.URL).
		WithRetry(3).
		WithListenAddr(listener.Addr().String()))

	// Create test account and node
	acc, err := srv.createAccount("test-account", "client-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := srv.addNodeToAccount(acc, "test-node", up.URL, "", 1); err != nil {
		t.Fatalf("add node: %v", err)
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

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(up.URL).
		WithRetry(2).
		WithFailLimit(2).
		WithHealthEvery(5*time.Second).
		WithListenAddr(listener.Addr().String()))

	// Create admin account
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
		t.Fatalf("admin account not found")
	}

	go http.Serve(listener, srv.Handler())
	sess := srv.sessionMgr.Create(adminAcc.ID, true)

	// First, set the config to known values
	initReq := httptest.NewRequest(http.MethodPut, "/admin/api/config", strings.NewReader(`{"retries":2,"fail_limit":2,"health_interval_sec":5}`))
	initReq.Header.Set("Content-Type", "application/json")
	initReq.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	initRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("initial config PUT status %d, body: %s", initRec.Code, initRec.Body.String())
	}

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
	if cfgResp["retries"] != 2 || cfgResp["fail_limit"] != 2 || cfgResp["health_interval_sec"] != 5 {
		t.Fatalf("unexpected config payload: %+v", cfgResp)
	}

	// PUT update
	updateReq := httptest.NewRequest(http.MethodPut, "/admin/api/config", strings.NewReader(`{"retries":4,"fail_limit":5,"health_interval_sec":9}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	updateRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config PUT status %d", updateRec.Code)
	}

	// Verify config was updated via GET
	resp2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/admin/api/config", nil)
	req2.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
	srv.Handler().ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("config GET status %d", resp2.Code)
	}
	var cfgResp2 map[string]int
	if err := json.NewDecoder(resp2.Body).Decode(&cfgResp2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfgResp2["retries"] != 4 || cfgResp2["fail_limit"] != 5 || cfgResp2["health_interval_sec"] != 9 {
		t.Fatalf("config not updated via API: %+v", cfgResp2)
	}

	// invalid values
	badReq := httptest.NewRequest(http.MethodPut, "/admin/api/config", strings.NewReader(`{"retries":0,"fail_limit":0,"health_interval_sec":0}`))
	badReq.Header.Set("Content-Type", "application/json")
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

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(upA.URL).
		WithAPIKey("client-key").
		WithRetry(1).
		WithFailLimit(1).
		WithHealthEvery(200*time.Millisecond).
		WithListenAddr(listener.Addr().String()))

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "client-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	defaultNode, err := srv.addNodeToAccount(acc, "default", upA.URL, "client-key", 2)
	if err != nil {
		t.Fatalf("add default node: %v", err)
	}

	go http.Serve(listener, srv.Handler())

	// 第一次请求失败并熔断 default 节点。
	reqFail, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/v1/messages", strings.NewReader("{}"))
	reqFail.Header.Set("x-api-key", "client-key")
	resp, _ := http.DefaultClient.Do(reqFail)
	if resp == nil || resp.StatusCode == http.StatusOK {
		t.Fatalf("expected failure status, got %d", resp.StatusCode)
	}

	// Now add the backup node after the default has failed
	backupNode, err := srv.addNodeToAccount(acc, "backup", upB.URL, "", 1)
	if err != nil {
		t.Fatalf("add backup node: %v", err)
	}

	// 等待健康检查把 failed 节点保持失败，选择权重最低的健康节点（backup）。
	reqOk, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/v1/messages", strings.NewReader("{}"))
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
	_ = defaultNode // avoid unused variable error
	_ = backupNode  // avoid unused variable error
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

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(up.URL))

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	primary, err := srv.addNodeToAccount(acc, "primary", up.URL, "", 10)
	if err != nil {
		t.Fatalf("add primary node: %v", err)
	}

	low, err := srv.addNodeToAccount(acc, "low", up.URL, "", 1)
	if err != nil {
		t.Fatalf("add low node: %v", err)
	}

	if acc.ActiveID != low.ID {
		t.Fatalf("expected auto switch to lowest weight node, got %s", acc.ActiveID)
	}

	node, err := srv.getActiveNode(acc)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if node.ID != low.ID {
		t.Fatalf("expected switch to lowest weight node, got %s", node.ID)
	}
	if acc.ActiveID != low.ID {
		t.Fatalf("activeID not updated, got %s", acc.ActiveID)
	}
	_ = primary // avoid unused variable error
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

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(upA.URL))

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	defNode, err := srv.addNodeToAccount(acc, "default", upA.URL, "", 2)
	if err != nil {
		t.Fatalf("add default node: %v", err)
	}

	backup, err := srv.addNodeToAccount(acc, "backup", upB.URL, "", 1)
	if err != nil {
		t.Fatalf("add backup: %v", err)
	}
	if err := srv.activate(backup.ID); err != nil {
		t.Fatalf("activate backup: %v", err)
	}

	if err := srv.disableNode(backup.ID); err != nil {
		t.Fatalf("disable active: %v", err)
	}

	active, err := srv.getActiveNode(acc)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.ID != defNode.ID {
		t.Fatalf("expected switch to default after disabling active, got %s", active.ID)
	}
}

func TestEnableNodeAutoSwitchesByPriority(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(up.URL))

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	primary, err := srv.addNodeToAccount(acc, "primary", up.URL, "", 5)
	if err != nil {
		t.Fatalf("add primary: %v", err)
	}

	low, err := srv.addNodeToAccount(acc, "low", up.URL, "", 1)
	if err != nil {
		t.Fatalf("add low: %v", err)
	}
	if err := srv.disableNode(low.ID); err != nil {
		t.Fatalf("pre-disable low: %v", err)
	}

	if acc.ActiveID != primary.ID {
		t.Fatalf("expected primary active before enable, got %s", acc.ActiveID)
	}

	if err := srv.enableNode(low.ID); err != nil {
		t.Fatalf("enable low: %v", err)
	}

	if acc.ActiveID != low.ID {
		t.Fatalf("expected auto switch to enabled higher priority node, got %s", acc.ActiveID)
	}
}

func TestAccountsCreateStoresPassword(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(up.URL))

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

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(up.URL))

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

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(up.URL))

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

// TestNodeRecoveryAutoSwitch tests that when nodes recover, the system automatically
// switches to the highest priority (lowest weight) healthy node.
// Scenario:
// 1. Create 3 nodes with weights 1, 2, 3
// 2. All nodes fail
// 3. Node 3 recovers -> should switch to node 3 (only healthy)
// 4. Node 2 recovers -> should switch to node 2 (weight 2 < 3)
// 5. Node 1 recovers -> should switch to node 1 (weight 1 is smallest)
func TestNodeRecoveryAutoSwitch(t *testing.T) {
	// Create 3 test servers with controllable health
	healthy1, healthy2, healthy3 := false, false, false

	up1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy1 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":[{"text":"ok"}]}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer up1.Close()

	up2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy2 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":[{"text":"ok"}]}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer up2.Close()

	up3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy3 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":[{"text":"ok"}]}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer up3.Close()

	// Create proxy server with fast health check
	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(up1.URL).
		WithAPIKey("test-key").
		WithFailLimit(1).
		WithHealthEvery(300*time.Millisecond))

	// Start health check loop
	go srv.healthLoop()

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node1, err := srv.addNodeToAccount(acc, "node1", up1.URL, "", 1)
	if err != nil {
		t.Fatalf("add node1: %v", err)
	}

	// Add nodes 2 and 3
	node2, err := srv.addNodeToAccount(acc, "node2", up2.URL, "", 2)
	if err != nil {
		t.Fatalf("add node2: %v", err)
	}
	node3, err := srv.addNodeToAccount(acc, "node3", up3.URL, "", 3)
	if err != nil {
		t.Fatalf("add node3: %v", err)
	}

	// Manually mark all nodes as failed to simulate failure
	srv.mu.Lock()
	serverAcc := srv.accountByID[acc.ID]
	srv.nodeIndex[node1.ID].Failed = true
	srv.nodeIndex[node1.ID].Metrics.FailStreak = 1
	serverAcc.FailedSet[node1.ID] = struct{}{}
	srv.nodeIndex[node2.ID].Failed = true
	srv.nodeIndex[node2.ID].Metrics.FailStreak = 1
	serverAcc.FailedSet[node2.ID] = struct{}{}
	srv.nodeIndex[node3.ID].Failed = true
	srv.nodeIndex[node3.ID].Metrics.FailStreak = 1
	serverAcc.FailedSet[node3.ID] = struct{}{}
	srv.mu.Unlock()

	// Verify all nodes are failed
	srv.mu.RLock()
	if !srv.nodeIndex[node1.ID].Failed {
		t.Errorf("node1 should be failed")
	}
	if !srv.nodeIndex[node2.ID].Failed {
		t.Errorf("node2 should be failed")
	}
	if !srv.nodeIndex[node3.ID].Failed {
		t.Errorf("node3 should be failed")
	}
	srv.mu.RUnlock()

	// Scenario 1: Node 3 recovers (should become active)
	t.Log("Scenario 1: Node 3 recovers")
	healthy3 = true
	time.Sleep(800 * time.Millisecond) // Wait for health check

	srv.mu.RLock()
	currentAcc := srv.accountByID[acc.ID]
	activeID := currentAcc.ActiveID
	node3Failed := srv.nodeIndex[node3.ID].Failed
	srv.mu.RUnlock()
	if node3Failed {
		t.Errorf("node3 should be healthy after recovery")
	}
	if activeID != node3.ID {
		t.Errorf("expected node3 to be active after recovery, got %s", activeID)
	}

	// Scenario 2: Node 2 recovers (should switch to node2 due to lower weight)
	t.Log("Scenario 2: Node 2 recovers")
	healthy2 = true
	time.Sleep(800 * time.Millisecond) // Wait for health check

	srv.mu.RLock()
	currentAcc = srv.accountByID[acc.ID]
	activeID = currentAcc.ActiveID
	node2Failed := srv.nodeIndex[node2.ID].Failed
	srv.mu.RUnlock()
	if node2Failed {
		t.Errorf("node2 should be healthy after recovery")
	}
	if activeID != node2.ID {
		t.Errorf("expected node2 to be active after recovery (weight 2 < 3), got %s", activeID)
	}

	// Scenario 3: Node 1 recovers (should switch to node1 due to lowest weight)
	t.Log("Scenario 3: Node 1 recovers")
	healthy1 = true
	time.Sleep(800 * time.Millisecond) // Wait for health check

	srv.mu.RLock()
	currentAcc = srv.accountByID[acc.ID]
	activeID = currentAcc.ActiveID
	node1Failed := srv.nodeIndex[node1.ID].Failed
	srv.mu.RUnlock()
	if node1Failed {
		t.Errorf("node1 should be healthy after recovery")
	}
	if activeID != node1.ID {
		t.Errorf("expected node1 to be active after recovery (weight 1 is lowest), got %s", activeID)
	}

	// Reverse scenario: Node 1 fails again
	t.Log("Scenario 4: Node 1 fails again")
	healthy1 = false
	// Simulate failure by incrementing FailStreak and calling handleFailure
	srv.mu.Lock()
	srv.nodeIndex[node1.ID].Metrics.FailStreak = 1
	srv.mu.Unlock()
	srv.handleFailure(node1.ID, "simulated failure")
	time.Sleep(100 * time.Millisecond) // Small delay for processing

	srv.mu.RLock()
	currentAcc = srv.accountByID[acc.ID]
	activeID = currentAcc.ActiveID
	srv.mu.RUnlock()
	if activeID != node2.ID {
		t.Errorf("expected node2 to be active after node1 fails, got %s", activeID)
	}
}
