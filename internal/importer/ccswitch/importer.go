package ccswitch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"qcc_plus/internal/store"
)

const (
	defaultSourcePath         = "~/.cc-switch/cc-switch.db"
	defaultTargetSQLiteDBName = "qccplus.db"
	defaultWeightOffset       = 1000
	defaultClaudeHealthModel  = "claude-haiku-4-5-20251001"
	defaultOpenAIHealthModel  = "gpt-5.1-mini"
	defaultGeminiHealthModel  = "gemini-2.5-flash"
)

var tomlStringPattern = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_]+)\s*=\s*"([^"]+)"\s*$`)

type Options struct {
	SourcePath       string
	TargetMySQLDSN   string
	TargetSQLitePath string
	AccountID        string
	WeightOffset     int
	DryRun           bool
	ImportProviders  bool
	ImportPricing    bool
	ImportLogs       bool
	Logger           *log.Logger
}

type Summary struct {
	SourcePath            string `json:"source_path"`
	Target                string `json:"target"`
	AccountID             string `json:"account_id"`
	ProvidersRead         int    `json:"providers_read"`
	ProvidersImported     int    `json:"providers_imported"`
	ProvidersSkipped      int    `json:"providers_skipped"`
	PricingRowsRead       int    `json:"pricing_rows_read"`
	PricingImported       int    `json:"pricing_imported"`
	LogRowsRead           int    `json:"log_rows_read"`
	LogsImported          int    `json:"logs_imported"`
	LogsSkippedDuplicate  int    `json:"logs_skipped_duplicate"`
	LogsSkippedNoProvider int    `json:"logs_skipped_no_provider"`
}

type sourceProvider struct {
	ID              string
	AppType         string
	Name            string
	SettingsConfig  string
	Meta            string
	SortIndex       sql.NullInt64
	IsCurrent       bool
	InFailoverQueue bool
	CreatedAt       sql.NullInt64
	Endpoints       []string
	Health          sourceProviderHealth
	Stats           sourceProviderStats
}

type sourceProviderHealth struct {
	Valid               bool
	IsHealthy           bool
	ConsecutiveFailures int64
	LastError           string
	UpdatedAt           time.Time
}

type sourceProviderStats struct {
	Requests        int64
	SuccessRequests int64
	InputTokens     int64
	OutputTokens    int64
	DurationSumMs   int64
	FirstTokenSumMs int64
	FirstRequestAt  time.Time
	LastRequestAt   time.Time
}

type sourcePricing struct {
	ModelID     string
	DisplayName string
	InputCost   string
	OutputCost  string
}

type sourceLog struct {
	RequestID    string
	ProviderID   string
	AppType      string
	Model        string
	RequestModel sql.NullString
	InputTokens  int64
	OutputTokens int64
	TotalCostUSD string
	LatencyMs    int64
	DurationMs   sql.NullInt64
	StatusCode   int
	ErrorMessage sql.NullString
	CreatedAt    int64
}

type importedNode struct {
	Record store.NodeRecord
	Name   string
}

type importedNodeGroup struct {
	Record   store.NodeRecord
	Name     string
	KeyItems []namedImportKey
}

type namedImportKey struct {
	Name string
	Key  string
}

type roundTripMeta struct {
	NodeID   string
	NodeName string
	KeyName  string
}

func Run(ctx context.Context, opts Options) (Summary, error) {
	opts = normalizeOptions(opts)
	sourcePath, err := resolvePath(opts.SourcePath, defaultSourcePath)
	if err != nil {
		return Summary{}, err
	}

	sourceDB, err := openSource(sourcePath)
	if err != nil {
		return Summary{}, err
	}
	defer sourceDB.Close()

	targetStore, targetDesc, err := openTargetStore(opts)
	if err != nil {
		return Summary{}, err
	}
	defer targetStore.Close()
	return runImport(ctx, sourcePath, sourceDB, targetStore, targetDesc, opts)
}

func RunWithTargetStore(ctx context.Context, targetStore *store.Store, opts Options) (Summary, error) {
	opts = normalizeOptions(opts)
	if targetStore == nil {
		return Summary{}, errors.New("target store is required")
	}

	sourcePath, err := resolvePath(opts.SourcePath, defaultSourcePath)
	if err != nil {
		return Summary{}, err
	}

	sourceDB, err := openSource(sourcePath)
	if err != nil {
		return Summary{}, err
	}
	defer sourceDB.Close()

	return runImport(ctx, sourcePath, sourceDB, targetStore, "store", opts)
}

