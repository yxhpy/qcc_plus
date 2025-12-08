package store

import (
	"context"
	cryptoRand "crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// genUUID 生成 UUID v4 字符串
func genUUID() string {
	b := make([]byte, 16)
	_, _ = cryptoRand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ensurePricingTables 创建模型定价和使用日志表
func (s *Store) ensurePricingTables(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// 模型定价表
	var pricingTable, usageLogTable string
	if s.IsSQLite() {
		pricingTable = `CREATE TABLE IF NOT EXISTS model_pricing (
			id TEXT PRIMARY KEY,
			model_id TEXT NOT NULL UNIQUE,
			model_name TEXT NOT NULL,
			input_price_mtok REAL NOT NULL DEFAULT 0,
			output_price_mtok REAL NOT NULL DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
		usageLogTable = `CREATE TABLE IF NOT EXISTS usage_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cost_usd REAL NOT NULL DEFAULT 0,
			request_id TEXT,
			success INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	} else {
		pricingTable = `CREATE TABLE IF NOT EXISTS model_pricing (
			id VARCHAR(64) PRIMARY KEY,
			model_id VARCHAR(128) NOT NULL,
			model_name VARCHAR(255) NOT NULL,
			input_price_mtok DECIMAL(10,6) NOT NULL DEFAULT 0,
			output_price_mtok DECIMAL(10,6) NOT NULL DEFAULT 0,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uniq_model_id (model_id),
			KEY idx_is_active (is_active)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		usageLogTable = `CREATE TABLE IF NOT EXISTS usage_logs (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			account_id VARCHAR(64) NOT NULL,
			node_id VARCHAR(64) NOT NULL,
			model_id VARCHAR(128) NOT NULL,
			input_tokens BIGINT NOT NULL DEFAULT 0,
			output_tokens BIGINT NOT NULL DEFAULT 0,
			cost_usd DECIMAL(16,8) NOT NULL DEFAULT 0,
			request_id VARCHAR(128),
			success BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			KEY idx_account_time (account_id, created_at),
			KEY idx_node_time (node_id, created_at),
			KEY idx_model_time (model_id, created_at),
			KEY idx_account_node (account_id, node_id),
			KEY idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	}

	if _, err := s.db.ExecContext(ctx, pricingTable); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, usageLogTable); err != nil {
		return err
	}

	// Create indexes for SQLite
	if s.IsSQLite() {
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_is_active ON model_pricing(is_active)`)
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_account_time ON usage_logs(account_id, created_at)`)
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_node_time ON usage_logs(node_id, created_at)`)
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_model_time ON usage_logs(model_id, created_at)`)
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_account_node ON usage_logs(account_id, node_id)`)
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_created_at ON usage_logs(created_at)`)
	}

	return nil
}

