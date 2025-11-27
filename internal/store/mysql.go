package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ensureSettingsTable 创建统一配置表。
func (s *Store) ensureSettingsTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	stmt := "CREATE TABLE IF NOT EXISTS settings (" +
		"  id BIGINT AUTO_INCREMENT PRIMARY KEY," +
		"  `key` VARCHAR(128) NOT NULL COMMENT '配置键'," +
		"  scope ENUM('system', 'account', 'user') NOT NULL DEFAULT 'system' COMMENT '作用域'," +
		"  account_id VARCHAR(64) NULL COMMENT '账号ID'," +
		"  value JSON NOT NULL COMMENT '配置值'," +
		"  data_type VARCHAR(32) NOT NULL DEFAULT 'string' COMMENT '数据类型: string/number/boolean/object/array/duration'," +
		"  category VARCHAR(64) NOT NULL DEFAULT 'general' COMMENT '分类: monitor/health/performance/notification/security'," +
		"  description TEXT NULL COMMENT '配置说明'," +
		"  is_secret BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否敏感配置'," +
		"  version INT NOT NULL DEFAULT 1 COMMENT '版本号(乐观锁)'," +
		"  updated_by VARCHAR(64) NULL COMMENT '最后修改人'," +
		"  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP," +
		"  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP," +
		"  UNIQUE KEY uk_scope_key_account (scope, `key`, account_id)," +
		"  INDEX idx_category (category)," +
		"  INDEX idx_updated_at (updated_at)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统一配置表';"
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

// SeedDefaultSettings 插入默认配置（若不存在）。
func (s *Store) SeedDefaultSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	defaults := []Setting{
		{Key: "monitor.refresh_interval_ms", Scope: "system", Value: 30000, DataType: "number", Category: "monitor", Description: strPtr("监控大屏刷新间隔（毫秒）")},
		{Key: "monitor.error_display", Scope: "system", Value: "icon", DataType: "string", Category: "monitor", Description: strPtr("错误显示方式：icon/inline")},
		{Key: "monitor.show_node_stats", Scope: "system", Value: map[string]bool{"showProxy": true, "showHealth": true}, DataType: "object", Category: "monitor", Description: strPtr("节点统计栏显示配置")},
		{Key: "health.check_interval_sec", Scope: "system", Value: 30, DataType: "number", Category: "health", Description: strPtr("健康检查间隔（秒）")},
		{Key: "health.fail_threshold", Scope: "system", Value: 3, DataType: "number", Category: "health", Description: strPtr("失败阈值")},
		{Key: "proxy.retry_max", Scope: "system", Value: 3, DataType: "number", Category: "performance", Description: strPtr("最大重试次数")},
	}

	for _, d := range defaults {
		body, err := json.Marshal(d.Value)
		if err != nil {
			return fmt.Errorf("marshal default setting %s: %w", d.Key, err)
		}
		dataType := d.DataType
		if dataType == "" {
			dataType = "string"
		}
		category := d.Category
		if category == "" {
			category = "general"
		}
		if _, err := s.db.ExecContext(ctx, "INSERT IGNORE INTO settings (`key`, scope, account_id, value, data_type, category, description, is_secret, version) VALUES (?,?,?,?,?,?,?,?,1)",
			d.Key, d.Scope, nil, body, dataType, category, nullOrStringPtr(d.Description), d.IsSecret); err != nil {
			return err
		}
	}
	return nil
}

// ListSettings 获取配置列表，支持 scope/category/account_id 过滤。
func (s *Store) ListSettings(scope, category, accountID string) ([]Setting, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var (
		sb     strings.Builder
		args   []any
		result []Setting
	)

	sb.WriteString("SELECT id,`key`,scope,account_id,value,data_type,category,description,is_secret,version,updated_by,updated_at,created_at FROM settings WHERE 1=1")
	if scope != "" {
		sb.WriteString(" AND scope=?")
		args = append(args, scope)
	}
	if category != "" {
		sb.WriteString(" AND category=?")
		args = append(args, category)
	}
	if accountID != "" {
		sb.WriteString(" AND account_id=?")
		args = append(args, accountID)
	}
	sb.WriteString(" ORDER BY updated_at DESC, id DESC")

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		setting, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *setting)
	}
	return result, nil
}