func runImport(ctx context.Context, sourcePath string, sourceDB *sql.DB, targetStore *store.Store, targetDesc string, opts Options) (Summary, error) {
	summary := Summary{
		SourcePath: sourcePath,
		Target:     targetDesc,
	}
	if targetStore == nil {
		return summary, errors.New("target store is required")
	}

	accountID, err := resolveAccountID(ctx, targetStore, opts.AccountID)
	if err != nil {
		return summary, err
	}
	summary.AccountID = accountID

	providers, err := loadProviders(ctx, sourceDB)
	if err != nil {
		return summary, err
	}
	summary.ProvidersRead = len(providers)

	importedNodes := make(map[string]importedNode, len(providers))
	groupAliases := make(map[string]string, len(providers))
	nodeGroups := make(map[string]*importedNodeGroup, len(providers))
	for i, provider := range providers {
		record, ok, err := buildNodeRecord(provider, accountID, i, opts.WeightOffset)
		if err != nil {
			return summary, err
		}
		if !ok {
			summary.ProvidersSkipped++
			continue
		}

		alias := providerKey(provider.AppType, provider.ID)
		meta := parseRoundTripMeta(provider.Meta)
		groupKey := alias
		if meta.NodeID != "" {
			groupKey = provider.AppType + "|group|" + meta.NodeID
			record.ID = importedNodeID(provider.AppType, meta.NodeID)
			record.Name = chooseNonEmpty(meta.NodeName, record.Name)
		}

		group := nodeGroups[groupKey]
		if group == nil {
			group = &importedNodeGroup{
				Record: record,
				Name:   record.Name,
			}
			nodeGroups[groupKey] = group
		} else {
			mergeImportedNodeRecord(&group.Record, record)
			group.Name = group.Record.Name
		}
		if meta.NodeID != "" {
			group.KeyItems = appendNamedImportKey(group.KeyItems, namedImportKey{
				Name: strings.TrimSpace(meta.KeyName),
				Key:  extractAPIKey(parseJSONObject(provider.SettingsConfig)),
			})
		}
		groupAliases[alias] = groupKey
	}

	for _, group := range nodeGroups {
		finalizeImportedNodeGroup(group)
	}

	if opts.ImportProviders {
		for _, provider := range providers {
			alias := providerKey(provider.AppType, provider.ID)
			groupKey, ok := groupAliases[alias]
			if !ok {
				continue
			}
			group := nodeGroups[groupKey]
			if group == nil {
				continue
			}
			if !opts.DryRun {
				if err := targetStore.UpsertNode(ctx, group.Record); err != nil {
					return summary, fmt.Errorf("import provider %s/%s failed: %w", provider.AppType, provider.Name, err)
				}
			}
			summary.ProvidersImported++
		}
	}
	for _, provider := range providers {
		alias := providerKey(provider.AppType, provider.ID)
		groupKey, ok := groupAliases[alias]
		if !ok {
			continue
		}
		group := nodeGroups[groupKey]
		if group == nil {
			continue
		}
		importedNodes[alias] = importedNode{
			Record: group.Record,
			Name:   chooseNonEmpty(provider.Name, group.Record.Name),
		}
	}

	if opts.ImportPricing {
		pricingRows, err := loadPricing(ctx, sourceDB)
		if err != nil {
			return summary, err
		}
		summary.PricingRowsRead = len(pricingRows)
		for _, row := range pricingRows {
			inputCost, err := parseDecimal(row.InputCost)
			if err != nil {
				return summary, fmt.Errorf("parse input cost for %s failed: %w", row.ModelID, err)
			}
			outputCost, err := parseDecimal(row.OutputCost)
			if err != nil {
				return summary, fmt.Errorf("parse output cost for %s failed: %w", row.ModelID, err)
			}
			rec := store.ModelPricingRecord{
				ModelID:         row.ModelID,
				ModelName:       chooseNonEmpty(row.DisplayName, row.ModelID),
				InputPriceMTok:  inputCost,
				OutputPriceMTok: outputCost,
				IsActive:        true,
			}
			if !opts.DryRun {
				if err := targetStore.UpsertModelPricing(ctx, rec); err != nil {
					return summary, fmt.Errorf("import pricing %s failed: %w", row.ModelID, err)
				}
			}
			summary.PricingImported++
		}
	}

	if opts.ImportLogs {
		existingRequestIDs, err := targetStore.ListUsageRequestIDs(ctx)
		if err != nil {
			return summary, fmt.Errorf("load existing request IDs failed: %w", err)
		}
		logRows, err := loadLogs(ctx, sourceDB)
		if err != nil {
			return summary, err
		}
		summary.LogRowsRead = len(logRows)
		orphanNodes := buildOrphanNodes(logRows, importedNodes, accountID, opts.WeightOffset)
		for key, node := range orphanNodes {
			importedNodes[key] = node
			if opts.ImportProviders && !opts.DryRun {
				if err := targetStore.UpsertNode(ctx, node.Record); err != nil {
					return summary, fmt.Errorf("import orphan log node %s failed: %w", node.Record.ID, err)
				}
			}
		}
		for _, row := range logRows {
			if row.RequestID != "" {
				if _, exists := existingRequestIDs[row.RequestID]; exists {
					summary.LogsSkippedDuplicate++
					continue
				}
			}
			node, ok := importedNodes[providerKey(row.AppType, row.ProviderID)]
			if !ok {
				summary.LogsSkippedNoProvider++
				continue
			}
			logRecord, err := buildUsageLog(row, accountID, node)
			if err != nil {
				return summary, err
			}
			if !opts.DryRun {
				if err := targetStore.InsertUsageLog(ctx, logRecord); err != nil {
					return summary, fmt.Errorf("import usage log %s failed: %w", row.RequestID, err)
				}
				if row.RequestID != "" {
					existingRequestIDs[row.RequestID] = struct{}{}
				}
			}
			summary.LogsImported++
		}
	}
	return summary, nil
}

