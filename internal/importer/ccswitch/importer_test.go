package ccswitch

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"qcc_plus/internal/store"
)

func TestRunImportsProvidersPricingAndLogsIdempotently(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "cc-switch.db")
	targetPath := filepath.Join(tmpDir, "qccplus.db")

	if err := createSourceDB(sourcePath); err != nil {
		t.Fatalf("create source db: %v", err)
	}

	targetStore, err := store.OpenSQLite(targetPath)
	if err != nil {
		t.Fatalf("open target store: %v", err)
	}
	ctx := context.Background()
	if err := targetStore.CreateAccount(ctx, store.AccountRecord{
		ID:          "admin-test",
		Name:        "admin",
		Password:    "admin123",
		ProxyAPIKey: "admin",
		IsAdmin:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("create target account: %v", err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatalf("close target store: %v", err)
	}

	summary, err := Run(ctx, Options{
		SourcePath:       sourcePath,
		TargetSQLitePath: targetPath,
		AccountID:        "admin-test",
		Logger:           log.New(os.Stdout, "[test] ", 0),
	})
	if err != nil {
		t.Fatalf("run importer: %v", err)
	}

	if summary.ProvidersImported != 4 {
		t.Fatalf("expected 4 imported providers, got %d", summary.ProvidersImported)
	}
	if summary.PricingImported != 1 {
		t.Fatalf("expected 1 imported pricing row, got %d", summary.PricingImported)
	}
	if summary.LogsImported != 2 {
		t.Fatalf("expected 2 imported logs, got %d", summary.LogsImported)
	}

	targetStore, err = store.OpenSQLite(targetPath)
	if err != nil {
		t.Fatalf("reopen target store: %v", err)
	}
	defer targetStore.Close()

	nodes, err := targetStore.GetNodesByAccount(ctx, "admin-test")
	if err != nil {
		t.Fatalf("get nodes: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}

	nodeByID := make(map[string]store.NodeRecord)
	for _, node := range nodes {
		nodeByID[node.ID] = node
	}

	openAINode := nodeByID["ccswitch-codex-p-openai"]
	if openAINode.SourceProtocol != "openai" {
		t.Fatalf("expected openai protocol, got %s", openAINode.SourceProtocol)
	}
	if openAINode.HealthCheckModel != "gpt-5.4" {
		t.Fatalf("expected openai health model gpt-5.4, got %s", openAINode.HealthCheckModel)
	}
	if openAINode.Requests != 1 || openAINode.FailCount != 0 {
		t.Fatalf("unexpected openai node stats: requests=%d fail_count=%d", openAINode.Requests, openAINode.FailCount)
	}

	opencodeNode := nodeByID["ccswitch-opencode-p-opencode"]
	if opencodeNode.SourceProtocol != "claude" {
		t.Fatalf("expected opencode provider to map to claude, got %s", opencodeNode.SourceProtocol)
	}
	if opencodeNode.HealthCheckModel != "claude-sonnet-4-5-vendor" {
		t.Fatalf("expected imported alias health model, got %s", opencodeNode.HealthCheckModel)
	}
	if opencodeNode.ModelMapping == "" {
		t.Fatal("expected model mapping to be imported")
	}

	pricing, err := targetStore.GetModelPricing(ctx, "gpt-5.4")
	if err != nil {
		t.Fatalf("get pricing: %v", err)
	}
	if pricing.InputPriceMTok != 2.5 || pricing.OutputPriceMTok != 10.0 {
		t.Fatalf("unexpected pricing values: %+v", pricing)
	}

	logs, err := targetStore.QueryUsageLogs(ctx, store.QueryUsageParams{AccountID: "admin-test", Limit: 10})
	if err != nil {
		t.Fatalf("query usage logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 usage logs, got %d", len(logs))
	}

	secondSummary, err := Run(ctx, Options{
		SourcePath:       sourcePath,
		TargetSQLitePath: targetPath,
		AccountID:        "admin-test",
	})
	if err != nil {
		t.Fatalf("run importer second time: %v", err)
	}
	if secondSummary.LogsSkippedDuplicate != 2 {
		t.Fatalf("expected 2 duplicate logs on second run, got %d", secondSummary.LogsSkippedDuplicate)
	}

	total, err := targetStore.CountUsageLogs(ctx, store.QueryUsageParams{AccountID: "admin-test"})
	if err != nil {
		t.Fatalf("count usage logs: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 usage logs after second run, got %d", total)
	}
}

func TestDetectProtocolPrefersAnthropicEnv(t *testing.T) {
	t.Parallel()

	settings := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "https://vendor.example.com/api",
		},
	}

	if got := detectProtocol("claude", settings, nil); got != "claude" {
		t.Fatalf("detectProtocol() = %s, want claude", got)
	}
}

func createSourceDB(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE providers (
			id TEXT PRIMARY KEY,
			app_type TEXT NOT NULL,
			name TEXT NOT NULL,
			settings_config TEXT NOT NULL,
			meta TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER,
			sort_index INTEGER,
			is_current BOOLEAN NOT NULL DEFAULT 0,
			in_failover_queue BOOLEAN NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE provider_endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id TEXT NOT NULL,
			app_type TEXT NOT NULL,
			url TEXT NOT NULL
		)`,
		`CREATE TABLE provider_health (
			provider_id TEXT NOT NULL,
			app_type TEXT NOT NULL,
			is_healthy INTEGER NOT NULL DEFAULT 1,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (provider_id, app_type)
		)`,
		`CREATE TABLE model_pricing (
			model_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			input_cost_per_million TEXT NOT NULL,
			output_cost_per_million TEXT NOT NULL,
			cache_read_cost_per_million TEXT NOT NULL DEFAULT '0',
			cache_creation_cost_per_million TEXT NOT NULL DEFAULT '0'
		)`,
		`CREATE TABLE proxy_request_logs (
			request_id TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			app_type TEXT NOT NULL,
			model TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			input_cost_usd TEXT NOT NULL DEFAULT '0',
			output_cost_usd TEXT NOT NULL DEFAULT '0',
			cache_read_cost_usd TEXT NOT NULL DEFAULT '0',
			cache_creation_cost_usd TEXT NOT NULL DEFAULT '0',
			total_cost_usd TEXT NOT NULL DEFAULT '0',
			latency_ms INTEGER NOT NULL,
			first_token_ms INTEGER,
			duration_ms INTEGER,
			status_code INTEGER NOT NULL,
			error_message TEXT,
			session_id TEXT,
			provider_type TEXT,
			is_streaming INTEGER NOT NULL DEFAULT 0,
			cost_multiplier TEXT NOT NULL DEFAULT '1.0',
			created_at INTEGER NOT NULL,
			request_model TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	providerInserts := []string{
		`INSERT INTO providers (id, app_type, name, settings_config, meta, created_at, sort_index, is_current, in_failover_queue) VALUES
		 ('p-claude', 'claude', 'Claude Vendor', '{"env":{"ANTHROPIC_AUTH_TOKEN":"sk-ant","ANTHROPIC_BASE_URL":"https://claude.example.com","ANTHROPIC_MODEL":"claude-sonnet-4-5-20250929"}}', '{"apiFormat":"anthropic"}', 1774000000000, 0, 1, 1)`,
		`INSERT INTO providers (id, app_type, name, settings_config, meta, created_at, sort_index, is_current, in_failover_queue) VALUES
		 ('p-openai', 'codex', 'OpenAI Vendor', '{"auth":{"OPENAI_API_KEY":"sk-openai"},"config":"model = \"gpt-5.4\"\nbase_url = \"https://openai.example.com/v1\"\n"}', '{}', 1774000001000, 1, 0, 1)`,
		`INSERT INTO providers (id, app_type, name, settings_config, meta, created_at, sort_index, is_current, in_failover_queue) VALUES
		 ('p-gemini', 'gemini', 'Gemini Vendor', '{"env":{"GEMINI_API_KEY":"sk-gemini","GEMINI_MODEL":"gemini-2.5-pro","GOOGLE_GEMINI_BASE_URL":"https://gemini.example.com"}}', '{}', 1774000002000, 2, 0, 1)`,
		`INSERT INTO providers (id, app_type, name, settings_config, meta, created_at, sort_index, is_current, in_failover_queue) VALUES
		 ('p-opencode', 'opencode', 'Opencode Vendor', '{"models":{"claude-sonnet-4-5":{"name":"claude-sonnet-4-5-vendor"}},"npm":"@ai-sdk/anthropic","options":{"apiKey":"sk-opencode","baseURL":"https://opencode.example.com/v1"}}', '{}', 1774000003000, 3, 0, 1)`,
	}
	for _, stmt := range providerInserts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	if _, err := db.Exec(`INSERT INTO provider_endpoints (provider_id, app_type, url) VALUES
		('p-claude', 'claude', 'https://claude.example.com'),
		('p-openai', 'codex', 'https://openai.example.com/v1'),
		('p-gemini', 'gemini', 'https://gemini.example.com')`); err != nil {
		return err
	}

	if _, err := db.Exec(`INSERT INTO provider_health (provider_id, app_type, is_healthy, consecutive_failures, last_error, updated_at) VALUES
		('p-claude', 'claude', 1, 0, '', '2026-03-31T11:00:00Z'),
		('p-openai', 'codex', 1, 0, '', '2026-03-31T11:01:00Z'),
		('p-gemini', 'gemini', 0, 3, 'quota exceeded', '2026-03-31T11:02:00Z'),
		('p-opencode', 'opencode', 1, 0, '', '2026-03-31T11:03:00Z')`); err != nil {
		return err
	}

	if _, err := db.Exec(`INSERT INTO model_pricing (model_id, display_name, input_cost_per_million, output_cost_per_million) VALUES
		('gpt-5.4', 'GPT-5.4', '2.5', '10.0')`); err != nil {
		return err
	}

	if _, err := db.Exec(`INSERT INTO proxy_request_logs (request_id, provider_id, app_type, model, input_tokens, output_tokens, total_cost_usd, latency_ms, first_token_ms, duration_ms, status_code, error_message, created_at, request_model) VALUES
		('req-openai-1', 'p-openai', 'codex', 'gpt-5.4', 100, 20, '0.012', 800, 120, 900, 200, '', 1774957000, 'gpt-5.4'),
		('req-opencode-1', 'p-opencode', 'opencode', 'claude-sonnet-4-5-vendor', 200, 30, '0.021', 1100, 200, 1300, 200, '', 1774957100, 'claude-sonnet-4-5')`); err != nil {
		return err
	}

	return nil
}
