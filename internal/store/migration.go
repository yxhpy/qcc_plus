package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
)

func (s *Store) ensureAccountsTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := `CREATE TABLE IF NOT EXISTS accounts (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		password VARCHAR(500) DEFAULT '',
		proxy_api_key VARCHAR(255),
		is_admin BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE KEY uniq_proxy_api_key (proxy_api_key)
	)`
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return err
	}
	return nil
}

// ensureAccountPassword 迁移 accounts 表增加 password 列，并填充默认密码。
func (s *Store) ensureAccountPassword(ctx context.Context) error {
	hasPwd, err := s.columnExists(context.Background(), "accounts", "password")
	if err != nil {
		return err
	}
	if !hasPwd {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE accounts ADD COLUMN password VARCHAR(500) DEFAULT '' AFTER name`); err != nil {
			return err
		}
	}

	// 补齐默认账号与管理员账号的初始密码（仅空密码时写入）。
	updCtx, cancel := withTimeout(ctx)
	defer cancel()
	if _, err := s.db.ExecContext(updCtx, `UPDATE accounts SET password=? WHERE (password IS NULL OR password='') AND id=?`, "default123", DefaultAccountID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(updCtx, `UPDATE accounts SET password=? WHERE (password IS NULL OR password='') AND (name='admin' OR is_admin=TRUE)`, "admin123"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureNodesTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := `CREATE TABLE IF NOT EXISTS nodes (
            id VARCHAR(64) PRIMARY KEY,
            name VARCHAR(255),
            base_url TEXT NOT NULL,
            api_key TEXT,
			health_check_method VARCHAR(10) DEFAULT 'api',
			account_id VARCHAR(64) NOT NULL DEFAULT '` + DefaultAccountID + `',
            weight INT DEFAULT 1,
            failed BOOLEAN DEFAULT FALSE,
			disabled BOOLEAN DEFAULT FALSE,
            last_error TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            requests BIGINT DEFAULT 0,
            fail_count BIGINT DEFAULT 0,
            fail_streak BIGINT DEFAULT 0,
            total_bytes BIGINT DEFAULT 0,
            total_input BIGINT DEFAULT 0,
            total_output BIGINT DEFAULT 0,
            stream_dur_ms BIGINT DEFAULT 0,
            first_byte_ms BIGINT DEFAULT 0,
            last_ping_ms BIGINT DEFAULT -1,
            last_ping_err TEXT,
			last_health_check_at DATETIME DEFAULT NULL,
			KEY idx_nodes_account (account_id)
        )`
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return err
	}

	// 兼容旧版本，补充缺失的列。
	hasDisabled, err := s.columnExists(context.Background(), "nodes", "disabled")
	if err != nil {
		return err
	}
	if !hasDisabled {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE nodes ADD COLUMN disabled BOOLEAN DEFAULT FALSE AFTER failed`); err != nil {
			return err
		}
	}

	hasAccount, err := s.columnExists(context.Background(), "nodes", "account_id")
	if err != nil {
		return err
	}
	if !hasAccount {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE nodes ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '`+DefaultAccountID+`' AFTER api_key`); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(alterCtx, `CREATE INDEX idx_nodes_account ON nodes(account_id)`); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(alterCtx, `UPDATE nodes SET account_id='`+DefaultAccountID+`' WHERE account_id IS NULL OR account_id=''`); err != nil {
			return err
		}
	}

	hasLastHealthCheckAt, err := s.columnExists(context.Background(), "nodes", "last_health_check_at")
	if err != nil {
		return err
	}
	if !hasLastHealthCheckAt {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE nodes ADD COLUMN last_health_check_at DATETIME DEFAULT NULL AFTER last_ping_err`); err != nil {
			return err
		}
	}

	hasHealthMethod, err := s.columnExists(context.Background(), "nodes", "health_check_method")
	if err != nil {
		return err
	}
	if !hasHealthMethod {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE nodes ADD COLUMN health_check_method VARCHAR(10) DEFAULT 'api' AFTER api_key`); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureMonitorShareTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := `CREATE TABLE IF NOT EXISTS monitor_shares (
		id VARCHAR(64) PRIMARY KEY,
		account_id VARCHAR(64) NOT NULL,
		token VARCHAR(128) NOT NULL,
		expire_at DATETIME NULL,
		created_by VARCHAR(64) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		revoked BOOLEAN DEFAULT FALSE,
		revoked_at DATETIME NULL,
		UNIQUE KEY uniq_monitor_share_token (token),
		KEY idx_monitor_share_account (account_id)
	)`
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureHealthHistoryTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := `CREATE TABLE IF NOT EXISTS health_check_history (
	  id BIGINT AUTO_INCREMENT PRIMARY KEY,
	  account_id VARCHAR(255) NOT NULL,
	  node_id VARCHAR(255) NOT NULL,
	  check_time DATETIME(3) NOT NULL,
	  success BOOLEAN NOT NULL,
	  response_time_ms INT,
	  error_message TEXT,
	  check_method VARCHAR(20) NOT NULL,
	  check_source VARCHAR(20) NOT NULL DEFAULT 'scheduled',
	  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	  INDEX idx_node_time (node_id, check_time),
	  INDEX idx_account_node_time (account_id, node_id, check_time),
	  INDEX idx_account_node_source_time (account_id, node_id, check_source, check_time)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return err
	}

	hasCheckSource, err := s.columnExists(context.Background(), "health_check_history", "check_source")
	if err != nil {
		return err
	}
	if !hasCheckSource {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE health_check_history ADD COLUMN check_source VARCHAR(20) NOT NULL DEFAULT 'scheduled' AFTER check_method`); err != nil {
			return err
		}
	}

	hasIndex, err := s.indexExists(context.Background(), "health_check_history", "idx_account_node_source_time")
	if err != nil {
		return err
	}
	if !hasIndex {
		alterCtx, cancel := withTimeout(context.Background())
		defer cancel()
		if _, err := s.db.ExecContext(alterCtx, `ALTER TABLE health_check_history ADD INDEX idx_account_node_source_time (account_id, node_id, check_source, check_time)`); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) ensureConfigTable(ctx context.Context) error {
	hasAccount, err := s.columnExists(context.Background(), "config", "account_id")
	if err != nil {
		return err
	}
	if !hasAccount {
		if err := s.recreateConfigTable(); err != nil {
			return err
		}
	}
	if err := s.ensureConfigRow(ctx, DefaultAccountID); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureTunnelConfigTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := `CREATE TABLE IF NOT EXISTS tunnel_config (
		id VARCHAR(64) PRIMARY KEY,
		api_token VARCHAR(512),
		subdomain VARCHAR(128),
		zone VARCHAR(256),
		enabled TINYINT(1) DEFAULT 0,
		public_url VARCHAR(512),
		status VARCHAR(32),
		last_error TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	)`
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

func (s *Store) ensureMetricsTables(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	createRaw := `CREATE TABLE IF NOT EXISTS node_metrics_raw (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		account_id VARCHAR(64) NOT NULL,
		node_id VARCHAR(64) NOT NULL,
		ts DATETIME NOT NULL,
		requests_total BIGINT DEFAULT 0,
		requests_success BIGINT DEFAULT 0,
		requests_failed BIGINT DEFAULT 0,
		response_time_sum_ms BIGINT DEFAULT 0,
		response_time_count BIGINT DEFAULT 0,
		bytes_total BIGINT DEFAULT 0,
		input_tokens_total BIGINT DEFAULT 0,
		output_tokens_total BIGINT DEFAULT 0,
		first_byte_time_sum_ms BIGINT DEFAULT 0,
		stream_duration_sum_ms BIGINT DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		KEY idx_metrics_raw_account_node_time (account_id, node_id, ts),
		KEY idx_metrics_raw_time (ts)
	)`

	createHourly := `CREATE TABLE IF NOT EXISTS node_metrics_hourly (
		account_id VARCHAR(64) NOT NULL,
		node_id VARCHAR(64) NOT NULL,
		bucket_start DATETIME NOT NULL,
		requests_total BIGINT DEFAULT 0,
		requests_success BIGINT DEFAULT 0,
		requests_failed BIGINT DEFAULT 0,
		response_time_sum_ms BIGINT DEFAULT 0,
		response_time_count BIGINT DEFAULT 0,
		bytes_total BIGINT DEFAULT 0,
		input_tokens_total BIGINT DEFAULT 0,
		output_tokens_total BIGINT DEFAULT 0,
		first_byte_time_sum_ms BIGINT DEFAULT 0,
		stream_duration_sum_ms BIGINT DEFAULT 0,
		PRIMARY KEY (account_id, node_id, bucket_start),
		KEY idx_metrics_hour_time (bucket_start)
	)`

	createDaily := `CREATE TABLE IF NOT EXISTS node_metrics_daily (
		account_id VARCHAR(64) NOT NULL,
		node_id VARCHAR(64) NOT NULL,
		bucket_start DATETIME NOT NULL,
		requests_total BIGINT DEFAULT 0,
		requests_success BIGINT DEFAULT 0,
		requests_failed BIGINT DEFAULT 0,
		response_time_sum_ms BIGINT DEFAULT 0,
		response_time_count BIGINT DEFAULT 0,
		bytes_total BIGINT DEFAULT 0,
		input_tokens_total BIGINT DEFAULT 0,
		output_tokens_total BIGINT DEFAULT 0,
		first_byte_time_sum_ms BIGINT DEFAULT 0,
		stream_duration_sum_ms BIGINT DEFAULT 0,
		PRIMARY KEY (account_id, node_id, bucket_start),
		KEY idx_metrics_day_time (bucket_start)
	)`

	createMonthly := `CREATE TABLE IF NOT EXISTS node_metrics_monthly (
		account_id VARCHAR(64) NOT NULL,
		node_id VARCHAR(64) NOT NULL,
		bucket_start DATETIME NOT NULL,
		requests_total BIGINT DEFAULT 0,
		requests_success BIGINT DEFAULT 0,
		requests_failed BIGINT DEFAULT 0,
		response_time_sum_ms BIGINT DEFAULT 0,
		response_time_count BIGINT DEFAULT 0,
		bytes_total BIGINT DEFAULT 0,
		input_tokens_total BIGINT DEFAULT 0,
		output_tokens_total BIGINT DEFAULT 0,
		first_byte_time_sum_ms BIGINT DEFAULT 0,
		stream_duration_sum_ms BIGINT DEFAULT 0,
		PRIMARY KEY (account_id, node_id, bucket_start),
		KEY idx_metrics_month_time (bucket_start)
	)`

	stmts := []string{createRaw, createHourly, createDaily, createMonthly}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recreateConfigTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS config_new (
            account_id VARCHAR(64) PRIMARY KEY,
            retries INT DEFAULT 3,
            fail_limit INT DEFAULT 3,
            health_every_ms BIGINT DEFAULT 30000,
            active_node VARCHAR(64)
        )`); err != nil {
		return err
	}

	// 迁移旧配置（如果存在）。
	_, _ = s.db.ExecContext(ctx, `INSERT INTO config_new (account_id,retries,fail_limit,health_every_ms,active_node)
		SELECT '`+DefaultAccountID+`', retries, fail_limit, health_every_ms, active_node FROM config LIMIT 1`)

	if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS config`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE config_new RENAME TO config`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureNotificationTables(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS notification_channels (
		id VARCHAR(64) PRIMARY KEY,
		account_id VARCHAR(64) NOT NULL,
		channel_type VARCHAR(64) NOT NULL,
		name VARCHAR(255),
		config JSON,
		enabled BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		KEY idx_notification_channels_account (account_id)
	)`); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS notification_subscriptions (
		id VARCHAR(64) PRIMARY KEY,
		account_id VARCHAR(64) NOT NULL,
		channel_id VARCHAR(64) NOT NULL,
		event_type VARCHAR(128) NOT NULL,
		enabled BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE KEY uniq_subscription (account_id, channel_id, event_type),
		KEY idx_subscription_account_event (account_id, event_type),
		FOREIGN KEY (channel_id) REFERENCES notification_channels(id) ON DELETE CASCADE
	)`); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS notification_history (
		id VARCHAR(64) PRIMARY KEY,
		account_id VARCHAR(64) NOT NULL,
		channel_id VARCHAR(64) NOT NULL,
		event_type VARCHAR(128) NOT NULL,
		title VARCHAR(255),
		content TEXT,
		status VARCHAR(32) NOT NULL,
		error TEXT,
		sent_at TIMESTAMP NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		KEY idx_history_account_event (account_id, event_type),
        KEY idx_history_channel (channel_id)
	)`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureMonitorSharesTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := `CREATE TABLE IF NOT EXISTS monitor_shares (
		id VARCHAR(64) PRIMARY KEY,
		account_id VARCHAR(64) NOT NULL,
		token VARCHAR(64) UNIQUE NOT NULL,
		expire_at TIMESTAMP NULL,
		created_by VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		revoked BOOLEAN NOT NULL DEFAULT FALSE,
		revoked_at TIMESTAMP NULL,
		INDEX idx_account_id (account_id),
		INDEX idx_token (token),
		INDEX idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

func (s *Store) ensureDefaultAccount(ctx context.Context) error {
	// 默认账号自动创建已禁用，保留函数以兼容旧调用。
	return nil
}

// migrateConfigToSettings 将旧 config 表的数据迁移到新的 settings 表。
//
// 迁移规则：
//   - system 级别：取 config 第一条记录的 retries/fail_limit/health_every_ms 分别写入
//     proxy.retry_max、health.fail_threshold、health.check_interval_sec。
//   - account 级别：为每个账号写入 node.active_node。
//   - 使用 INSERT IGNORE 保证幂等，不覆盖已存在的 settings。
//   - 当 config 表不存在或无数据时直接跳过。
func (s *Store) migrateConfigToSettings(ctx context.Context) error {
	log.Printf("[migration] start config->settings migration")

	// 1) 检查 config 表是否存在。
	configExists, err := s.tableExists(ctx, "config")
	if err != nil {
		log.Printf("[migration] check config table failed: %v", err)
		return err
	}
	if !configExists {
		log.Printf("[migration] skip: config table not found")
		return nil
	}

	// 2) 读取所有 config 记录。
	qctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(qctx, `SELECT account_id, retries, fail_limit, health_every_ms, active_node FROM config ORDER BY account_id ASC`)
	if err != nil {
		log.Printf("[migration] query config failed: %v", err)
		return err
	}
	defer rows.Close()

	type legacyConfig struct {
		AccountID     string
		Retries       int
		FailLimit     int
		HealthEveryMs int64
		ActiveNode    sql.NullString
	}

	var configs []legacyConfig
	for rows.Next() {
		var cfg legacyConfig
		if err := rows.Scan(&cfg.AccountID, &cfg.Retries, &cfg.FailLimit, &cfg.HealthEveryMs, &cfg.ActiveNode); err != nil {
			log.Printf("[migration] scan config row failed: %v", err)
			return err
		}
		cfg.AccountID = normalizeAccount(cfg.AccountID)
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[migration] iterate config failed: %v", err)
		return err
	}
	if len(configs) == 0 {
		log.Printf("[migration] skip: config table empty")
		return nil
	}

	log.Printf("[migration] loaded %d legacy config rows", len(configs))

	// 3) system 级别配置使用第一条记录。
	sysCfg := configs[0]
	if err := s.insertSettingIfMissing(ctx, "proxy.retry_max", "system", nil, sysCfg.Retries, "number", "performance"); err != nil {
		return err
	}
	if err := s.insertSettingIfMissing(ctx, "health.fail_threshold", "system", nil, sysCfg.FailLimit, "number", "health"); err != nil {
		return err
	}
	if err := s.insertSettingIfMissing(ctx, "health.check_interval_sec", "system", nil, sysCfg.HealthEveryMs/1000, "number", "health"); err != nil {
		return err
	}
	log.Printf("[migration] system settings migrated from account %s", sysCfg.AccountID)

	// 4) account 级别 active_node。
	var accountInserted int
	for _, cfg := range configs {
		accountID := normalizeAccount(cfg.AccountID)
		var activeValue any
		if cfg.ActiveNode.Valid {
			activeValue = cfg.ActiveNode.String
		} else {
			activeValue = ""
		}
		if err := s.insertSettingIfMissing(ctx, "node.active_node", "account", accountID, activeValue, "string", "performance"); err != nil {
			return err
		}
		accountInserted++
	}
	log.Printf("[migration] account active_node migrated: %d rows", accountInserted)

	log.Printf("[migration] config->settings migration finished")
	return nil
}

// insertSettingIfMissing 使用 INSERT IGNORE 写入 settings，保持幂等。
func (s *Store) insertSettingIfMissing(ctx context.Context, key, scope string, account interface{}, value any, dataType, category string) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ictx, cancel := withTimeout(ctx)
	defer cancel()
	_, err = s.db.ExecContext(ictx, "INSERT IGNORE INTO settings (`key`, scope, account_id, value, data_type, category, is_secret, version) VALUES (?,?,?,?,?,?,FALSE,1)",
		key, scope, account, body, dataType, category)
	return err
}

func (s *Store) columnExists(ctx context.Context, table, column string) (bool, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) > 0
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?
	`, table, column)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) indexExists(ctx context.Context, table, index string) (bool, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) > 0
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND INDEX_NAME = ?
	`, table, index)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) tableExists(ctx context.Context, table string) (bool, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) > 0
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
	`, table)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