func normalizeOptions(opts Options) Options {
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	if opts.WeightOffset == 0 {
		opts.WeightOffset = defaultWeightOffset
	}
	if !opts.ImportProviders && !opts.ImportPricing && !opts.ImportLogs {
		opts.ImportProviders = true
		opts.ImportPricing = true
		opts.ImportLogs = true
	}
	if opts.TargetMySQLDSN == "" {
		opts.TargetMySQLDSN = strings.TrimSpace(os.Getenv("PROXY_MYSQL_DSN"))
	}
	if opts.TargetSQLitePath == "" {
		opts.TargetSQLitePath = strings.TrimSpace(os.Getenv("PROXY_SQLITE_PATH"))
	}
	return opts
}

func resolvePath(raw, fallback string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		path = fallback
	}
	if path == "" {
		return "", errors.New("path required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Clean(path), nil
}

func openSource(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dsn = "file:" + filepath.ToSlash(path) + "?mode=ro"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func openTargetStore(opts Options) (*store.Store, string, error) {
	if opts.TargetMySQLDSN != "" {
		st, err := store.OpenExisting(opts.TargetMySQLDSN)
		if err != nil {
			return nil, "", err
		}
		return st, "mysql", nil
	}

	sqlitePath := strings.TrimSpace(opts.TargetSQLitePath)
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

func defaultTargetSQLitePath() string {
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".qccplus", defaultTargetSQLiteDBName)
	}
	return defaultTargetSQLiteDBName
}

func resolveAccountID(ctx context.Context, st *store.Store, requested string) (string, error) {
	if requested != "" {
		acc, err := st.GetAccountByID(ctx, requested)
		if err != nil {
			return "", err
		}
		return acc.ID, nil
	}

	accounts, err := st.ListAccounts(ctx)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "", errors.New("no account found in target store; create an account first or pass --account-id")
	}
	for _, acc := range accounts {
		if acc.IsAdmin {
			return acc.ID, nil
		}
	}
	if len(accounts) == 1 {
		return accounts[0].ID, nil
	}
	return "", errors.New("multiple non-admin accounts found; pass --account-id explicitly")
}

func loadProviders(ctx context.Context, db *sql.DB) ([]sourceProvider, error) {
	endpoints, err := loadEndpoints(ctx, db)
	if err != nil {
		return nil, err
	}
	healthMap, err := loadProviderHealth(ctx, db)
	if err != nil {
		return nil, err
	}
	statsMap, err := loadProviderStats(ctx, db)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `SELECT id, app_type, name, settings_config, meta, sort_index, is_current, in_failover_queue, created_at
		FROM providers
		ORDER BY app_type ASC, COALESCE(sort_index, 2147483647) ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []sourceProvider
	for rows.Next() {
		var (
			provider        sourceProvider
			isCurrent       int64
			inFailoverQueue int64
		)
		if err := rows.Scan(&provider.ID, &provider.AppType, &provider.Name, &provider.SettingsConfig, &provider.Meta, &provider.SortIndex, &isCurrent, &inFailoverQueue, &provider.CreatedAt); err != nil {
			return nil, err
		}
		provider.IsCurrent = isCurrent != 0
		provider.InFailoverQueue = inFailoverQueue != 0
		provider.Endpoints = endpoints[providerKey(provider.AppType, provider.ID)]
		provider.Health = healthMap[providerKey(provider.AppType, provider.ID)]
		provider.Stats = statsMap[providerKey(provider.AppType, provider.ID)]
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func loadEndpoints(ctx context.Context, db *sql.DB) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT provider_id, app_type, url FROM provider_endpoints ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var providerID, appType, endpointURL string
		if err := rows.Scan(&providerID, &appType, &endpointURL); err != nil {
			return nil, err
		}
		key := providerKey(appType, providerID)
		result[key] = append(result[key], strings.TrimSpace(endpointURL))
	}
	return result, rows.Err()
}

func loadProviderHealth(ctx context.Context, db *sql.DB) (map[string]sourceProviderHealth, error) {
	rows, err := db.QueryContext(ctx, `SELECT provider_id, app_type, is_healthy, consecutive_failures, last_error, updated_at FROM provider_health`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]sourceProviderHealth)
	for rows.Next() {
		var (
			providerID          string
			appType             string
			isHealthy           int64
			consecutiveFailures int64
			lastError           sql.NullString
			updatedAt           sql.NullString
		)
		if err := rows.Scan(&providerID, &appType, &isHealthy, &consecutiveFailures, &lastError, &updatedAt); err != nil {
			return nil, err
		}
		health := sourceProviderHealth{
			Valid:               true,
			IsHealthy:           isHealthy != 0,
			ConsecutiveFailures: consecutiveFailures,
			LastError:           lastError.String,
		}
		if updatedAt.Valid {
			if ts, err := time.Parse(time.RFC3339Nano, updatedAt.String); err == nil {
				health.UpdatedAt = ts.UTC()
			}
		}
		result[providerKey(appType, providerID)] = health
	}
	return result, rows.Err()
}

func loadProviderStats(ctx context.Context, db *sql.DB) (map[string]sourceProviderStats, error) {
	rows, err := db.QueryContext(ctx, `SELECT provider_id, app_type,
			COUNT(*) AS requests_total,
			COALESCE(SUM(CASE WHEN status_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END), 0) AS requests_success,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(COALESCE(duration_ms, latency_ms, 0)), 0) AS duration_sum_ms,
			COALESCE(SUM(COALESCE(first_token_ms, 0)), 0) AS first_token_sum_ms,
			MIN(created_at) AS first_request_at,
			MAX(created_at) AS last_request_at
		FROM proxy_request_logs
		GROUP BY provider_id, app_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]sourceProviderStats)
	for rows.Next() {
		var (
			providerID     string
			appType        string
			stats          sourceProviderStats
			firstRequestAt sql.NullInt64
			lastRequestAt  sql.NullInt64
		)
		if err := rows.Scan(&providerID, &appType, &stats.Requests, &stats.SuccessRequests, &stats.InputTokens, &stats.OutputTokens, &stats.DurationSumMs, &stats.FirstTokenSumMs, &firstRequestAt, &lastRequestAt); err != nil {
			return nil, err
		}
		if firstRequestAt.Valid {
			stats.FirstRequestAt = time.Unix(firstRequestAt.Int64, 0).UTC()
		}
		if lastRequestAt.Valid {
			stats.LastRequestAt = time.Unix(lastRequestAt.Int64, 0).UTC()
		}
		result[providerKey(appType, providerID)] = stats
	}
	return result, rows.Err()
}

