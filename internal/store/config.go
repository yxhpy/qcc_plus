package store

import (
	"context"
	"time"
)

func (s *Store) LoadAllByAccount(ctx context.Context, accountID string) (records []NodeRecord, cfg Config, activeID string, err error) {
	accountID = normalizeAccount(accountID)
	cfg = Config{Retries: 3, FailLimit: 3, HealthEvery: 30 * time.Second}

	if err = s.ensureConfigRow(ctx, accountID); err != nil {
		return
	}

	cctx, cancel := withTimeout(ctx)
	defer cancel()
	row := s.db.QueryRowContext(cctx, `SELECT retries, fail_limit, health_every_ms, active_node FROM config WHERE account_id=?`, accountID)
	var healthMs int64
	if err = row.Scan(&cfg.Retries, &cfg.FailLimit, &healthMs, &activeID); err != nil {
		return
	}
	cfg.HealthEvery = time.Duration(healthMs) * time.Millisecond

	nctx, ncancel := withTimeout(ctx)
	defer ncancel()
	rows, err := s.queryNodesByAccount(nctx, accountID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var r NodeRecord
		r, err = scanNodeRecord(rows)
		if err != nil {
			return
		}
		records = append(records, r)
	}
	return
}

// LoadConfigByAccount 获取账号配置与活跃节点。
func (s *Store) LoadConfigByAccount(ctx context.Context, accountID string) (Config, string, error) {
	accountID = normalizeAccount(accountID)
	var cfg Config
	if err := s.ensureConfigRow(ctx, accountID); err != nil {
		return cfg, "", err
	}
	cctx, cancel := withTimeout(ctx)
	defer cancel()
	row := s.db.QueryRowContext(cctx, `SELECT retries, fail_limit, health_every_ms, active_node FROM config WHERE account_id=?`, accountID)
	var healthMs int64
	var active string
	if err := row.Scan(&cfg.Retries, &cfg.FailLimit, &healthMs, &active); err != nil {
		return cfg, "", err
	}
	cfg.HealthEvery = time.Duration(healthMs) * time.Millisecond
	return cfg, active, nil
}

func (s *Store) SetActive(ctx context.Context, accountID, id string) error {
	accountID = normalizeAccount(accountID)
	if err := s.ensureConfigRow(ctx, accountID); err != nil {
		return err
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `UPDATE config SET active_node=? WHERE account_id=?`, id, accountID)
	return err
}

func (s *Store) UpdateConfig(ctx context.Context, accountID string, cfg Config, active string) error {
	accountID = normalizeAccount(accountID)
	if err := s.ensureConfigRow(ctx, accountID); err != nil {
		return err
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `UPDATE config SET retries=?, fail_limit=?, health_every_ms=?, active_node=? WHERE account_id=?`,
		cfg.Retries, cfg.FailLimit, cfg.HealthEvery.Milliseconds(), active, accountID)
	return err
}

func (s *Store) ensureConfigRow(ctx context.Context, accountID string) error {
	accountID = normalizeAccount(accountID)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var stmt string
	if s.IsSQLite() {
		stmt = `INSERT OR IGNORE INTO config (account_id,retries,fail_limit,health_every_ms,active_node) VALUES (?,?,?,?,?)`
	} else {
		stmt = `INSERT IGNORE INTO config (account_id,retries,fail_limit,health_every_ms,active_node) VALUES (?,?,?,?,?)`
	}
	_, err := s.db.ExecContext(ctx, stmt, accountID, 3, 3, 30000, "")
	return err
}
