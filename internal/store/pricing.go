package store

import (
	"context"
	cryptoRand "crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"strings"
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

	// 迁移：添加 node_name 和 duration_ms 列（已有表不会重复添加）
	if s.IsSQLite() {
		s.db.ExecContext(ctx, `ALTER TABLE usage_logs ADD COLUMN node_name TEXT NOT NULL DEFAULT ''`)
		s.db.ExecContext(ctx, `ALTER TABLE usage_logs ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0`)
		s.db.ExecContext(ctx, `ALTER TABLE usage_logs ADD COLUMN total_attempts INTEGER NOT NULL DEFAULT 1`)
	} else {
		s.db.ExecContext(ctx, "ALTER TABLE usage_logs ADD COLUMN node_name VARCHAR(255) NOT NULL DEFAULT ''")
		s.db.ExecContext(ctx, "ALTER TABLE usage_logs ADD COLUMN duration_ms BIGINT NOT NULL DEFAULT 0")
		s.db.ExecContext(ctx, "ALTER TABLE usage_logs ADD COLUMN total_attempts INT NOT NULL DEFAULT 1")
	}

	// 迁移：添加 error_msg 列（存储最终错误信息，便于直接展示）
	if s.IsSQLite() {
		s.db.ExecContext(ctx, `ALTER TABLE usage_logs ADD COLUMN error_msg TEXT NOT NULL DEFAULT ''`)
	} else {
		s.db.ExecContext(ctx, "ALTER TABLE usage_logs ADD COLUMN error_msg TEXT NOT NULL DEFAULT ('')")
	}

	// 创建 usage_log_attempts 表（链路追踪）
	attemptsTable := `CREATE TABLE IF NOT EXISTS usage_log_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		log_id INTEGER NOT NULL,
		seq INTEGER NOT NULL DEFAULT 1,
		node_id TEXT NOT NULL DEFAULT '',
		node_name TEXT NOT NULL DEFAULT '',
		status_code INTEGER NOT NULL DEFAULT 0,
		success INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		error_msg TEXT NOT NULL DEFAULT '',
		severity TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT ''
	)`
	if !s.IsSQLite() {
		attemptsTable = `CREATE TABLE IF NOT EXISTS usage_log_attempts (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			log_id BIGINT NOT NULL,
			seq INT NOT NULL DEFAULT 1,
			node_id VARCHAR(255) NOT NULL DEFAULT '',
			node_name VARCHAR(255) NOT NULL DEFAULT '',
			status_code INT NOT NULL DEFAULT 0,
			success TINYINT(1) NOT NULL DEFAULT 0,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			error_msg TEXT NOT NULL DEFAULT (''),
			severity VARCHAR(50) NOT NULL DEFAULT '',
			action VARCHAR(50) NOT NULL DEFAULT '',
			INDEX idx_log_id (log_id)
		)`
	}
	if _, err := s.db.ExecContext(ctx, attemptsTable); err != nil {
		return err
	}
	if s.IsSQLite() {
		s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_attempts_log_id ON usage_log_attempts(log_id)`)
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

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var stmt string
	if s.IsSQLite() {
		stmt = `INSERT OR IGNORE INTO model_pricing (id, model_id, model_name, input_price_mtok, output_price_mtok, is_active) VALUES (?, ?, ?, ?, ?, ?)`
	} else {
		stmt = `INSERT IGNORE INTO model_pricing (id, model_id, model_name, input_price_mtok, output_price_mtok, is_active) VALUES (?, ?, ?, ?, ?, ?)`
	}

	for _, p := range OfficialPricing() {
		p.ID = genUUID()
		_, err := s.db.ExecContext(ctx, stmt,
			p.ID, p.ModelID, p.ModelName, p.InputPriceMTok, p.OutputPriceMTok, p.IsActive)
		if err != nil {
			return err
		}
	}
	return nil
}

// OfficialPricing 返回 Anthropic 官方模型定价表（截至 2026-02）
// 数据来源: https://docs.anthropic.com/en/docs/about-claude/pricing
func OfficialPricing() []ModelPricingRecord {
	return []ModelPricingRecord{
		// 最新一代模型
		{ModelID: "claude-opus-4-6", ModelName: "Claude Opus 4.6", InputPriceMTok: 5.0, OutputPriceMTok: 25.0, IsActive: true},
		{ModelID: "claude-sonnet-4-5-20250929", ModelName: "Claude Sonnet 4.5", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-haiku-4-5-20251001", ModelName: "Claude Haiku 4.5", InputPriceMTok: 1.0, OutputPriceMTok: 5.0, IsActive: true},

		// Legacy 模型
		{ModelID: "claude-opus-4-5-20251101", ModelName: "Claude Opus 4.5", InputPriceMTok: 5.0, OutputPriceMTok: 25.0, IsActive: true},
		{ModelID: "claude-opus-4-1-20250805", ModelName: "Claude Opus 4.1", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-sonnet-4-20250514", ModelName: "Claude Sonnet 4", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-7-sonnet-20250219", ModelName: "Claude Sonnet 3.7", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-opus-4-20250514", ModelName: "Claude Opus 4", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-3-5-sonnet-20241022", ModelName: "Claude 3.5 Sonnet", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-5-haiku-20241022", ModelName: "Claude 3.5 Haiku", InputPriceMTok: 0.8, OutputPriceMTok: 4.0, IsActive: true},
		{ModelID: "claude-3-opus-20240229", ModelName: "Claude 3 Opus", InputPriceMTok: 15.0, OutputPriceMTok: 75.0, IsActive: true},
		{ModelID: "claude-3-sonnet-20240229", ModelName: "Claude 3 Sonnet", InputPriceMTok: 3.0, OutputPriceMTok: 15.0, IsActive: true},
		{ModelID: "claude-3-haiku-20240307", ModelName: "Claude 3 Haiku", InputPriceMTok: 0.25, OutputPriceMTok: 1.25, IsActive: true},
	}
}

// SyncOfficialPricing 从官方定价表同步所有模型定价到数据库
// 已存在的模型会更新价格，新模型会插入，返回同步数量
func (s *Store) SyncOfficialPricing(ctx context.Context) (int, error) {
	official := OfficialPricing()
	synced := 0
	for _, p := range official {
		p.ID = genUUID()
		if err := s.UpsertModelPricing(ctx, p); err != nil {
			return synced, fmt.Errorf("sync model %s failed: %w", p.ModelID, err)
		}
		synced++
	}
	return synced, nil
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
	if log.TotalAttempts == 0 {
		log.TotalAttempts = 1
	}

	result, err := s.db.ExecContext(ctx,
		`INSERT INTO usage_logs (account_id, node_id, node_name, model_id, input_tokens, output_tokens, cost_usd, request_id, success, error_msg, duration_ms, total_attempts, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.AccountID, log.NodeID, log.NodeName, log.ModelID, log.InputTokens, log.OutputTokens, log.CostUSD, log.RequestID, log.Success, log.ErrorMsg, log.DurationMs, log.TotalAttempts, log.CreatedAt)
	if err != nil {
		return err
	}

	// 写入 attempts 子记录
	if len(log.Attempts) > 0 {
		logID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		for _, a := range log.Attempts {
			_, err := s.db.ExecContext(ctx,
				`INSERT INTO usage_log_attempts (log_id, seq, node_id, node_name, status_code, success, duration_ms, error_msg, severity, action)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				logID, a.Seq, a.NodeID, a.NodeName, a.StatusCode, a.Success, a.DurationMs, a.ErrorMsg, a.Severity, a.Action)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// QueryUsageLogs 查询使用日志
func (s *Store) QueryUsageLogs(ctx context.Context, params QueryUsageParams) ([]UsageLogRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `SELECT id, account_id, node_id, node_name, model_id, input_tokens, output_tokens, cost_usd, request_id, success, error_msg, duration_ms, total_attempts, created_at
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
	if params.Success != nil {
		query += " AND success = ?"
		args = append(args, *params.Success)
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
		var nodeName sql.NullString
		var errorMsg sql.NullString
		if err := rows.Scan(&log.ID, &log.AccountID, &log.NodeID, &nodeName, &log.ModelID, &log.InputTokens, &log.OutputTokens, &log.CostUSD, &reqID, &log.Success, &errorMsg, &log.DurationMs, &log.TotalAttempts, &log.CreatedAt); err != nil {
			return nil, err
		}
		if reqID.Valid {
			log.RequestID = reqID.String
		}
		if nodeName.Valid {
			log.NodeName = nodeName.String
		}
		if errorMsg.Valid {
			log.ErrorMsg = errorMsg.String
		}
		results = append(results, log)
	}
	return results, rows.Err()
}

// CountUsageLogs 统计使用日志总数（用于分页）
func (s *Store) CountUsageLogs(ctx context.Context, params QueryUsageParams) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `SELECT COUNT(*) FROM usage_logs WHERE 1=1`
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
	if params.Success != nil {
		query += " AND success = ?"
		args = append(args, *params.Success)
	}
	if !params.From.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, params.From.UTC())
	}
	if !params.To.IsZero() {
		query += " AND created_at < ?"
		args = append(args, params.To.UTC())
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// QueryAttemptsByLogIDs 批量查询指定日志 ID 的尝试记录
func (s *Store) QueryAttemptsByLogIDs(ctx context.Context, logIDs []int64) (map[int64][]UsageLogAttempt, error) {
	if len(logIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// 构建 IN 子句
	placeholders := make([]string, len(logIDs))
	args := make([]interface{}, len(logIDs))
	for i, id := range logIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT id, log_id, seq, node_id, node_name, status_code, success, duration_ms, error_msg, severity, action
		FROM usage_log_attempts WHERE log_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY log_id, seq`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]UsageLogAttempt)
	for rows.Next() {
		var a UsageLogAttempt
		if err := rows.Scan(&a.ID, &a.LogID, &a.Seq, &a.NodeID, &a.NodeName, &a.StatusCode, &a.Success, &a.DurationMs, &a.ErrorMsg, &a.Severity, &a.Action); err != nil {
			return nil, err
		}
		result[a.LogID] = append(result[a.LogID], a)
	}
	return result, rows.Err()
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