func buildNodeRecord(provider sourceProvider, accountID string, order int, weightOffset int) (store.NodeRecord, bool, error) {
	settings := parseJSONObject(provider.SettingsConfig)
	meta := parseJSONObject(provider.Meta)

	protocol := detectProtocol(provider.AppType, settings, meta)
	if protocol == "" {
		return store.NodeRecord{}, false, nil
	}

	baseURL := extractBaseURL(provider.Endpoints, settings)
	if baseURL == "" {
		return store.NodeRecord{}, false, nil
	}
	if _, err := urlParse(baseURL); err != nil {
		return store.NodeRecord{}, false, fmt.Errorf("invalid base URL for provider %s/%s: %w", provider.AppType, provider.Name, err)
	}

	modelMapping := extractModelMapping(protocol, settings)
	modelMappingJSON := ""
	if len(modelMapping) > 0 {
		body, err := json.Marshal(modelMapping)
		if err != nil {
			return store.NodeRecord{}, false, err
		}
		modelMappingJSON = string(body)
	}

	createdAt := providerCreatedAt(provider)
	healthMethod := "api"
	apiKey := extractAPIKey(settings)
	if apiKey == "" {
		healthMethod = "head"
	}

	requests := provider.Stats.Requests
	failCount := requests - provider.Stats.SuccessRequests
	if failCount < 0 {
		failCount = 0
	}

	record := store.NodeRecord{
		ID:                importedNodeID(provider.AppType, provider.ID),
		Name:              importedNodeName(provider.AppType, provider.Name),
		BaseURL:           baseURL,
		APIKey:            apiKey,
		HealthCheckMethod: healthMethod,
		HealthCheckModel:  extractHealthModel(protocol, settings, modelMapping),
		ModelMapping:      modelMappingJSON,
		SourceProtocol:    protocol,
		AccountID:         accountID,
		Weight:            providerWeight(provider, order, weightOffset),
		Failed:            provider.Health.Valid && !provider.Health.IsHealthy,
		LastError:         provider.Health.LastError,
		CreatedAt:         createdAt,
		Requests:          requests,
		FailCount:         failCount,
		FailStreak:        provider.Health.ConsecutiveFailures,
		TotalInput:        provider.Stats.InputTokens,
		TotalOutput:       provider.Stats.OutputTokens,
		StreamDurMs:       provider.Stats.DurationSumMs,
		FirstByteMs:       provider.Stats.FirstTokenSumMs,
		LastPingErr:       provider.Health.LastError,
		LastHealthCheckAt: provider.Health.UpdatedAt,
	}
	if record.HealthCheckModel == "" {
		record.HealthCheckModel = defaultHealthModelForProtocol(protocol)
	}
	return record, true, nil
}

func parseRoundTripMeta(raw string) roundTripMeta {
	meta := parseJSONObject(raw)
	if len(meta) == 0 {
		return roundTripMeta{}
	}
	return roundTripMeta{
		NodeID:   strings.TrimSpace(getString(meta, "qcc_plus_node_id")),
		NodeName: strings.TrimSpace(getString(meta, "qcc_plus_node_name")),
		KeyName:  strings.TrimSpace(getString(meta, "qcc_plus_key_name")),
	}
}

