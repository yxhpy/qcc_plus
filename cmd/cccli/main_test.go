package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"qcc_plus/internal/proxy"
	"qcc_plus/internal/store"
)

func TestCommandSmoke_GeminiProxyStreamIgnoresIncompleteTrailingJSON(t *testing.T) {
	var (
		mu       sync.Mutex
		gotPath  string
		gotQuery string
		gotBody  map[string]any
	)

	upstreamURL, closeUpstream := startLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"OK\"}]}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"BROKEN\"}]}}"))
	}))
	defer closeUpstream()

	dbPath := filepath.Join(t.TempDir(), "proxy.db")
	seedProxyStore(t, dbPath, upstreamURL)
	t.Setenv("PROXY_SQLITE_PATH", dbPath)
	t.Setenv("METRICS_SCHEDULER_ENABLED", "false")
	t.Setenv("QCC_SKIP_TUNNEL_AUTOSTART", "1")
	t.Setenv("PROXY_HEALTH_CHECK_MODE", "head")

	srv, err := proxy.NewBuilder().
		WithUpstream(upstreamURL).
		WithRetry(1).
		WithEnv().
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer proxyListener.Close()
	httpServer := &http.Server{Handler: srv.Handler()}
	go httpServer.Serve(proxyListener)
	defer httpServer.Close()

	cliCmd := exec.Command("go", "run", "./cmd/cccli", "hello")
	cliCmd.Dir = filepath.Join("..", "..")
	cliCmd.Env = append(cliCmd.Environ(),
		"ANTHROPIC_AUTH_TOKEN=test-token",
		"ANTHROPIC_BASE_URL=http://"+proxyListener.Addr().String(),
		"MODEL=gemini-node/gemini-2.5-flash",
		"NO_WARMUP=1",
		"MINIMAL_SYSTEM=1",
	)
	output, err := cliCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli command failed: %v\n%s", err, string(output))
	}

	out := string(output)
	if !strings.Contains(out, "OK") {
		t.Fatalf("expected cli output to contain streamed OK text, got %s", out)
	}
	if strings.Contains(out, "Incomplete JSON segment at the end") {
		t.Fatalf("cli output still contains incomplete JSON error: %s", out)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/v1beta/models/gemini-2.5-flash:streamGenerateContent" {
		t.Fatalf("expected upstream streamGenerateContent path, got %s", gotPath)
	}
	if gotQuery != "alt=sse" {
		t.Fatalf("expected upstream alt=sse query, got %q", gotQuery)
	}
	if _, exists := gotBody["contents"]; !exists {
		t.Fatalf("expected translated Gemini contents payload, got %#v", gotBody)
	}
}

func TestCommandSmoke_RealGeminiCLIThroughProxy(t *testing.T) {
	if _, err := exec.LookPath("gemini"); err != nil {
		t.Skipf("gemini CLI not installed: %v", err)
	}

	type upstreamHit struct {
		path  string
		query string
		body  map[string]any
	}

	var (
		mu   sync.Mutex
		hits []upstreamHit
	)

	upstreamURL, closeUpstream := startLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(bodyBytes, &body)

		mu.Lock()
		hits = append(hits, upstreamHit{
			path:  r.URL.Path,
			query: r.URL.RawQuery,
			body:  body,
		})
		mu.Unlock()

		switch {
		case strings.Contains(r.URL.Path, ":streamGenerateContent"):
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"OK\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1,\"totalTokenCount\":2}}\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case strings.Contains(r.URL.Path, ":generateContent"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"OK"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer closeUpstream()

	dbPath := filepath.Join(t.TempDir(), "proxy.db")
	seedProxyStore(t, dbPath, upstreamURL)
	t.Setenv("PROXY_SQLITE_PATH", dbPath)
	t.Setenv("METRICS_SCHEDULER_ENABLED", "false")
	t.Setenv("QCC_SKIP_TUNNEL_AUTOSTART", "1")
	t.Setenv("PROXY_HEALTH_CHECK_MODE", "head")

	srv, err := proxy.NewBuilder().
		WithUpstream(upstreamURL).
		WithRetry(1).
		WithEnv().
		Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer proxyListener.Close()
	httpServer := &http.Server{Handler: srv.Handler()}
	go httpServer.Serve(proxyListener)
	defer httpServer.Close()

	cliCmd := exec.Command("gemini",
		"-p", "Say OK only.",
		"-m", "gemini-2.5-flash",
		"--output-format", "text",
	)
	cliCmd.Dir = filepath.Join("..", "..")
	cliCmd.Env = append(cliCmd.Environ(),
		"GEMINI_API_KEY=test-token",
		"GOOGLE_GEMINI_BASE_URL=http://"+proxyListener.Addr().String(),
		"NO_COLOR=1",
		"CI=1",
	)
	output, err := cliCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gemini CLI command failed: %v\n%s", err, string(output))
	}

	out := string(output)
	if !strings.Contains(out, "OK") {
		t.Fatalf("expected gemini CLI output to contain OK, got %s", out)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, hit := range hits {
		if hit.path == "/v1beta/models/gemini-2.5-flash:streamGenerateContent" && hit.query == "alt=sse" {
			if _, exists := hit.body["contents"]; !exists {
				t.Fatalf("expected Gemini CLI upstream body to contain contents, got %#v", hit.body)
			}
			return
		}
	}
	t.Fatalf("expected gemini CLI to call streamGenerateContent?alt=sse, got %#v", hits)
}

func seedProxyStore(t *testing.T, dbPath, upstreamURL string) {
	t.Helper()

	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	accountID := "cli-test"
	now := time.Now()
	if err := st.CreateAccount(ctx, store.AccountRecord{
		ID:          accountID,
		Name:        "cli-test",
		Password:    "test123",
		ProxyAPIKey: "test-token",
		IsAdmin:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create account: %v", err)
	}

	nodeID := "gemini-node-id"
	if err := st.UpsertNode(ctx, store.NodeRecord{
		ID:                nodeID,
		Name:              "gemini-node",
		BaseURL:           upstreamURL,
		APIKey:            "AIza-test-key",
		HealthCheckMethod: "head",
		HealthCheckModel:  "gemini-2.5-flash",
		SourceProtocol:    "gemini",
		AccountID:         accountID,
		Weight:            1,
		CreatedAt:         now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := st.SetActive(ctx, accountID, nodeID); err != nil {
		t.Fatalf("set active node: %v", err)
	}
}

func startLocalHTTPServer(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen local server: %v", err)
	}
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	return "http://" + listener.Addr().String(), func() {
		_ = server.Close()
		_ = listener.Close()
	}
}
