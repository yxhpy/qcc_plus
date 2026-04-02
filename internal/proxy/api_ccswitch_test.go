package proxy

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"qcc_plus/internal/importer/ccswitch"
	"qcc_plus/internal/store"
)

func TestHandleCCSwitchImportAndExport(t *testing.T) {
	ctx := context.Background()
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

	adminAcc := testAdminAccount(t, srv)
	adminSess := srv.sessionMgr.Create(adminAcc.ID, true)

	t.Run("POST imports cc-switch db and refreshes runtime nodes", func(t *testing.T) {
		targetAcc, err := srv.createAccount("import-target", "proxy-import-target", "target123", false)
		if err != nil {
			t.Fatalf("create target account: %v", err)
		}

		ccswitchDB := buildCCSwitchFixtureFromStore(t, func(src *store.Store, accountID string) {
			node := store.NodeRecord{
				ID:                "source-openai-node",
				Name:              "source-openai-node",
				BaseURL:           "https://openai.example.com/v1",
				APIKey:            "sk-openai-import",
				HealthCheckMethod: HealthCheckMethodAPI,
				HealthCheckModel:  "gpt-5.4",
				SourceProtocol:    SourceProtocolOpenAI,
				AccountID:         accountID,
				Weight:            1,
				CreatedAt:         time.Now().Add(-2 * time.Hour).UTC(),
			}
			if err := src.UpsertNode(ctx, node); err != nil {
				t.Fatalf("upsert source node: %v", err)
			}
			if err := src.SetActive(ctx, accountID, node.ID); err != nil {
				t.Fatalf("set source active node: %v", err)
			}
			if err := src.UpsertModelPricing(ctx, store.ModelPricingRecord{
				ModelID:         "custom-import-model",
				ModelName:       "Custom Import Model",
				InputPriceMTok:  1.25,
				OutputPriceMTok: 6.5,
				IsActive:        true,
			}); err != nil {
				t.Fatalf("upsert source pricing: %v", err)
			}
			if err := src.InsertUsageLog(ctx, store.UsageLogRecord{
				AccountID:     accountID,
				NodeID:        node.ID,
				NodeName:      node.Name,
				ModelID:       "custom-import-model",
				InputTokens:   123,
				OutputTokens:  45,
				CostUSD:       0.12,
				RequestID:     "import-request-1",
				Success:       true,
				DurationMs:    321,
				TotalAttempts: 1,
				CreatedAt:     time.Now().Add(-30 * time.Minute).UTC(),
				Attempts: []store.UsageLogAttempt{
					{Seq: 1, NodeID: node.ID, NodeName: node.Name, StatusCode: 200, Success: true, DurationMs: 321, Action: "success"},
				},
			}); err != nil {
				t.Fatalf("insert source usage log: %v", err)
			}
		})

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		if err := writer.WriteField("account_id", targetAcc.ID); err != nil {
			t.Fatalf("write account_id: %v", err)
		}
		if err := writer.WriteField("import_providers", "true"); err != nil {
			t.Fatalf("write import_providers: %v", err)
		}
		if err := writer.WriteField("import_pricing", "true"); err != nil {
			t.Fatalf("write import_pricing: %v", err)
		}
		if err := writer.WriteField("import_logs", "true"); err != nil {
			t.Fatalf("write import_logs: %v", err)
		}
		part, err := writer.CreateFormFile("file", "cc-switch.db")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		srcFile, err := os.Open(ccswitchDB)
		if err != nil {
			t.Fatalf("open ccswitch fixture: %v", err)
		}
		if _, err := io.Copy(part, srcFile); err != nil {
			srcFile.Close()
			t.Fatalf("copy fixture into multipart body: %v", err)
		}
		srcFile.Close()
		if err := writer.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/api/cc-switch/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Summary ccswitch.Summary `json:"summary"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode import response: %v", err)
		}
		if resp.Summary.AccountID != targetAcc.ID {
			t.Fatalf("expected imported account %s, got %s", targetAcc.ID, resp.Summary.AccountID)
		}
		if resp.Summary.ProvidersImported != 1 {
			t.Fatalf("expected 1 imported provider, got %d", resp.Summary.ProvidersImported)
		}
		if resp.Summary.PricingImported < 1 {
			t.Fatalf("expected imported pricing rows, got %d", resp.Summary.PricingImported)
		}
		if resp.Summary.LogsImported != 1 {
			t.Fatalf("expected 1 imported log, got %d", resp.Summary.LogsImported)
		}

		importedAcc := srv.TestAccount(targetAcc.ID)
		if importedAcc == nil {
			t.Fatal("target account missing after import")
		}
		if len(importedAcc.Nodes) != 1 {
			t.Fatalf("expected 1 runtime node after import, got %d", len(importedAcc.Nodes))
		}
		if importedAcc.ActiveID == "" {
			t.Fatal("expected runtime active node after import")
		}
		importedNode := importedAcc.Nodes[importedAcc.ActiveID]
		if importedNode == nil {
			t.Fatalf("expected active node %s to exist", importedAcc.ActiveID)
		}
		if !strings.Contains(importedNode.Name, "source-openai-node") {
			t.Fatalf("unexpected imported node name: %s", importedNode.Name)
		}
		if importedNode.SourceProtocol != SourceProtocolOpenAI {
			t.Fatalf("unexpected imported protocol: %s", importedNode.SourceProtocol)
		}

		pricing, err := srv.store.GetModelPricing(ctx, "custom-import-model")
		if err != nil {
			t.Fatalf("get imported pricing: %v", err)
		}
		if pricing.InputPriceMTok != 1.25 || pricing.OutputPriceMTok != 6.5 {
			t.Fatalf("unexpected imported pricing: %+v", pricing)
		}

		logs, err := srv.store.QueryUsageLogs(ctx, store.QueryUsageParams{AccountID: targetAcc.ID, Limit: 10})
		if err != nil {
			t.Fatalf("query imported logs: %v", err)
		}
		if len(logs) != 1 {
			t.Fatalf("expected 1 imported usage log, got %d", len(logs))
		}
		if logs[0].RequestID != "import-request-1" {
			t.Fatalf("unexpected imported request id: %s", logs[0].RequestID)
		}
	})

	t.Run("GET exports account data as cc-switch sqlite", func(t *testing.T) {
		exportAcc, err := srv.createAccount("export-target", "proxy-export-target", "export123", false)
		if err != nil {
			t.Fatalf("create export account: %v", err)
		}

		node, err := srv.addNodeWithMethodAndKeys(exportAcc, "export-node", "https://claude.example.com", "", []NamedAPIKey{
			{Name: "primary", Key: "sk-export-primary"},
		}, 1, HealthCheckMethodAPI, "claude-haiku-4-5-20251001", map[string]string{
			"claude-haiku-4-5-20251001": "vendor-haiku",
		}, SourceProtocolClaude, "", "")
		if err != nil {
			t.Fatalf("add export node: %v", err)
		}
		if err := srv.store.UpsertModelPricing(ctx, store.ModelPricingRecord{
			ModelID:         "custom-export-model",
			ModelName:       "Custom Export Model",
			InputPriceMTok:  2.5,
			OutputPriceMTok: 9.5,
			IsActive:        true,
		}); err != nil {
			t.Fatalf("upsert export pricing: %v", err)
		}
		if err := srv.store.InsertUsageLog(ctx, store.UsageLogRecord{
			AccountID:     exportAcc.ID,
			NodeID:        node.ID,
			NodeName:      node.Name,
			ModelID:       "custom-export-model",
			InputTokens:   10,
			OutputTokens:  20,
			CostUSD:       0.01,
			RequestID:     "export-request-1",
			Success:       true,
			DurationMs:    111,
			TotalAttempts: 1,
			CreatedAt:     time.Now().Add(-10 * time.Minute).UTC(),
			Attempts: []store.UsageLogAttempt{
				{Seq: 1, NodeID: node.ID, NodeName: node.Name, StatusCode: 200, Success: true, DurationMs: 111, Action: "success"},
			},
		}); err != nil {
			t.Fatalf("insert export usage log: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/admin/api/cc-switch/export?account_id="+exportAcc.ID, nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: adminSess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); !strings.Contains(got, "sqlite") {
			t.Fatalf("expected sqlite content-type, got %s", got)
		}
		if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment;") {
			t.Fatalf("expected attachment content-disposition, got %s", got)
		}

		exportedPath := filepath.Join(t.TempDir(), "exported-cc-switch.db")
		if err := os.WriteFile(exportedPath, w.Body.Bytes(), 0644); err != nil {
			t.Fatalf("write exported sqlite file: %v", err)
		}

		db, err := sql.Open("sqlite", exportedPath)
		if err != nil {
			t.Fatalf("open exported sqlite: %v", err)
		}
		defer db.Close()

		var providerCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM providers`).Scan(&providerCount); err != nil {
			t.Fatalf("count exported providers: %v", err)
		}
		if providerCount != 1 {
			t.Fatalf("expected 1 exported provider, got %d", providerCount)
		}

		var pricingCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM model_pricing WHERE model_id = ?`, "custom-export-model").Scan(&pricingCount); err != nil {
			t.Fatalf("count exported pricing rows: %v", err)
		}
		if pricingCount != 1 {
			t.Fatalf("expected custom pricing row in export, got %d", pricingCount)
		}

		var logCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM proxy_request_logs WHERE request_id = ?`, "export-request-1").Scan(&logCount); err != nil {
			t.Fatalf("count exported logs: %v", err)
		}
		if logCount != 1 {
			t.Fatalf("expected exported log row, got %d", logCount)
		}
	})
}

