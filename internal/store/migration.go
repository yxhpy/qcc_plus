package store

import "context"

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

func (s *Store) ensureDefaultAccount(ctx context.Context) error {
	// 默认账号自动创建已禁用，保留函数以兼容旧调用。
	return nil
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