// SeedDefaultPricing 预置默认的模型定价数据
func (s *Store) SeedDefaultPricing(ctx context.Context) error {
	// 检查是否已有定价数据
	var count int
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM model_pricing")
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // 已有数据，不再预置
	}

	// Claude 官方定价（截至 2025-12-06）
	defaultPricing := []ModelPricingRecord{
		// 最新旗舰模型
		{ModelID: "claude-opus-4-5-20251101", ModelName: "Claude Opus 4.5", InputPriceMTok: 5.0, OutputPriceMTok: 25.0, IsActive: true},
		{ModelID: "claude-sonnet-4-5-20250929", ModelName: "Claude Sonnet 4.5", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-haiku-4-5-20251001", ModelName: "Claude Haiku 4.5", InputPriceMTok: 1.0, OutputPriceMTok: 5.0, IsActive: true},

		// Legacy 模型
		{ModelID: "claude-opus-4-20250514", ModelName: "Claude Opus 4", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-sonnet-4-20250514", ModelName: "Claude Sonnet 4", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-5-sonnet-20241022", ModelName: "Claude 3.5 Sonnet", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-5-haiku-20241022", ModelName: "Claude 3.5 Haiku", InputPriceMTok: 0.8, OutputPriceMTok: 4.0, IsActive: true},
		{ModelID: "claude-3-opus-20240229", ModelName: "Claude 3 Opus", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-3-sonnet-20240229", ModelName: "Claude 3 Sonnet", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-haiku-20240307", ModelName: "Claude 3 Haiku", InputPriceMTok: 0.25, OutputPriceMTok: 1.25, IsActive: true},
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var stmt string
	if s.IsSQLite() {
		stmt = `INSERT OR IGNORE INTO model_pricing (id, model_id, model_name, input_price_mtok, output_price_mtok, is_active) VALUES (?, ?, ?, ?, ?, ?)`
	} else {
		stmt = `INSERT IGNORE INTO model_pricing (id, model_id, model_name, input_price_mtok, output_price_mtok, is_active) VALUES (?, ?, ?, ?, ?, ?)`
	}

	for _, p := range defaultPricing {
		p.ID = genUUID()
		_, err := s.db.ExecContext(ctx, stmt,
			p.ID, p.ModelID, p.ModelName, p.InputPriceMTok, p.OutputPriceMTok, p.IsActive)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetModelPricing 获取单个模型定价
func (s *Store) GetModelPricing(ctx context.Context, modelID string) (*ModelPricingRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	row := s.db.QueryRowContext(ctx,
		`SELECT id, model_id, model_name, input_price_mtok, output_price_mtok, is_active, created_at, updated_at
		FROM model_pricing WHERE model_id = ?`, modelID)

	var p ModelPricingRecord
	err := row.Scan(&p.ID, &p.ModelID, &p.ModelName, &p.InputPriceMTok, &p.OutputPriceMTok, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListModelPricing 列出所有模型定价
func (s *Store) ListModelPricing(ctx context.Context, activeOnly bool) ([]ModelPricingRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `SELECT id, model_id, model_name, input_price_mtok, output_price_mtok, is_active, created_at, updated_at FROM model_pricing`
	if activeOnly {
		if s.IsSQLite() {
			query += " WHERE is_active = 1"
		} else {
			query += " WHERE is_active = TRUE"
		}
	}
	query += " ORDER BY model_name ASC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ModelPricingRecord
	for rows.Next() {
		var p ModelPricingRecord
		if err := rows.Scan(&p.ID, &p.ModelID, &p.ModelName, &p.InputPriceMTok, &p.OutputPriceMTok, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// UpsertModelPricing 创建或更新模型定价
func (s *Store) UpsertModelPricing(ctx context.Context, p ModelPricingRecord) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	if p.ID == "" {
		p.ID = genUUID()
	}

	var err error
	if s.IsSQLite() {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO model_pricing (id, model_id, model_name, input_price_mtok, output_price_mtok, is_active)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(model_id) DO UPDATE SET
				model_name = excluded.model_name,
				input_price_mtok = excluded.input_price_mtok,
				output_price_mtok = excluded.output_price_mtok,
				is_active = excluded.is_active,
				updated_at = CURRENT_TIMESTAMP`,
			p.ID, p.ModelID, p.ModelName, p.InputPriceMTok, p.OutputPriceMTok, p.IsActive)
	} else {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO model_pricing (id, model_id, model_name, input_price_mtok, output_price_mtok, is_active)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				model_name = VALUES(model_name),
				input_price_mtok = VALUES(input_price_mtok),
				output_price_mtok = VALUES(output_price_mtok),
				is_active = VALUES(is_active)`,
			p.ID, p.ModelID, p.ModelName, p.InputPriceMTok, p.OutputPriceMTok, p.IsActive)
	}
	return err
}

// DeleteModelPricing 删除模型定价
func (s *Store) DeleteModelPricing(ctx context.Context, modelID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	result, err := s.db.ExecContext(ctx, "DELETE FROM model_pricing WHERE model_id = ?", modelID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// CalculateCost 计算指定模型的费用（美元）
func (s *Store) CalculateCost(ctx context.Context, modelID string, inputTokens, outputTokens int64) (float64, error) {
	pricing, err := s.GetModelPricing(ctx, modelID)
	if err != nil {
		if err == ErrNotFound {
			// 未知模型返回 0 费用，记录警告便于追踪
			log.Printf("[pricing] unknown model %q, cost calculated as $0 (input=%d, output=%d tokens)", modelID, inputTokens, outputTokens)
			return 0, nil
		}
		return 0, err
	}

	// 计算费用：tokens / 1,000,000 * price_per_mtok
	inputCost := float64(inputTokens) / 1_000_000 * pricing.InputPriceMTok
	outputCost := float64(outputTokens) / 1_000_000 * pricing.OutputPriceMTok
	return inputCost + outputCost, nil
}

// InsertUsageLog 插入使用日志
func (s *Store) InsertUsageLog(ctx context.Context, log UsageLogRecord) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	log.AccountID = normalizeAccount(log.AccountID)
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage_logs (account_id, node_id, model_id, input_tokens, output_tokens, cost_usd, request_id, success, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.AccountID, log.NodeID, log.ModelID, log.InputTokens, log.OutputTokens, log.CostUSD, log.RequestID, log.Success, log.CreatedAt)
	return err
}

// QueryUsageLogs 查询使用日志
func (s *Store) QueryUsageLogs(ctx context.Context, params QueryUsageParams) ([]UsageLogRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `SELECT id, account_id, node_id, model_id, input_tokens, output_tokens, cost_usd, request_id, success, created_at
		FROM usage_logs WHERE 1=1`
	var args []interface{}

	if params.AccountID != "" {
		query += " AND account_id = ?"
		args = append(args, normalizeAccount(params.AccountID))
	}
	if params.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, params.NodeID)
	}
	if params.ModelID != "" {
		query += " AND model_id = ?"
		args = append(args, params.ModelID)
	}
	if !params.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, params.From.UTC())
	}
	if !params.To.IsZero() {
		query += " AND created_at < ?"
		args = append(args, params.To.UTC())
	}

	query += " ORDER BY created_at DESC"

	if params.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, params.Limit)
	}
	if params.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, params.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UsageLogRecord
	for rows.Next() {
		var log UsageLogRecord
		var reqID sql.NullString
		if err := rows.Scan(&log.ID, &log.AccountID, &log.NodeID, &log.ModelID, &log.InputTokens, &log.OutputTokens, &log.CostUSD, &reqID, &log.Success, &log.CreatedAt); err != nil {
			return nil, err
		}
		if reqID.Valid {
			log.RequestID = reqID.String
		}
		results = append(results, log)
	}
	return results, rows.Err()
}

// GetUsageSummary 获取使用汇总（按账号、可选按节点或模型分组）
func (s *Store) GetUsageSummary(ctx context.Context, params QueryUsageParams) (*UsageSummary, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// SQLite uses 1/0 for boolean, MySQL uses TRUE/FALSE - both work with success = 1
	query := `SELECT
		COALESCE(COUNT(*), 0) as total_requests,
		COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) as success_requests,
		COALESCE(SUM(input_tokens), 0) as total_input_tokens,
		COALESCE(SUM(output_tokens), 0) as total_output_tokens,
		COALESCE(SUM(cost_usd), 0) as total_cost_usd
		FROM usage_logs WHERE 1=1`
	var args []interface{}

	if params.AccountID != "" {
		query += " AND account_id = ?"
		args = append(args, normalizeAccount(params.AccountID))
	}
	if params.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, params.NodeID)
	}
	if params.ModelID != "" {
		query += " AND model_id = ?"
		args = append(args, params.ModelID)
	}
	if !params.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, params.From.UTC())
	}
	if !params.To.IsZero() {
		query += " AND created_at < ?"
		args = append(args, params.To.UTC())
	}

	row := s.db.QueryRowContext(ctx, query, args...)
	var summary UsageSummary
	if err := row.Scan(&summary.TotalRequests, &summary.SuccessRequests, &summary.TotalInputTokens, &summary.TotalOutputTokens, &summary.TotalCostUSD); err != nil {
		return nil, err
	}
	summary.AccountID = params.AccountID
	summary.NodeID = params.NodeID
	summary.ModelID = params.ModelID
	return &summary, nil
}