func appendNamedImportKey(keys []namedImportKey, item namedImportKey) []namedImportKey {
	item.Key = strings.TrimSpace(item.Key)
	item.Name = strings.TrimSpace(item.Name)
	if item.Key == "" {
		return keys
	}
	for _, existing := range keys {
		if existing.Key == item.Key && existing.Name == item.Name {
			return keys
		}
	}
	return append(keys, item)
}

func finalizeImportedNodeGroup(group *importedNodeGroup) {
	if group == nil {
		return
	}
	if len(group.KeyItems) == 0 {
		return
	}
	keys := make([]struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	}, 0, len(group.KeyItems))
	joined := make([]string, 0, len(group.KeyItems))
	for idx, item := range group.KeyItems {
		name := item.Name
		if name == "" && len(group.KeyItems) > 1 {
			name = fmt.Sprintf("key%d", idx+1)
		}
		keys = append(keys, struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		}{Name: name, Key: item.Key})
		joined = append(joined, item.Key)
	}
	group.Record.APIKey = strings.Join(joined, ",")
	if len(keys) > 1 || strings.TrimSpace(keys[0].Name) != "" {
		if body, err := json.Marshal(keys); err == nil {
			group.Record.APIKeyConfig = string(body)
		}
	}
	if group.Record.APIKey != "" {
		group.Record.HealthCheckMethod = "api"
	}
}

func mergeImportedNodeRecord(dst *store.NodeRecord, src store.NodeRecord) {
	if dst == nil {
		return
	}
	if dst.BaseURL == "" {
		dst.BaseURL = src.BaseURL
	}
	if dst.HealthCheckModel == "" {
		dst.HealthCheckModel = src.HealthCheckModel
	}
	if dst.ModelMapping == "" {
		dst.ModelMapping = src.ModelMapping
	}
	if dst.SourceProtocol == "" {
		dst.SourceProtocol = src.SourceProtocol
	}
	if dst.AuthProfile == "" {
		dst.AuthProfile = src.AuthProfile
	}
	if dst.Capabilities == "" {
		dst.Capabilities = src.Capabilities
	}
	if dst.Name == "" {
		dst.Name = src.Name
	}
	if dst.Weight == 0 || (src.Weight > 0 && src.Weight < dst.Weight) {
		dst.Weight = src.Weight
	}
	if dst.CreatedAt.IsZero() || (!src.CreatedAt.IsZero() && src.CreatedAt.Before(dst.CreatedAt)) {
		dst.CreatedAt = src.CreatedAt
	}
	dst.Requests += src.Requests
	dst.FailCount += src.FailCount
	if src.FailStreak > dst.FailStreak {
		dst.FailStreak = src.FailStreak
	}
	dst.TotalBytes += src.TotalBytes
	dst.TotalInput += src.TotalInput
	dst.TotalOutput += src.TotalOutput
	dst.StreamDurMs += src.StreamDurMs
	dst.FirstByteMs += src.FirstByteMs
	if src.LastPingMs > dst.LastPingMs {
		dst.LastPingMs = src.LastPingMs
	}
	if src.LastHealthCheckAt.After(dst.LastHealthCheckAt) {
		dst.LastHealthCheckAt = src.LastHealthCheckAt
		dst.LastPingErr = src.LastPingErr
		if src.LastError != "" {
			dst.LastError = src.LastError
		}
		dst.Failed = src.Failed
	} else {
		dst.Failed = dst.Failed || src.Failed
		if dst.LastError == "" {
			dst.LastError = src.LastError
		}
	}
	dst.Disabled = dst.Disabled || src.Disabled
	if dst.APIKey == "" {
		dst.APIKey = src.APIKey
	}
	if dst.APIKeyConfig == "" {
		dst.APIKeyConfig = src.APIKeyConfig
	}
	if dst.HealthCheckMethod == "" || (dst.HealthCheckMethod == "head" && src.HealthCheckMethod == "api") {
		dst.HealthCheckMethod = src.HealthCheckMethod
	}
}

func providerCreatedAt(provider sourceProvider) time.Time {
	if provider.CreatedAt.Valid && provider.CreatedAt.Int64 > 0 {
		return millisToTime(provider.CreatedAt.Int64)
	}
	if !provider.Stats.FirstRequestAt.IsZero() {
		return provider.Stats.FirstRequestAt
	}
	return time.Now().UTC()
}

func providerWeight(provider sourceProvider, order int, weightOffset int) int {
	base := weightOffset + order + 1
	if provider.SortIndex.Valid && provider.SortIndex.Int64 >= 0 {
		base = weightOffset + int(provider.SortIndex.Int64) + 1
	}
	if provider.IsCurrent {
		base = weightOffset
	}
	return base
}

