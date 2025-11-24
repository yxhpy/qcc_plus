package healthcheckcli

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"qcc_plus/internal/proxy"
	"qcc_plus/internal/store"
)

func newProxyForTest(t *testing.T, upstream string, runner proxy.CliRunner) *proxy.Server {
	t.Helper()
	builder := proxy.NewBuilder().
		WithUpstream(upstream).
		WithAPIKey("sk-test").
		WithLogger(log.New(io.Discard, "", 0))
	if runner != nil {
		builder = builder.WithCLIRunner(runner)
	}
	srv, err := builder.Build()
	if err != nil {
		t.Fatalf("build proxy: %v", err)
	}
	return srv
}

func TestHealthCheckAPI(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/messages" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiSrv.Close()

	srv := newProxyForTest(t, apiSrv.URL, nil)
	acc := srv.TestAccount(store.DefaultAccountID)
	if acc == nil {
		t.Fatalf("default account missing")
	}
	node := acc.Nodes[acc.ActiveID]
	node.HealthCheckMethod = proxy.HealthCheckMethodAPI
	node.Failed = true
	node.Metrics.FailStreak = 2

	srv.TestCheckNodeHealth(node.ID)

	if node.Failed {
		t.Fatalf("expected node recovered")
	}
	if node.Metrics.FailStreak != 0 {
		t.Fatalf("fail streak not reset")
	}
	if node.Metrics.LastPingErr != "" {
		t.Fatalf("expected no ping error, got %s", node.Metrics.LastPingErr)
	}
}

func TestHealthCheckHEAD(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiSrv.Close()

	srv := newProxyForTest(t, apiSrv.URL, nil)
	acc := srv.TestAccount(store.DefaultAccountID)
	if acc == nil {
		t.Fatalf("default account missing")
	}
	node := acc.Nodes[acc.ActiveID]
	node.HealthCheckMethod = proxy.HealthCheckMethodHEAD
	node.APIKey = "" // force HEAD path
	node.Failed = true

	srv.TestCheckNodeHealth(node.ID)

	if node.Failed {
		t.Fatalf("expected node recovered via HEAD")
	}
	if node.Metrics.LastPingErr != "" {
		t.Fatalf("expected empty ping error, got %s", node.Metrics.LastPingErr)
	}
}

func TestHealthCheckCLI(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	runner := func(ctx context.Context, image string, env map[string]string, prompt string) (string, error) {
		return "ok", nil
	}

	srv := newProxyForTest(t, apiSrv.URL, runner)
	acc := srv.TestAccount(store.DefaultAccountID)
	if acc == nil {
		t.Fatalf("default account missing")
	}
	node := acc.Nodes[acc.ActiveID]
	node.HealthCheckMethod = proxy.HealthCheckMethodCLI
	node.APIKey = "sk-cli"
	node.Failed = true

	srv.TestCheckNodeHealth(node.ID)

	if node.Failed {
		t.Fatalf("expected node recovered via CLI")
	}
	if node.Metrics.LastPingErr != "" {
		t.Fatalf("unexpected ping error: %s", node.Metrics.LastPingErr)
	}
}

func TestHealthCheckCLIFallbackToAPI(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/messages" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer apiSrv.Close()

	runner := func(ctx context.Context, image string, env map[string]string, prompt string) (string, error) {
		return "", exec.ErrNotFound
	}

	srv := newProxyForTest(t, apiSrv.URL, runner)
	acc := srv.TestAccount(store.DefaultAccountID)
	if acc == nil {
		t.Fatalf("default account missing")
	}
	node := acc.Nodes[acc.ActiveID]
	node.HealthCheckMethod = proxy.HealthCheckMethodCLI
	node.APIKey = "sk-cli"
	node.Failed = true

	srv.TestCheckNodeHealth(node.ID)

	if node.Failed {
		t.Fatalf("expected node recovered via API fallback")
	}
	if node.Metrics.LastPingErr != "" {
		t.Fatalf("expected ping error cleared, got %s", node.Metrics.LastPingErr)
	}
}
