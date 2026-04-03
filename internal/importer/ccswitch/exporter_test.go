package ccswitch

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"qcc_plus/internal/store"
)

func TestExportRoundTripViaImporter(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	sourcePath := filepath.Join(dir, "source-qcc.db")
	exportPath := filepath.Join(dir, "exported-cc-switch.db")
	targetPath := filepath.Join(dir, "target-qcc.db")

	sourceStore, err := store.OpenSQLite(sourcePath)
	if err != nil {
		t.Fatalf("open source store: %v", err)
	}
	defer sourceStore.Close()

	const sourceAccountID = "source-admin"
	if err := sourceStore.CreateAccount(ctx, store.AccountRecord{
		ID:          sourceAccountID,
		Name:        "source-admin",
		Password:    "secret",
		ProxyAPIKey: "proxy-source",
		IsAdmin:     true,
	}); err != nil {
		t.Fatalf("create source account: %v", err)
	}

	openAINode := store.NodeRecord{
		ID:                "node-openai",
		Name:              "openai-node",
		BaseURL:           "https://openai.example.com/v1",
		APIKey:            "sk-openai-a,sk-openai-b",
		APIKeyConfig:      `[{"name":"primary","key":"sk-openai-a"},{"name":"backup","key":"sk-openai-b"}]`,
		HealthCheckMethod: "api",
		HealthCheckModel:  "gpt-5.4",
		ModelMapping:      `{"gpt-5.4":"gpt-5.4"}`,
		SourceProtocol:    "openai",
		AccountID:         sourceAccountID,
		Weight:            1,
		CreatedAt:         time.Now().Add(-3 * time.Hour).UTC(),
	}
	claudeNode := store.NodeRecord{
		ID:                "node-claude",
		Name:              "claude-node",
		BaseURL:           "https://claude.example.com",
		APIKey:            "sk-claude",
		HealthCheckMethod: "api",
		HealthCheckModel:  "claude-sonnet-4-5-20250929",
		ModelMapping:      `{"claude-sonnet-4-5-20250929":"claude-sonnet-4-5-20250929"}`,
		SourceProtocol:    "claude",
		AccountID:         sourceAccountID,
		Weight:            2,
		CreatedAt:         time.Now().Add(-2 * time.Hour).UTC(),
	}
	geminiNode := store.NodeRecord{
		ID:                "node-gemini",
		Name:              "gemini-node",
		BaseURL:           "https://gemini.example.com",
		APIKey:            "sk-gemini",
		HealthCheckMethod: "api",
		HealthCheckModel:  "gemini-2.5-flash",
		SourceProtocol:    "gemini",
		AccountID:         sourceAccountID,
		Weight:            3,
		CreatedAt:         time.Now().Add(-1 * time.Hour).UTC(),
	}

	for _, node := range []store.NodeRecord{openAINode, claudeNode, geminiNode} {
		if err := sourceStore.UpsertNode(ctx, node); err != nil {
			t.Fatalf("upsert source node %s: %v", node.ID, err)
		}
	}
	if err := sourceStore.SetActive(ctx, sourceAccountID, openAINode.ID); err != nil {
		t.Fatalf("set active source node: %v", err)
	}

	for _, pricing := range []store.ModelPricingRecord{
		{ModelID: "gpt-5.4", ModelName: "GPT-5.4", InputPriceMTok: 1.25, OutputPriceMTok: 10.5, IsActive: true},
		{ModelID: "claude-sonnet-4-5-20250929", ModelName: "Claude Sonnet 4.5", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
	} {
		if err := sourceStore.UpsertModelPricing(ctx, pricing); err != nil {
			t.Fatalf("upsert pricing %s: %v", pricing.ModelID, err)
		}
	}
	sourcePricing, err := sourceStore.ListModelPricing(ctx, false)
	if err != nil {
		t.Fatalf("list source pricing: %v", err)
	}
	expectedPricingCount := len(sourcePricing)

	sourceLogs := []store.UsageLogRecord{
		{
			AccountID:     sourceAccountID,
			NodeID:        openAINode.ID,
			NodeName:      "openai-node-primary",
			ModelID:       "gpt-5.4",
			InputTokens:   111,
			OutputTokens:  22,
			CostUSD:       0.123,
			RequestID:     "req-openai-primary",
			Success:       true,
			DurationMs:    321,
			TotalAttempts: 1,
			CreatedAt:     time.Now().Add(-45 * time.Minute).UTC(),
			Attempts: []store.UsageLogAttempt{
				{Seq: 1, NodeID: openAINode.ID, NodeName: "openai-node-primary", StatusCode: 200, Success: true, DurationMs: 321, Action: "success"},
			},
		},
		{
			AccountID:     sourceAccountID,
			NodeID:        openAINode.ID,
			NodeName:      "openai-node-backup",
			ModelID:       "gpt-5.4",
			InputTokens:   222,
			OutputTokens:  33,
			CostUSD:       0.456,
			RequestID:     "req-openai-backup",
			Success:       false,
			ErrorMsg:      "rate limited",
			DurationMs:    654,
			TotalAttempts: 2,
			CreatedAt:     time.Now().Add(-30 * time.Minute).UTC(),
			Attempts: []store.UsageLogAttempt{
				{Seq: 1, NodeID: openAINode.ID, NodeName: "openai-node-primary", StatusCode: 429, Success: false, DurationMs: 300, ErrorMsg: "rate limited", Severity: "transient", Action: "key_rotate"},
				{Seq: 2, NodeID: openAINode.ID, NodeName: "openai-node-backup", StatusCode: 429, Success: false, DurationMs: 654, ErrorMsg: "rate limited", Severity: "transient", Action: "fail"},
			},
		},
		{
			AccountID:     sourceAccountID,
			NodeID:        claudeNode.ID,
			NodeName:      claudeNode.Name,
			ModelID:       "claude-sonnet-4-5-20250929",
			InputTokens:   333,
			OutputTokens:  44,
			CostUSD:       0.789,
			RequestID:     "req-claude",
			Success:       true,
			DurationMs:    987,
			TotalAttempts: 1,
			CreatedAt:     time.Now().Add(-15 * time.Minute).UTC(),
			Attempts: []store.UsageLogAttempt{
				{Seq: 1, NodeID: claudeNode.ID, NodeName: claudeNode.Name, StatusCode: 200, Success: true, DurationMs: 987, Action: "success"},
			},
		},
		{
			AccountID:     sourceAccountID,
			NodeID:        geminiNode.ID,
			NodeName:      geminiNode.Name,
			ModelID:       "gemini-2.5-flash",
			InputTokens:   444,
			OutputTokens:  55,
			CostUSD:       0.111,
			RequestID:     "req-gemini",
			Success:       true,
			DurationMs:    1234,
			TotalAttempts: 1,
			CreatedAt:     time.Now().Add(-5 * time.Minute).UTC(),
			Attempts: []store.UsageLogAttempt{
				{Seq: 1, NodeID: geminiNode.ID, NodeName: geminiNode.Name, StatusCode: 200, Success: true, DurationMs: 1234, Action: "success"},
			},
		},
	}
	for _, item := range sourceLogs {
		if err := sourceStore.InsertUsageLog(ctx, item); err != nil {
			t.Fatalf("insert usage log %s: %v", item.RequestID, err)
		}
	}

	exportSummary, err := Export(ctx, ExportOptions{
		SourceSQLitePath: sourcePath,
		AccountID:        sourceAccountID,
		OutputPath:       exportPath,
		Overwrite:        true,
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if exportSummary.ProvidersExported != 4 {
		t.Fatalf("expected 4 exported providers, got %d", exportSummary.ProvidersExported)
	}
	if exportSummary.PricingExported != expectedPricingCount {
		t.Fatalf("expected %d exported pricing rows, got %d", expectedPricingCount, exportSummary.PricingExported)
	}
	if exportSummary.LogsExported != 4 {
		t.Fatalf("expected 4 exported logs, got %d", exportSummary.LogsExported)
	}

	exportDB, err := sql.Open("sqlite", exportPath)
	if err != nil {
		t.Fatalf("open export db: %v", err)
	}
	defer exportDB.Close()

	assertCount := func(table string, want int) {
		t.Helper()
		var got int
		if err := exportDB.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("expected %d rows in %s, got %d", want, table, got)
		}
	}
	assertCount("providers", 4)
	assertCount("provider_endpoints", 4)
	assertCount("provider_health", 4)
	assertCount("model_pricing", expectedPricingCount)
	assertCount("proxy_request_logs", 4)

	var providerName, providerAppType string
	if err := exportDB.QueryRow(`SELECT name, app_type FROM providers WHERE id = ?`, "node-openai-key-2").Scan(&providerName, &providerAppType); err != nil {
		t.Fatalf("query exported provider: %v", err)
	}
	if providerName != "openai-node-backup" {
		t.Fatalf("expected exported provider name openai-node-backup, got %s", providerName)
	}
	if providerAppType != "openai" {
		t.Fatalf("expected exported provider app type openai, got %s", providerAppType)
	}

	targetStore, err := store.OpenSQLite(targetPath)
	if err != nil {
		t.Fatalf("open target store: %v", err)
	}
	defer targetStore.Close()

	const targetAccountID = "target-admin"
	if err := targetStore.CreateAccount(ctx, store.AccountRecord{
		ID:          targetAccountID,
		Name:        "target-admin",
		Password:    "secret",
		ProxyAPIKey: "proxy-target",
		IsAdmin:     true,
	}); err != nil {
		t.Fatalf("create target account: %v", err)
	}

	importSummary, err := Run(ctx, Options{
		SourcePath:       exportPath,
		TargetSQLitePath: targetPath,
		AccountID:        targetAccountID,
		ImportProviders:  true,
		ImportPricing:    true,
		ImportLogs:       true,
	})
	if err != nil {
		t.Fatalf("re-import exported db failed: %v", err)
	}
	if importSummary.ProvidersImported != 4 {
		t.Fatalf("expected 4 imported providers, got %d", importSummary.ProvidersImported)
	}
	if importSummary.PricingImported != expectedPricingCount {
		t.Fatalf("expected %d imported pricing rows, got %d", expectedPricingCount, importSummary.PricingImported)
	}
	if importSummary.LogsImported != 4 {
		t.Fatalf("expected 4 imported logs, got %d", importSummary.LogsImported)
	}

	targetNodes, _, _, err := targetStore.LoadAllByAccount(ctx, targetAccountID)
	if err != nil {
		t.Fatalf("load imported nodes: %v", err)
	}
	if len(targetNodes) != 3 {
		t.Fatalf("expected 3 imported nodes after regrouping named keys, got %d", len(targetNodes))
	}

	var foundOpenAI bool
	for _, node := range targetNodes {
		if node.Name == "openai-node" {
			foundOpenAI = true
			if node.APIKey != "sk-openai-a,sk-openai-b" {
				t.Fatalf("expected merged openai api keys, got %s", node.APIKey)
			}
			if node.APIKeyConfig != `[{"name":"primary","key":"sk-openai-a"},{"name":"backup","key":"sk-openai-b"}]` {
				t.Fatalf("expected merged api key config, got %s", node.APIKeyConfig)
			}
			if node.BaseURL != openAINode.BaseURL {
				t.Fatalf("expected merged node base url %s, got %s", openAINode.BaseURL, node.BaseURL)
			}
		}
	}
	if !foundOpenAI {
		t.Fatalf("expected regrouped openai node")
	}

	importedLogs, err := targetStore.QueryUsageLogs(ctx, store.QueryUsageParams{AccountID: targetAccountID, Limit: 10})
	if err != nil {
		t.Fatalf("query imported logs: %v", err)
	}
	if len(importedLogs) != 4 {
		t.Fatalf("expected 4 imported logs, got %d", len(importedLogs))
	}
}

func TestExportAppTypeCodex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		protocol string
		want     string
	}{
		{"codex", "codex"},
		{"openai", "openai"},
		{"gemini", "gemini"},
		{"claude", "claude"},
		{"", "claude"},
	}

	for _, tc := range cases {
		if got := exportAppType(tc.protocol); got != tc.want {
			t.Fatalf("exportAppType(%q) = %q, want %q", tc.protocol, got, tc.want)
		}
	}
}