// GetUsageSummaryByModel 按模型分组获取使用汇总
func (s *Store) GetUsageSummaryByModel(ctx context.Context, params QueryUsageParams) ([]UsageSummary, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// SQLite uses 1/0 for boolean, MySQL uses TRUE/FALSE - both work with success = 1
	query := `SELECT
		model_id,
		COALESCE(COUNT(*), 0) as total_requests,
		COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) as success_requests,
		COALESCE(SUM(input_tokens), 0) as total_input_tokens,
		COALESCE(SUM(output_tokens), 0) as total_output_tokens,
		COALESCE(SUM(cost_usd), 0) as total_cost_usd
		FROM usage_logs WHERE 1=1`
	var args []interface{}

	if params.AccountID != "" {
		query += " AND account_id = ?"
		args = append(args, normalizeAccount(params.AccountID))
	}
	if params.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, params.NodeID)
	}
	if !params.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, params.From.UTC())
	}
	if !params.To.IsZero() {
		query += " AND created_at < ?"
		args = append(args, params.To.UTC())
	}

	query += " GROUP BY model_id ORDER BY total_cost_usd DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UsageSummary
	for rows.Next() {
		var summary UsageSummary
		if err := rows.Scan(&summary.ModelID, &summary.TotalRequests, &summary.SuccessRequests, &summary.TotalInputTokens, &summary.TotalOutputTokens, &summary.TotalCostUSD); err != nil {
			return nil, err
		}
		summary.AccountID = params.AccountID
		summary.NodeID = params.NodeID
		results = append(results, summary)
	}
	return results, rows.Err()
}

