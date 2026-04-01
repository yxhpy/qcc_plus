package client

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"qcc_plus/internal/proxy"
	"qcc_plus/internal/store"
)

func TestSend_ViaGeminiProxy_IgnoresIncompleteTrailingJSONSegment(t *testing.T) {
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

	srv, err := proxy.NewBuilder().
		WithUpstream(upstreamURL).
		WithRetry(1).
		Build()
	if err != nil {
		t.Fatalf("build proxy server: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer listener.Close()

	httpServer := &http.Server{Handler: srv.Handler()}
	go httpServer.Serve(listener)
	defer httpServer.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: "http://" + listener.Addr().String(),
	}
	body := Body{
		Model: "gemini-node/gemini-2.5-flash",
		Messages: []Message{{
			Role: "user",
			Content: []ContentItem{{
				Type: "text",
				Text: "hello",
			}},
		}},
		MaxTokens: 32,
		Stream:    true,
	}

	if err := send(context.Background(), cfg, body, "test-beta"); err != nil {
		t.Fatalf("send via Gemini proxy returned error: %v", err)
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
		t.Fatalf("expected Gemini contents payload, got %#v", gotBody)
	}
	if _, exists := gotBody["model"]; exists {
		t.Fatalf("expected Claude model field to be removed, got %#v", gotBody)
	}
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
