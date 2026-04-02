package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) ensureFailedModelsTable(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var stmt string
	if s.IsSQLite() {
		stmt = `CREATE TABLE IF NOT EXISTS failed_models (
			node_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			error TEXT DEFAULT '',
			failed_at DATETIME NOT NULL,
			last_check DATETIME DEFAULT NULL,
			check_count INTEGER DEFAULT 0,
			non_recoverable INTEGER DEFAULT 0,
			PRIMARY KEY (node_id, model_id)
		)`
	} else {
		stmt = `CREATE TABLE IF NOT EXISTS failed_models (
			node_id VARCHAR(64) NOT NULL,
			model_id VARCHAR(255) NOT NULL,
			account_id VARCHAR(64) NOT NULL,
			error TEXT,
			failed_at DATETIME NOT NULL,
			last_check DATETIME DEFAULT NULL,
			check_count INT DEFAULT 0,
			non_recoverable BOOLEAN DEFAULT FALSE,
			PRIMARY KEY (node_id, model_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	}
	_, err := s.db.ExecContext(ctx, stmt)
	if err != nil {
		return err
	}

	hasCol, err := s.columnExists(ctx, "failed_models", "non_recoverable")
	if err != nil {
		return err
	}
	if !hasCol {
		alter := "ALTER TABLE failed_models ADD COLUMN non_recoverable INTEGER DEFAULT 0"
		if !s.IsSQLite() {
			alter = "ALTER TABLE failed_models ADD COLUMN non_recoverable BOOLEAN DEFAULT FALSE"
		}
		if _, err := s.db.ExecContext(ctx, alter); err != nil {
			return err
		}
	}
	return nil
}

// UpsertFailedModel 插入或更新失败模型记录。
func (s *Store) UpsertFailedModel(ctx context.Context, r FailedModelRecord) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if r.NodeID == "" || r.ModelID == "" {
		return errors.New("node_id and model_id required")
	}
	r.AccountID = normalizeAccount(r.AccountID)
	if r.FailedAt.IsZero() {
		r.FailedAt = time.Now().UTC()
	} else {
		r.FailedAt = r.FailedAt.UTC()
	}

	lastCheck := sql.NullTime{}
	if !r.LastCheck.IsZero() {
		lastCheck.Valid = true
		lastCheck.Time = r.LastCheck.UTC()
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var err error
	if s.IsSQLite() {
		_, err = s.db.ExecContext(ctx, `INSERT INTO failed_models (node_id, model_id, account_id, error, failed_at, last_check, check_count, non_recoverable)
			VALUES (?,?,?,?,?,?,?,?)
			ON CONFLICT(node_id, model_id) DO UPDATE SET
				error=excluded.error,
				last_check=excluded.last_check,
				check_count=excluded.check_count,
				non_recoverable=excluded.non_recoverable`,
			r.NodeID, r.ModelID, r.AccountID, r.Error, r.FailedAt, lastCheck, r.CheckCount, r.NonRecoverable)
	} else {
		_, err = s.db.ExecContext(ctx, `INSERT INTO failed_models (node_id, model_id, account_id, error, failed_at, last_check, check_count, non_recoverable)
			VALUES (?,?,?,?,?,?,?,?)
			ON DUPLICATE KEY UPDATE
				error=VALUES(error),
				last_check=VALUES(last_check),
				check_count=VALUES(check_count),
				non_recoverable=VALUES(non_recoverable)`,
			r.NodeID, r.ModelID, r.AccountID, r.Error, r.FailedAt, lastCheck, r.CheckCount, r.NonRecoverable)
	}
	return err
}

// DeleteFailedModel 删除指定节点上指定模型的失败记录（模型已恢复）。
func (s *Store) DeleteFailedModel(ctx context.Context, nodeID, modelID string) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `DELETE FROM failed_models WHERE node_id=? AND model_id=?`, nodeID, modelID)
	return err
}

// DeleteFailedModelsByNode 删除指定节点上所有失败模型记录（节点整体恢复时调用）。
func (s *Store) DeleteFailedModelsByNode(ctx context.Context, nodeID string) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `DELETE FROM failed_models WHERE node_id=?`, nodeID)
	return err
}

// ListAllFailedModels 加载所有失败模型记录（启动时恢复状态用）。
func (s *Store) ListAllFailedModels(ctx context.Context) ([]FailedModelRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `SELECT node_id, model_id, account_id, error, failed_at, last_check, check_count, non_recoverable FROM failed_models`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []FailedModelRecord
	for rows.Next() {
		var r FailedModelRecord
		var lastCheck sql.NullTime
		if err := rows.Scan(&r.NodeID, &r.ModelID, &r.AccountID, &r.Error, &r.FailedAt, &lastCheck, &r.CheckCount, &r.NonRecoverable); err != nil {
			return nil, err
		}
		if lastCheck.Valid {
			r.LastCheck = lastCheck.Time
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// SetFailedModelNonRecoverable 更新失败模型记录的不可恢复标记。
func (s *Store) SetFailedModelNonRecoverable(ctx context.Context, nodeID, modelID string, nonRecoverable bool) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `UPDATE failed_models SET non_recoverable=? WHERE node_id=? AND model_id=?`, nonRecoverable, nodeID, modelID)
	return err
}