// GetUsageSummaryByNode 按节点分组获取使用汇总
func (s *Store) GetUsageSummaryByNode(ctx context.Context, params QueryUsageParams) ([]UsageSummary, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// SQLite uses 1/0 for boolean, MySQL uses TRUE/FALSE - both work with success = 1
	query := `SELECT
		node_id,
		COALESCE(COUNT(*), 0) as total_requests,
		COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) as success_requests,
		COALESCE(SUM(input_tokens), 0) as total_input_tokens,
		COALESCE(SUM(output_tokens), 0) as total_output_tokens,
		COALESCE(SUM(cost_usd), 0) as total_cost_usd
		FROM usage_logs WHERE 1=1`
	var args []interface{}

	if params.AccountID != "" {
		query += " AND account_id = ?"
		args = append(args, normalizeAccount(params.AccountID))
	}
	if params.ModelID != "" {
		query += " AND model_id = ?"
		args = append(args, params.ModelID)
	}
	if !params.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, params.From.UTC())
	}
	if !params.To.IsZero() {
		query += " AND created_at < ?"
		args = append(args, params.To.UTC())
	}

	query += " GROUP BY node_id ORDER BY total_cost_usd DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UsageSummary
	for rows.Next() {
		var summary UsageSummary
		if err := rows.Scan(&summary.NodeID, &summary.TotalRequests, &summary.SuccessRequests, &summary.TotalInputTokens, &summary.TotalOutputTokens, &summary.TotalCostUSD); err != nil {
			return nil, err
		}
		summary.AccountID = params.AccountID
		summary.ModelID = params.ModelID
		results = append(results, summary)
	}
	return results, rows.Err()
}

// CleanupUsageLogs 清理旧的使用日志（保留指定天数）
func (s *Store) CleanupUsageLogs(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 365 // 默认保留一年
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	_, err := s.db.ExecContext(ctx, "DELETE FROM usage_logs WHERE created_at < ?", cutoff)
	return err
}