// GetSetting 获取单个配置。
func (s *Store) GetSetting(key, scope, accountID string) (*Setting, error) {
	if key == "" {
		return nil, errors.New("key required")
	}
	scope = normalizeScope(scope)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	row := s.db.QueryRowContext(ctx, "SELECT id,`key`,scope,account_id,value,data_type,category,description,is_secret,version,updated_by,updated_at,created_at FROM settings WHERE `key`=? AND scope=? AND account_id <=> ? LIMIT 1",
		key, scope, accountArg(accountID))
	return scanSetting(row)
}

// UpsertSetting 创建或更新配置（不检查版本，自动递增版本号）。
func (s *Store) UpsertSetting(setting *Setting) error {
	if setting == nil {
		return errors.New("setting is nil")
	}
	if setting.Key == "" {
		return errors.New("key required")
	}
	normalizeSetting(setting)
	body, err := json.Marshal(setting.Value)
	if err != nil {
		return fmt.Errorf("marshal setting value: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	_, err = s.db.ExecContext(ctx, "INSERT INTO settings (`key`, scope, account_id, value, data_type, category, description, is_secret, version, updated_by) "+
		"VALUES (?,?,?,?,?,?,?,?,1,?) "+
		"ON DUPLICATE KEY UPDATE value=VALUES(value), data_type=VALUES(data_type), category=VALUES(category), description=VALUES(description), is_secret=VALUES(is_secret), updated_by=VALUES(updated_by), version=version+1",
		setting.Key, setting.Scope, accountArgPtr(setting.AccountID), body, setting.DataType, setting.Category, nullOrStringPtr(setting.Description), setting.IsSecret, nullOrStringPtr(setting.UpdatedBy))
	if err != nil {
		return err
	}
	updated, err := s.GetSetting(setting.Key, setting.Scope, deref(setting.AccountID))
	if err == nil && updated != nil {
		setting.Version = updated.Version
		setting.UpdatedAt = updated.UpdatedAt
		setting.CreatedAt = updated.CreatedAt
	}
	return nil
}

// UpdateSetting 带版本检查的更新，版本不匹配返回 ErrVersionConflict。
func (s *Store) UpdateSetting(setting *Setting) error {
	if setting == nil {
		return errors.New("setting is nil")
	}
	if setting.Key == "" {
		return errors.New("key required")
	}
	if setting.Version <= 0 {
		return errors.New("version required")
	}
	normalizeSetting(setting)
	body, err := json.Marshal(setting.Value)
	if err != nil {
		return fmt.Errorf("marshal setting value: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	res, err := s.db.ExecContext(ctx, "UPDATE settings SET value=?, data_type=?, category=?, description=?, is_secret=?, updated_by=?, version=version+1 "+
		"WHERE `key`=? AND scope=? AND account_id <=> ? AND version=?",
		body, setting.DataType, setting.Category, nullOrStringPtr(setting.Description), setting.IsSecret, nullOrStringPtr(setting.UpdatedBy),
		setting.Key, setting.Scope, accountArgPtr(setting.AccountID), setting.Version)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		exists, err := s.settingExists(ctx, setting.Key, setting.Scope, accountArgPtr(setting.AccountID))
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return ErrVersionConflict
	}
	updated, err := s.GetSetting(setting.Key, setting.Scope, deref(setting.AccountID))
	if err == nil && updated != nil {
		setting.Version = updated.Version
		setting.UpdatedAt = updated.UpdatedAt
		setting.CreatedAt = updated.CreatedAt
	}
	return nil
}

// DeleteSetting 删除配置。
func (s *Store) DeleteSetting(key, scope, accountID string) error {
	if key == "" {
		return errors.New("key required")
	}
	scope = normalizeScope(scope)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	res, err := s.db.ExecContext(ctx, "DELETE FROM settings WHERE `key`=? AND scope=? AND account_id <=> ?", key, scope, accountArg(accountID))
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}

// BatchUpdateSettings 批量更新（事务）。
func (s *Store) BatchUpdateSettings(settings []Setting) error {
	if len(settings) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for i := range settings {
		normalizeSetting(&settings[i])
		body, err := json.Marshal(settings[i].Value)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("marshal setting %s: %w", settings[i].Key, err)
		}
		if settings[i].Version > 0 {
			res, err := tx.ExecContext(ctx, "UPDATE settings SET value=?, data_type=?, category=?, description=?, is_secret=?, updated_by=?, version=version+1 "+
				"WHERE `key`=? AND scope=? AND account_id <=> ? AND version=?",
				body, settings[i].DataType, settings[i].Category, nullOrStringPtr(settings[i].Description), settings[i].IsSecret, nullOrStringPtr(settings[i].UpdatedBy),
				settings[i].Key, settings[i].Scope, accountArgPtr(settings[i].AccountID), settings[i].Version)
			if err != nil {
				tx.Rollback()
				return err
			}
			if rows, _ := res.RowsAffected(); rows == 0 {
				exists, err := s.settingExistsTx(ctx, tx, settings[i].Key, settings[i].Scope, accountArgPtr(settings[i].AccountID))
				if err != nil {
					tx.Rollback()
					return err
				}
				if !exists {
					tx.Rollback()
					return ErrNotFound
				}
				tx.Rollback()
				return ErrVersionConflict
			}
		} else {
			if _, err := tx.ExecContext(ctx, "INSERT INTO settings (`key`, scope, account_id, value, data_type, category, description, is_secret, version, updated_by) "+
				"VALUES (?,?,?,?,?,?,?,?,1,?) "+
				"ON DUPLICATE KEY UPDATE value=VALUES(value), data_type=VALUES(data_type), category=VALUES(category), description=VALUES(description), is_secret=VALUES(is_secret), updated_by=VALUES(updated_by), version=version+1",
				settings[i].Key, settings[i].Scope, accountArgPtr(settings[i].AccountID), body, settings[i].DataType, settings[i].Category, nullOrStringPtr(settings[i].Description), settings[i].IsSecret, nullOrStringPtr(settings[i].UpdatedBy)); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetGlobalVersion 返回全局最大版本号。
func (s *Store) GetGlobalVersion() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	var version sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(version) FROM settings`).Scan(&version)
	if err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return version.Int64, nil
}

// ---- helpers ----

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSetting(scanner rowScanner) (*Setting, error) {
	var (
		s         Setting
		accountID sql.NullString
		desc      sql.NullString
		updatedBy sql.NullString
		raw       json.RawMessage
	)
	if err := scanner.Scan(&s.ID, &s.Key, &s.Scope, &accountID, &raw, &s.DataType, &s.Category, &desc, &s.IsSecret, &s.Version, &updatedBy, &s.UpdatedAt, &s.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if accountID.Valid {
		val := accountID.String
		s.AccountID = &val
	}
	if desc.Valid {
		val := desc.String
		s.Description = &val
	}
	if updatedBy.Valid {
		val := updatedBy.String
		s.UpdatedBy = &val
	}
	if len(raw) > 0 {
		var val any
		if err := json.Unmarshal(raw, &val); err == nil {
			s.Value = val
		} else {
			// 保底：返回原始 JSON 字符串，避免因解析失败而丢失数据。
			s.Value = string(raw)
		}
	}
	return &s, nil
}

func accountArg(accountID string) interface{} {
	if accountID == "" {
		return nil
	}
	return accountID
}

func accountArgPtr(accountID *string) interface{} {
	if accountID == nil {
		return nil
	}
	return accountArg(*accountID)
}

func nullOrStringPtr(v *string) interface{} {
	if v == nil {
		return nil
	}
	if *v == "" {
		return nil
	}
	return *v
}

func strPtr(s string) *string {
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func normalizeScope(scope string) string {
	switch strings.ToLower(scope) {
	case "account":
		return "account"
	case "user":
		return "user"
	default:
		return "system"
	}
}

func normalizeSetting(s *Setting) {
	s.Scope = normalizeScope(s.Scope)
	if s.DataType == "" {
		s.DataType = "string"
	}
	if s.Category == "" {
		s.Category = "general"
	}
}

func (s *Store) settingExists(ctx context.Context, key, scope string, account interface{}) (bool, error) {
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(1) > 0 FROM settings WHERE `key`=? AND scope=? AND account_id <=> ?", key, scope, account)
	var ok bool
	if err := row.Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (s *Store) settingExistsTx(ctx context.Context, tx *sql.Tx, key, scope string, account interface{}) (bool, error) {
	row := tx.QueryRowContext(ctx, "SELECT COUNT(1) > 0 FROM settings WHERE `key`=? AND scope=? AND account_id <=> ?", key, scope, account)
	var ok bool
	if err := row.Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}