func testAdminAccount(t *testing.T, srv *Server) *Account {
	t.Helper()

	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for _, acc := range srv.accountByID {
		if acc.IsAdmin {
			return acc
		}
	}
	t.Fatal("admin account not found")
	return nil
}

func buildCCSwitchFixtureFromStore(t *testing.T, seed func(src *store.Store, accountID string)) string {
	t.Helper()

	ctx := context.Background()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source-qcc.db")
	exportPath := filepath.Join(dir, "fixture-cc-switch.db")

	src, err := store.OpenSQLite(sourcePath)
	if err != nil {
		t.Fatalf("open source store: %v", err)
	}
	defer src.Close()

	const accountID = "fixture-admin"
	if err := src.CreateAccount(ctx, store.AccountRecord{
		ID:          accountID,
		Name:        "fixture-admin",
		Password:    "secret123",
		ProxyAPIKey: "fixture-proxy",
		IsAdmin:     true,
	}); err != nil {
		t.Fatalf("create source account: %v", err)
	}

	seed(src, accountID)

	if _, err := ccswitch.ExportFromStore(ctx, src, ccswitch.ExportOptions{
		AccountID:  accountID,
		OutputPath: exportPath,
		Overwrite:  true,
	}); err != nil {
		t.Fatalf("export source store to ccswitch db: %v", err)
	}
	return exportPath
}
