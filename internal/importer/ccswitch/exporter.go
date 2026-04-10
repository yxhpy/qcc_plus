package ccswitch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"qcc_plus/internal/store"
)

const defaultExportPath = "cc-switch-export.db"

type ExportOptions struct {
	SourceMySQLDSN   string
	SourceSQLitePath string
	AccountID        string
	OutputPath       string
	Overwrite        bool
	Logger           *log.Logger
}

type ExportSummary struct {
	Source            string `json:"source"`
	AccountID         string `json:"account_id"`
	OutputPath        string `json:"output_path"`
	ProvidersExported int    `json:"providers_exported"`
	PricingExported   int    `json:"pricing_exported"`
	LogsExported      int    `json:"logs_exported"`
}

type exportNamedKey struct {
	Name string
	Key  string
}

type exportProvider struct {
	ID             string
	NodeID         string
	AppType        string
	Name           string
	BaseURL        string
	KeyName        string
	SettingsConfig string
	Meta           string
	SortIndex      int
	IsCurrent      bool
	Failed         bool
	FailStreak     int64
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func Export(ctx context.Context, opts ExportOptions) (ExportSummary, error) {
	opts = normalizeExportOptions(opts)
	sourceStore, sourceDesc, err := openExportSourceStore(opts)
	if err != nil {
		return ExportSummary{}, err
	}
	defer sourceStore.Close()
	return exportFromStore(ctx, sourceStore, sourceDesc, opts)
}

func ExportFromStore(ctx context.Context, sourceStore *store.Store, opts ExportOptions) (ExportSummary, error) {
	opts = normalizeExportOptions(opts)
	if sourceStore == nil {
		return ExportSummary{}, errors.New("source store is required")
	}
	return exportFromStore(ctx, sourceStore, "store", opts)
}

func exportFromStore(ctx context.Context, sourceStore *store.Store, sourceDesc string, opts ExportOptions) (ExportSummary, error) {
	summary := ExportSummary{
		Source: sourceDesc,
	}
	if sourceStore == nil {
		return summary, errors.New("source store is required")
	}

	accountID, err := resolveAccountID(ctx, sourceStore, opts.AccountID)
	if err != nil {
		return summary, err
	}
	summary.AccountID = accountID

	outputPath, err := resolveExportPath(opts.OutputPath)
	if err != nil {
		return summary, err
	}
	if err := prepareExportOutput(outputPath, opts.Overwrite); err != nil {
		return summary, err
	}
	summary.OutputPath = outputPath

	exportDB, err := sql.Open("sqlite", outputPath)
	if err != nil {
		return summary, err
	}
	defer exportDB.Close()
	if _, err := exportDB.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return summary, err
	}
	if err := createExportSchema(ctx, exportDB); err != nil {
		return summary, err
	}

	nodes, _, activeID, err := sourceStore.LoadAllByAccount(ctx, accountID)
	if err != nil {
		return summary, err
	}
	providers := buildExportProviders(nodes, activeID)

	tx, err := exportDB.BeginTx(ctx, nil)
	if err != nil {
		return summary, err
	}
	defer tx.Rollback()

	if err := insertExportProviders(ctx, tx, providers); err != nil {
		return summary, err
	}
	summary.ProvidersExported = len(providers)

	pricingRows, err := sourceStore.ListModelPricing(ctx, false)
	if err != nil {
		return summary, err
	}
	if err := insertExportPricing(ctx, tx, pricingRows); err != nil {
		return summary, err
	}
	summary.PricingExported = len(pricingRows)

	logs, err := loadExportUsageLogs(ctx, sourceStore, accountID)
	if err != nil {
		return summary, err
	}
	if err := insertExportLogs(ctx, tx, providers, logs); err != nil {
		return summary, err
	}
	summary.LogsExported = len(logs)

	if err := tx.Commit(); err != nil {
		return summary, err
	}
	return summary, nil
}

func normalizeExportOptions(opts ExportOptions) ExportOptions {
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	if opts.SourceMySQLDSN == "" {
		opts.SourceMySQLDSN = strings.TrimSpace(os.Getenv("PROXY_MYSQL_DSN"))
	}
	if opts.SourceSQLitePath == "" {
		opts.SourceSQLitePath = strings.TrimSpace(os.Getenv("PROXY_SQLITE_PATH"))
	}
	return opts
}