func buildUsageLog(row sourceLog, accountID string, node importedNode) (store.UsageLogRecord, error) {
	costUSD, err := parseDecimal(row.TotalCostUSD)
	if err != nil {
		return store.UsageLogRecord{}, fmt.Errorf("parse total_cost_usd for request %s failed: %w", row.RequestID, err)
	}
	modelID := strings.TrimSpace(row.RequestModel.String)
	if modelID == "" {
		modelID = strings.TrimSpace(row.Model)
	}
	durationMs := row.LatencyMs
	if row.DurationMs.Valid {
		durationMs = row.DurationMs.Int64
	}
	return store.UsageLogRecord{
		AccountID:     accountID,
		NodeID:        node.Record.ID,
		NodeName:      node.Name,
		ModelID:       modelID,
		InputTokens:   row.InputTokens,
		OutputTokens:  row.OutputTokens,
		CostUSD:       costUSD,
		RequestID:     row.RequestID,
		Success:       row.StatusCode >= 200 && row.StatusCode < 300,
		ErrorMsg:      row.ErrorMessage.String,
		DurationMs:    durationMs,
		TotalAttempts: 1,
		CreatedAt:     time.Unix(row.CreatedAt, 0).UTC(),
	}, nil
}

func loadPricing(ctx context.Context, db *sql.DB) ([]sourcePricing, error) {
	rows, err := db.QueryContext(ctx, `SELECT model_id, display_name, input_cost_per_million, output_cost_per_million FROM model_pricing ORDER BY model_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []sourcePricing
	for rows.Next() {
		var item sourcePricing
		if err := rows.Scan(&item.ModelID, &item.DisplayName, &item.InputCost, &item.OutputCost); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func buildOrphanNodes(logRows []sourceLog, imported map[string]importedNode, accountID string, weightOffset int) map[string]importedNode {
	type aggregate struct {
		AppType        string
		ProviderID     string
		Requests       int64
		Success        int64
		InputTokens    int64
		OutputTokens   int64
		DurationSumMs  int64
		FirstRequestAt time.Time
	}

	orphanStats := make(map[string]*aggregate)
	for _, row := range logRows {
		key := providerKey(row.AppType, row.ProviderID)
		if _, ok := imported[key]; ok {
			continue
		}
		item := orphanStats[key]
		if item == nil {
			item = &aggregate{
				AppType:        row.AppType,
				ProviderID:     row.ProviderID,
				FirstRequestAt: time.Unix(row.CreatedAt, 0).UTC(),
			}
			orphanStats[key] = item
		}
		item.Requests++
		if row.StatusCode >= 200 && row.StatusCode < 300 {
			item.Success++
		}
		item.InputTokens += row.InputTokens
		item.OutputTokens += row.OutputTokens
		item.DurationSumMs += chooseDuration(row)
		ts := time.Unix(row.CreatedAt, 0).UTC()
		if ts.Before(item.FirstRequestAt) {
			item.FirstRequestAt = ts
		}
	}

	result := make(map[string]importedNode, len(orphanStats))
	order := 0
	for key, item := range orphanStats {
		protocol := detectProtocol(item.AppType, nil, nil)
		if protocol == "" {
			continue
		}
		id := orphanNodeID(item.ProviderID)
		name := orphanNodeName(item.AppType, item.ProviderID)
		record := store.NodeRecord{
			ID:                id,
			Name:              name,
			BaseURL:           "https://orphan.invalid",
			HealthCheckMethod: "head",
			HealthCheckModel:  defaultHealthModelForProtocol(protocol),
			SourceProtocol:    protocol,
			AccountID:         accountID,
			Weight:            weightOffset + 10000 + order,
			Disabled:          true,
			CreatedAt:         item.FirstRequestAt,
			Requests:          item.Requests,
			FailCount:         item.Requests - item.Success,
			TotalInput:        item.InputTokens,
			TotalOutput:       item.OutputTokens,
			StreamDurMs:       item.DurationSumMs,
		}
		result[key] = importedNode{Record: record, Name: name}
		order++
	}
	return result
}

func loadLogs(ctx context.Context, db *sql.DB) ([]sourceLog, error) {
	rows, err := db.QueryContext(ctx, `SELECT request_id, provider_id, app_type, model, request_model, input_tokens, output_tokens, total_cost_usd, latency_ms, duration_ms, status_code, error_message, created_at
		FROM proxy_request_logs
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []sourceLog
	for rows.Next() {
		var item sourceLog
		if err := rows.Scan(&item.RequestID, &item.ProviderID, &item.AppType, &item.Model, &item.RequestModel, &item.InputTokens, &item.OutputTokens, &item.TotalCostUSD, &item.LatencyMs, &item.DurationMs, &item.StatusCode, &item.ErrorMessage, &item.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func parseJSONObject(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

func detectProtocol(appType string, settings map[string]any, meta map[string]any) string {
	if api := strings.ToLower(getString(settings, "api")); api != "" {
		switch {
		case strings.Contains(api, "anthropic"):
			return "claude"
		case strings.Contains(api, "openai"):
			return "openai"
		case strings.Contains(api, "gemini"):
			return "gemini"
		}
	}
	if getString(settings, "env", "ANTHROPIC_BASE_URL") != "" || getString(settings, "otherFields", "env", "ANTHROPIC_BASE_URL") != "" {
		return "claude"
	}
	if getString(settings, "env", "GOOGLE_GEMINI_BASE_URL") != "" || getString(settings, "otherFields", "env", "GOOGLE_GEMINI_BASE_URL") != "" {
		return "gemini"
	}
	if getString(settings, "env", "OPENAI_BASE_URL") != "" || getString(settings, "otherFields", "env", "OPENAI_BASE_URL") != "" {
		return "openai"
	}
	if npm := strings.ToLower(getString(settings, "npm")); npm != "" {
		switch {
		case strings.Contains(npm, "anthropic"):
			return "claude"
		case strings.Contains(npm, "gemini"):
			return "gemini"
		case strings.Contains(npm, "openai"):
			return "openai"
		}
	}
	if apiFormat := strings.ToLower(getString(meta, "apiFormat")); apiFormat != "" {
		switch {
		case strings.Contains(apiFormat, "anthropic"):
			return "claude"
		case strings.Contains(apiFormat, "codex"):
			return "codex"
		case strings.Contains(apiFormat, "openai"):
			return "openai"
		case strings.Contains(apiFormat, "gemini"):
			return "gemini"
		}
	}
	// Detect Codex via wire_api = "responses" in config string
	if cfg := getString(settings, "config"); strings.Contains(strings.ToLower(cfg), `wire_api = "responses"`) {
		return "codex"
	}
	switch strings.ToLower(appType) {
	case "claude":
		return "claude"
	case "codex":
		return "codex"
	case "gemini":
		return "gemini"
	case "openclaw":
		return "claude"
	case "opencode":
		return "openai"
	default:
		return ""
	}
}

func extractBaseURL(endpoints []string, settings map[string]any) string {
	for _, endpoint := range endpoints {
		if trimmed := strings.TrimSpace(endpoint); trimmed != "" {
			return trimmed
		}
	}
	for _, candidate := range []string{
		getString(settings, "options", "baseURL"),
		getString(settings, "baseUrl"),
		getString(settings, "env", "ANTHROPIC_BASE_URL"),
		getString(settings, "otherFields", "env", "ANTHROPIC_BASE_URL"),
		getString(settings, "env", "GOOGLE_GEMINI_BASE_URL"),
		getString(settings, "otherFields", "env", "GOOGLE_GEMINI_BASE_URL"),
		getString(settings, "env", "OPENAI_BASE_URL"),
		getString(settings, "otherFields", "env", "OPENAI_BASE_URL"),
	} {
		if candidate != "" {
			return candidate
		}
	}
	if cfg := getString(settings, "config"); cfg != "" {
		if baseURL := lookupTOMLString(cfg, "base_url"); baseURL != "" {
			return baseURL
		}
	}
	return ""
}

func extractAPIKey(settings map[string]any) string {
	for _, candidate := range []string{
		getString(settings, "options", "apiKey"),
		getString(settings, "apiKey"),
		getString(settings, "auth", "OPENAI_API_KEY"),
		getString(settings, "env", "ANTHROPIC_AUTH_TOKEN"),
		getString(settings, "env", "ANTHROPIC_API_KEY"),
		getString(settings, "otherFields", "env", "ANTHROPIC_AUTH_TOKEN"),
		getString(settings, "otherFields", "env", "ANTHROPIC_API_KEY"),
		getString(settings, "env", "OPENAI_API_KEY"),
		getString(settings, "otherFields", "env", "OPENAI_API_KEY"),
		getString(settings, "env", "GEMINI_API_KEY"),
		getString(settings, "otherFields", "env", "GEMINI_API_KEY"),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func extractHealthModel(protocol string, settings map[string]any, modelMapping map[string]string) string {
	switch protocol {
	case "openai":
		if cfg := getString(settings, "config"); cfg != "" {
			if model := lookupTOMLString(cfg, "model"); model != "" {
				return model
			}
		}
		if model := firstModelKey(settings); model != "" {
			return model
		}
		if model := firstModelIDFromArray(settings); model != "" {
			return model
		}
		return defaultOpenAIHealthModel
	case "gemini":
		if model := getString(settings, "env", "GEMINI_MODEL"); model != "" {
			return model
		}
		if model := getString(settings, "otherFields", "env", "GEMINI_MODEL"); model != "" {
			return model
		}
		if model := firstModelKey(settings); model != "" {
			return model
		}
		if model := firstModelIDFromArray(settings); model != "" {
			return model
		}
		return defaultGeminiHealthModel
	default:
		for _, candidate := range []string{
			getString(settings, "env", "ANTHROPIC_MODEL"),
			getString(settings, "env", "ANTHROPIC_DEFAULT_SONNET_MODEL"),
			getString(settings, "env", "ANTHROPIC_DEFAULT_HAIKU_MODEL"),
			getString(settings, "env", "ANTHROPIC_DEFAULT_OPUS_MODEL"),
			getString(settings, "otherFields", "env", "ANTHROPIC_MODEL"),
			getString(settings, "otherFields", "env", "ANTHROPIC_DEFAULT_SONNET_MODEL"),
			getString(settings, "otherFields", "env", "ANTHROPIC_DEFAULT_HAIKU_MODEL"),
			getString(settings, "otherFields", "env", "ANTHROPIC_DEFAULT_OPUS_MODEL"),
		} {
			if candidate != "" {
				return candidate
			}
		}
		if alias := firstModelAlias(modelMapping); alias != "" {
			return alias
		}
		if model := firstModelIDFromArray(settings); model != "" {
			return model
		}
		return defaultClaudeHealthModel
	}
}

func extractModelMapping(protocol string, settings map[string]any) map[string]string {
	models := getMap(settings, "models")
	if len(models) == 0 {
		return nil
	}
	keys := make([]string, 0, len(models))
	for key := range models {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string)
	for _, key := range keys {
		valMap, ok := models[key].(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(getString(valMap, "name"))
		if name == "" || name == key {
			continue
		}
		result[key] = name
	}
	if protocol == "claude" {
		if v := getString(settings, "env", "ANTHROPIC_DEFAULT_HAIKU_MODEL"); v != "" {
			result["claude-haiku-4-5-20251001"] = v
		}
		if v := getString(settings, "otherFields", "env", "ANTHROPIC_DEFAULT_HAIKU_MODEL"); v != "" {
			result["claude-haiku-4-5-20251001"] = v
		}
		if v := getString(settings, "env", "ANTHROPIC_DEFAULT_SONNET_MODEL"); v != "" {
			result["claude-sonnet-4-5-20250929"] = v
		}
		if v := getString(settings, "otherFields", "env", "ANTHROPIC_DEFAULT_SONNET_MODEL"); v != "" {
			result["claude-sonnet-4-5-20250929"] = v
		}
		if v := getString(settings, "env", "ANTHROPIC_DEFAULT_OPUS_MODEL"); v != "" {
			result["claude-opus-4-6"] = v
		}
		if v := getString(settings, "otherFields", "env", "ANTHROPIC_DEFAULT_OPUS_MODEL"); v != "" {
			result["claude-opus-4-6"] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func defaultHealthModelForProtocol(protocol string) string {
	switch protocol {
	case "openai":
		return defaultOpenAIHealthModel
	case "gemini":
		return defaultGeminiHealthModel
	default:
		return defaultClaudeHealthModel
	}
}

func firstModelKey(settings map[string]any) string {
	models := getMap(settings, "models")
	if len(models) == 0 {
		return ""
	}
	keys := make([]string, 0, len(models))
	for key := range models {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[0]
}

func firstModelAlias(modelMapping map[string]string) string {
	if len(modelMapping) == 0 {
		return ""
	}
	keys := make([]string, 0, len(modelMapping))
	for key := range modelMapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return modelMapping[keys[0]]
}

func firstModelIDFromArray(settings map[string]any) string {
	models, ok := settings["models"].([]any)
	if !ok || len(models) == 0 {
		return ""
	}
	first, ok := models[0].(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(getString(first, "id"))
}

func lookupTOMLString(raw, key string) string {
	for _, match := range tomlStringPattern.FindAllStringSubmatch(raw, -1) {
		if len(match) == 3 && match[1] == key {
			return strings.TrimSpace(match[2])
		}
	}
	return ""
}

func getMap(data map[string]any, path ...string) map[string]any {
	if len(path) == 0 {
		return data
	}
	current := data
	for i, key := range path {
		if current == nil {
			return nil
		}
		val, ok := current[key]
		if !ok {
			return nil
		}
		if i == len(path)-1 {
			if child, ok := val.(map[string]any); ok {
				return child
			}
			return nil
		}
		child, ok := val.(map[string]any)
		if !ok {
			return nil
		}
		current = child
	}
	return nil
}

func getString(data map[string]any, path ...string) string {
	if len(path) == 0 {
		return ""
	}
	var current any = data
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = obj[key]
	}
	switch v := current.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func parseDecimal(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func millisToTime(ms int64) time.Time {
	sec := ms / 1000
	nsec := (ms % 1000) * int64(time.Millisecond)
	return time.Unix(sec, nsec).UTC()
}

func providerKey(appType, providerID string) string {
	return appType + "|" + providerID
}

func importedNodeID(appType, providerID string) string {
	return "ccswitch-" + appType + "-" + providerID
}

func importedNodeName(appType, name string) string {
	return appType + ":" + name
}

func orphanNodeID(providerID string) string {
	return "ccswitch-orphan-" + providerID
}

func orphanNodeName(appType, providerID string) string {
	shortID := providerID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return appType + ":orphan:" + shortID
}

func chooseNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func urlParse(raw string) (*url.URL, error) {
	return url.Parse(strings.TrimSpace(raw))
}

func chooseDuration(row sourceLog) int64 {
	if row.DurationMs.Valid {
		return row.DurationMs.Int64
	}
	return row.LatencyMs
}
