package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const healthHistoryRetention = 30 * 24 * time.Hour

// InsertHealthCheck 插入健康检查记录。
func (s *Store) InsertHealthCheck(ctx context.Context, record *HealthCheckRecord) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if record == nil {
		return errors.New("record is nil")
	}
	record.AccountID = normalizeAccount(record.AccountID)
	if record.CheckMethod == "" {
		record.CheckMethod = "api"
	}
	if record.CheckTime.IsZero() {
		record.CheckTime = time.Now().UTC()
	} else {
		record.CheckTime = record.CheckTime.UTC()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	resp := sql.NullInt64{}
	if record.ResponseTimeMs >= 0 {
		resp.Valid = true
		resp.Int64 = int64(record.ResponseTimeMs)
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO health_check_history (
		account_id, node_id, check_time, success, response_time_ms, error_message, check_method, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		record.AccountID, record.NodeID, record.CheckTime, record.Success, resp, record.ErrorMessage, record.CheckMethod, record.CreatedAt)
	return err
}

// QueryHealthChecks 查询健康检查历史，返回最新的 limit 条记录，按时间正序排列（用于显示）。
func (s *Store) QueryHealthChecks(ctx context.Context, params QueryHealthCheckParams) ([]HealthCheckRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}
	if params.NodeID == "" {
		return nil, errors.New("node_id required")
	}
	params.AccountID = normalizeAccount(params.AccountID)

	now := time.Now().UTC()
	if params.To.IsZero() {
		params.To = now
	} else {
		params.To = params.To.UTC()
	}
	if params.From.IsZero() {
		params.From = params.To.Add(-24 * time.Hour)
	} else {
		params.From = params.From.UTC()
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 300
	} else if limit > 2000 {
		limit = 2000
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	// 使用子查询：先用 DESC 取最新的 N 条，再用 ASC 排序返回（正序显示）
	rows, err := s.db.QueryContext(ctx, `SELECT id, account_id, node_id, check_time, success, response_time_ms, error_message, check_method, created_at
		FROM (
			SELECT id, account_id, node_id, check_time, success, response_time_ms, error_message, check_method, created_at
			FROM health_check_history
			WHERE account_id=? AND node_id=? AND check_time >= ? AND check_time <= ?
			ORDER BY check_time DESC
			LIMIT ? OFFSET ?
		) AS latest
		ORDER BY check_time ASC`,
		params.AccountID, params.NodeID, params.From, params.To, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []HealthCheckRecord
	for rows.Next() {
		var rec HealthCheckRecord
		var resp sql.NullInt64
		if err := rows.Scan(&rec.ID, &rec.AccountID, &rec.NodeID, &rec.CheckTime, &rec.Success, &resp, &rec.ErrorMessage, &rec.CheckMethod, &rec.CreatedAt); err != nil {
			return nil, err
		}
		if resp.Valid {
			rec.ResponseTimeMs = int(resp.Int64)
		}
		res = append(res, rec)
	}
	return res, rows.Err()
}

// CountHealthChecks 统计指定条件的总记录数。
func (s *Store) CountHealthChecks(ctx context.Context, params QueryHealthCheckParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store not initialized")
	}
	if params.NodeID == "" {
		return 0, errors.New("node_id required")
	}
	params.AccountID = normalizeAccount(params.AccountID)

	if params.To.IsZero() {
		params.To = time.Now().UTC()
	} else {
		params.To = params.To.UTC()
	}
	if params.From.IsZero() {
		params.From = params.To.Add(-24 * time.Hour)
	} else {
		params.From = params.From.UTC()
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM health_check_history WHERE account_id=? AND node_id=? AND check_time >= ? AND check_time <= ?`,
		params.AccountID, params.NodeID, params.From, params.To)
	var total int64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// CleanupHealthChecks 清理早于 before 的记录；未传入时保留 30 天。
func (s *Store) CleanupHealthChecks(ctx context.Context, before time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	cutoff := before
	if cutoff.IsZero() {
		cutoff = time.Now().UTC().Add(-healthHistoryRetention)
	} else {
		cutoff = cutoff.UTC()
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `DELETE FROM health_check_history WHERE check_time < ?`, cutoff)
	return err
}