func openExportSourceStore(opts ExportOptions) (*store.Store, string, error) {
	if opts.SourceMySQLDSN != "" {
		st, err := store.OpenExisting(opts.SourceMySQLDSN)
		if err != nil {
			return nil, "", err
		}
		return st, "mysql", nil
	}

	sqlitePath := strings.TrimSpace(opts.SourceSQLitePath)
	if sqlitePath == "" {
		sqlitePath = defaultTargetSQLitePath()
	}
	resolved, err := resolvePath(sqlitePath, sqlitePath)
	if err != nil {
		return nil, "", err
	}
	st, err := store.OpenExistingSQLite(resolved)
	if err != nil {
		return nil, "", err
	}
	return st, resolved, nil
}

func resolveExportPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		path = defaultExportPath
	}
	if path == ":memory:" {
		return path, nil
	}
	return resolvePath(path, path)
}

func prepareExportOutput(path string, overwrite bool) error {
	if path == "" || path == ":memory:" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func createExportSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE providers (
			id TEXT NOT NULL,
			app_type TEXT NOT NULL,
			name TEXT NOT NULL,
			settings_config TEXT NOT NULL,
			website_url TEXT,
			category TEXT,
			created_at INTEGER,
			sort_index INTEGER,
			notes TEXT,
			icon TEXT,
			icon_color TEXT,
			meta TEXT NOT NULL DEFAULT '{}',
			is_current BOOLEAN NOT NULL DEFAULT 0,
			in_failover_queue BOOLEAN NOT NULL DEFAULT 0,
			cost_multiplier TEXT NOT NULL DEFAULT '1.0',
			limit_daily_usd TEXT,
			limit_monthly_usd TEXT,
			provider_type TEXT,
			PRIMARY KEY (id, app_type)
		)`,
		`CREATE INDEX idx_providers_failover ON providers(app_type, in_failover_queue, sort_index)`,
		`CREATE TABLE provider_endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id TEXT NOT NULL,
			app_type TEXT NOT NULL,
			url TEXT NOT NULL,
			added_at INTEGER,
			FOREIGN KEY (provider_id, app_type) REFERENCES providers(id, app_type) ON DELETE CASCADE
		)`,
		`CREATE TABLE provider_health (
			provider_id TEXT NOT NULL,
			app_type TEXT NOT NULL,
			is_healthy INTEGER NOT NULL DEFAULT 1,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			last_success_at TEXT,
			last_failure_at TEXT,
			last_error TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (provider_id, app_type),
			FOREIGN KEY (provider_id, app_type) REFERENCES providers(id, app_type) ON DELETE CASCADE
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
		`CREATE INDEX idx_request_logs_provider ON proxy_request_logs(provider_id, app_type)`,
		`CREATE INDEX idx_request_logs_created_at ON proxy_request_logs(created_at)`,
		`CREATE INDEX idx_request_logs_model ON proxy_request_logs(model)`,
		`CREATE INDEX idx_request_logs_session ON proxy_request_logs(session_id)`,
		`CREATE INDEX idx_request_logs_status ON proxy_request_logs(status_code)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func buildExportProviders(nodes []store.NodeRecord, activeID string) []exportProvider {
	providers := make([]exportProvider, 0)
	for _, node := range nodes {
		appType := exportAppType(node.SourceProtocol)
		if appType == "" {
			continue
		}
		baseURL := strings.TrimSpace(node.BaseURL)
		if baseURL == "" {
			continue
		}

		keys := decodeExportNodeKeys(node.APIKeyConfig, node.APIKey)
		if len(keys) == 0 {
			keys = []exportNamedKey{{}}
		}

		createdAt := node.CreatedAt.UTC()
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		updatedAt := node.LastHealthCheckAt.UTC()
		if updatedAt.IsZero() {
			updatedAt = createdAt
		}

		for idx, key := range keys {
			name := exportProviderDisplayName(node.Name, key.Name)
			if name == "" {
				name = node.Name
			}
			settingsConfig, err := buildExportSettings(node, baseURL, key.Key)
			if err != nil {
				continue
			}
			meta, err := buildExportMeta(node, key.Name)
			if err != nil {
				continue
			}
			sortIndex := node.Weight*100 + idx
			if sortIndex <= 0 {
				sortIndex = idx + 1
			}
			providers = append(providers, exportProvider{
				ID:             exportProviderID(node.ID, idx),
				NodeID:         node.ID,
				AppType:        appType,
				Name:           name,
				BaseURL:        baseURL,
				KeyName:        key.Name,
				SettingsConfig: settingsConfig,
				Meta:           meta,
				SortIndex:      sortIndex,
				IsCurrent:      node.ID == activeID && idx == 0,
				Failed:         node.Failed,
				FailStreak:     node.FailStreak,
				LastError:      node.LastError,
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			})
		}
	}

	sort.Slice(providers, func(i, j int) bool {
		if providers[i].SortIndex != providers[j].SortIndex {
			return providers[i].SortIndex < providers[j].SortIndex
		}
		if providers[i].CreatedAt.Equal(providers[j].CreatedAt) {
			return providers[i].ID < providers[j].ID
		}
		return providers[i].CreatedAt.Before(providers[j].CreatedAt)
	})
	return providers
}

func buildExportSettings(node store.NodeRecord, baseURL, apiKey string) (string, error) {
	protocol := strings.ToLower(strings.TrimSpace(node.SourceProtocol))
	modelMapping := decodeModelMappingJSON(node.ModelMapping)
	settings := map[string]any{}

	switch protocol {
	case "openai":
		wireAPI := strings.TrimSpace(node.WireAPI)
		switch strings.ToLower(wireAPI) {
		case "chat/completions", "chat-completions", "chat_completions":
			wireAPI = "chat_completions"
		case "responses":
			wireAPI = "responses"
		default:
			wireAPI = "responses"
		}
		auth := map[string]string{}
		if apiKey != "" {
			auth["OPENAI_API_KEY"] = apiKey
		}
		settings["auth"] = auth
		settings["config"] = fmt.Sprintf(
			"model_provider = \"custom\"\nmodel = %q\n\n[model_providers]\n[model_providers.custom]\nname = \"custom\"\nwire_api = %q\nrequires_openai_auth = true\nbase_url = %q\n",
			chooseNonEmpty(node.HealthCheckModel, defaultOpenAIHealthModel),
			wireAPI,
			baseURL,
		)
	case "gemini":
		env := map[string]string{
			"GOOGLE_GEMINI_BASE_URL": baseURL,
			"GEMINI_MODEL":           chooseNonEmpty(node.HealthCheckModel, defaultGeminiHealthModel),
		}
		if apiKey != "" {
			env["GEMINI_API_KEY"] = apiKey
		}
		settings["config"] = map[string]any{}
		settings["env"] = env
	default:
		healthModel := chooseNonEmpty(node.HealthCheckModel, defaultClaudeHealthModel)
		env := map[string]string{
			"ANTHROPIC_BASE_URL":             baseURL,
			"ANTHROPIC_MODEL":                healthModel,
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  chooseNonEmpty(modelMapping["claude-haiku-4-5-20251001"], healthModel),
			"ANTHROPIC_DEFAULT_SONNET_MODEL": chooseNonEmpty(modelMapping["claude-sonnet-4-5-20250929"], healthModel),
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   chooseNonEmpty(modelMapping["claude-opus-4-6"], healthModel),
		}
		if apiKey != "" {
			env["ANTHROPIC_AUTH_TOKEN"] = apiKey
		}
		settings["env"] = env
	}

	if models := buildExportModelsConfig(modelMapping); len(models) > 0 {
		settings["models"] = models
	}

	body, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func buildExportMeta(node store.NodeRecord, keyName string) (string, error) {
	apiFormat := "anthropic"
	switch strings.ToLower(strings.TrimSpace(node.SourceProtocol)) {
	case "openai":
		apiFormat = "openai"
	case "gemini":
		apiFormat = "gemini"
	}
	meta := map[string]string{
		"apiFormat":          apiFormat,
		"qcc_plus_node_id":   node.ID,
		"qcc_plus_node_name": node.Name,
	}
	if strings.TrimSpace(keyName) != "" {
		meta["qcc_plus_key_name"] = strings.TrimSpace(keyName)
	}
	body, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func buildExportModelsConfig(modelMapping map[string]string) map[string]any {
	if len(modelMapping) == 0 {
		return nil
	}
	keys := make([]string, 0, len(modelMapping))
	for key := range modelMapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	models := make(map[string]any, len(keys))
	for _, key := range keys {
		models[key] = map[string]any{"name": modelMapping[key]}
	}
	return models
}

func insertExportProviders(ctx context.Context, tx *sql.Tx, providers []exportProvider) error {
	providerStmt, err := tx.PrepareContext(ctx, `INSERT INTO providers
		(id, app_type, name, settings_config, created_at, sort_index, meta, is_current, in_failover_queue, cost_multiplier, provider_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer providerStmt.Close()

	endpointStmt, err := tx.PrepareContext(ctx, `INSERT INTO provider_endpoints (provider_id, app_type, url, added_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer endpointStmt.Close()

	healthStmt, err := tx.PrepareContext(ctx, `INSERT INTO provider_health
		(provider_id, app_type, is_healthy, consecutive_failures, last_success_at, last_failure_at, last_error, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer healthStmt.Close()

	for _, provider := range providers {
		createdAtMs := provider.CreatedAt.UTC().UnixMilli()
		if _, err := providerStmt.ExecContext(
			ctx,
			provider.ID,
			provider.AppType,
			provider.Name,
			provider.SettingsConfig,
			createdAtMs,
			provider.SortIndex,
			provider.Meta,
			provider.IsCurrent,
			false,
			"1.0",
			nil,
		); err != nil {
			return err
		}
		if _, err := endpointStmt.ExecContext(ctx, provider.ID, provider.AppType, provider.BaseURL, createdAtMs); err != nil {
			return err
		}

		updatedAt := provider.UpdatedAt.UTC()
		if updatedAt.IsZero() {
			updatedAt = time.Now().UTC()
		}
		var lastSuccessAt any
		var lastFailureAt any
		if provider.Failed {
			lastFailureAt = updatedAt.Format(time.RFC3339Nano)
		} else {
			lastSuccessAt = updatedAt.Format(time.RFC3339Nano)
		}
		if _, err := healthStmt.ExecContext(
			ctx,
			provider.ID,
			provider.AppType,
			!provider.Failed,
			provider.FailStreak,
			lastSuccessAt,
			lastFailureAt,
			nullIfEmpty(provider.LastError),
			updatedAt.Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}
	return nil
}

func insertExportPricing(ctx context.Context, tx *sql.Tx, pricingRows []store.ModelPricingRecord) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO model_pricing
		(model_id, display_name, input_cost_per_million, output_cost_per_million, cache_read_cost_per_million, cache_creation_cost_per_million)
		VALUES (?, ?, ?, ?, '0', '0')`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range pricingRows {
		if _, err := stmt.ExecContext(
			ctx,
			row.ModelID,
			chooseNonEmpty(row.ModelName, row.ModelID),
			formatDecimal(row.InputPriceMTok),
			formatDecimal(row.OutputPriceMTok),
		); err != nil {
			return err
		}
	}
	return nil
}

func loadExportUsageLogs(ctx context.Context, st *store.Store, accountID string) ([]store.UsageLogRecord, error) {
	const pageSize = 500
	total, err := st.CountUsageLogs(ctx, store.QueryUsageParams{AccountID: accountID})
	if err != nil {
		return nil, err
	}
	results := make([]store.UsageLogRecord, 0, total)
	for offset := 0; offset < int(total); offset += pageSize {
		page, err := st.QueryUsageLogs(ctx, store.QueryUsageParams{
			AccountID: accountID,
			Limit:     pageSize,
			Offset:    offset,
		})
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		logIDs := make([]int64, 0, len(page))
		for _, item := range page {
			logIDs = append(logIDs, item.ID)
		}
		attemptsByLogID, err := st.QueryAttemptsByLogIDs(ctx, logIDs)
		if err != nil {
			return nil, err
		}
		for i := range page {
			page[i].Attempts = attemptsByLogID[page[i].ID]
			results = append(results, page[i])
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].CreatedAt.Equal(results[j].CreatedAt) {
			return results[i].ID < results[j].ID
		}
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results, nil
}

func insertExportLogs(ctx context.Context, tx *sql.Tx, providers []exportProvider, logs []store.UsageLogRecord) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO proxy_request_logs
		(request_id, provider_id, app_type, model, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		 input_cost_usd, output_cost_usd, cache_read_cost_usd, cache_creation_cost_usd, total_cost_usd, latency_ms,
		 first_token_ms, duration_ms, status_code, error_message, session_id, provider_type, is_streaming, cost_multiplier, created_at, request_model)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, '0', '0', '0', '0', ?, ?, ?, ?, ?, ?, ?, ?, ?, '1.0', ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	providerIndex := indexExportProviders(providers)
	for _, item := range logs {
		provider, ok := resolveExportProvider(providerIndex, item)
		if !ok {
			continue
		}
		requestID := strings.TrimSpace(item.RequestID)
		if requestID == "" {
			requestID = fmt.Sprintf("qcc-export-%d", item.ID)
		}
		statusCode := inferExportStatusCode(item)
		durationMs := item.DurationMs
		if durationMs < 0 {
			durationMs = 0
		}
		if _, err := stmt.ExecContext(
			ctx,
			requestID,
			provider.ID,
			provider.AppType,
			item.ModelID,
			item.InputTokens,
			item.OutputTokens,
			formatDecimal(item.CostUSD),
			durationMs,
			nil,
			nullIfZero(durationMs),
			statusCode,
			nullIfEmpty(item.ErrorMsg),
			nil,
			nil,
			false,
			item.CreatedAt.UTC().Unix(),
			item.ModelID,
		); err != nil {
			return err
		}
	}
	return nil
}

func indexExportProviders(providers []exportProvider) map[string][]exportProvider {
	result := make(map[string][]exportProvider)
	for _, provider := range providers {
		result[provider.NodeID] = append(result[provider.NodeID], provider)
	}
	return result
}

func resolveExportProvider(providerIndex map[string][]exportProvider, logRecord store.UsageLogRecord) (exportProvider, bool) {
	candidates := providerIndex[logRecord.NodeID]
	if len(candidates) == 0 {
		return exportProvider{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	nodeName := strings.TrimSpace(logRecord.NodeName)
	if nodeName != "" {
		for _, candidate := range candidates {
			if candidate.Name == nodeName {
				return candidate, true
			}
		}
	}
	return candidates[0], true
}

func inferExportStatusCode(logRecord store.UsageLogRecord) int {
	if len(logRecord.Attempts) > 0 {
		last := logRecord.Attempts[len(logRecord.Attempts)-1]
		if last.StatusCode > 0 {
			return last.StatusCode
		}
	}
	if logRecord.Success {
		return 200
	}
	return 500
}

func exportAppType(sourceProtocol string) string {
	switch strings.ToLower(strings.TrimSpace(sourceProtocol)) {
	case "openai":
		return "codex"
	case "gemini":
		return "gemini"
	default:
		return "claude"
	}
}

func exportProviderID(nodeID string, keyIndex int) string {
	if keyIndex == 0 {
		return nodeID
	}
	return fmt.Sprintf("%s-key-%d", nodeID, keyIndex+1)
}

func exportProviderDisplayName(nodeName, keyName string) string {
	nodeName = strings.TrimSpace(nodeName)
	keyName = strings.TrimSpace(keyName)
	if keyName == "" {
		return nodeName
	}
	if nodeName == "" {
		return keyName
	}
	return nodeName + "-" + keyName
}

func decodeExportNodeKeys(configJSON, legacy string) []exportNamedKey {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON != "" {
		var direct []exportNamedKey
		if err := json.Unmarshal([]byte(configJSON), &direct); err == nil {
			return normalizeExportNodeKeys(direct)
		}
		var wrapped struct {
			Keys []exportNamedKey `json:"keys"`
		}
		if err := json.Unmarshal([]byte(configJSON), &wrapped); err == nil {
			return normalizeExportNodeKeys(wrapped.Keys)
		}
	}
	return normalizeExportNodeKeys(parseLegacyExportNodeKeys(legacy))
}

func normalizeExportNodeKeys(keys []exportNamedKey) []exportNamedKey {
	if len(keys) == 0 {
		return nil
	}
	result := make([]exportNamedKey, 0, len(keys))
	usedNames := make(map[string]int)
	for idx, item := range keys {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if len(keys) > 1 && name == "" {
			name = fmt.Sprintf("key%d", idx+1)
		}
		if name != "" {
			base := name
			lower := strings.ToLower(base)
			if usedNames[lower] > 0 {
				for suffix := usedNames[lower] + 1; ; suffix++ {
					candidate := fmt.Sprintf("%s-%d", base, suffix)
					if usedNames[strings.ToLower(candidate)] == 0 {
						name = candidate
						usedNames[lower] = suffix
						break
					}
				}
			}
			usedNames[strings.ToLower(name)]++
		}
		result = append(result, exportNamedKey{Name: name, Key: key})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func parseLegacyExportNodeKeys(raw string) []exportNamedKey {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]exportNamedKey, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		result = append(result, exportNamedKey{Key: key})
	}
	return result
}

func decodeModelMappingJSON(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func formatDecimal(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
