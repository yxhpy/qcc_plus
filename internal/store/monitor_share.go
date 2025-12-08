package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// CreateMonitorShare 创建分享链接
func (s *Store) CreateMonitorShare(ctx context.Context, rec MonitorShareRecord) error {
	if rec.ID == "" || rec.Token == "" {
		return errors.New("id and token are required")
	}
	if rec.AccountID == "" {
		return errors.New("account_id required")
	}
	if rec.CreatedBy == "" {
		return errors.New("created_by required")
	}
	rec.AccountID = normalizeAccount(rec.AccountID)
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	} else {
		rec.CreatedAt = rec.CreatedAt.UTC()
	}
	var expire interface{}
	if !rec.ExpireAt.IsZero() {
		rec.ExpireAt = rec.ExpireAt.UTC()
		expire = rec.ExpireAt
	}
	var revokedAt interface{}
	if rec.RevokedAt != nil && !rec.RevokedAt.IsZero() {
		t := rec.RevokedAt.UTC()
		rec.RevokedAt = &t
		revokedAt = t
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO monitor_shares (id,account_id,token,expire_at,created_by,created_at,revoked,revoked_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		rec.ID, rec.AccountID, rec.Token, expire, rec.CreatedBy, rec.CreatedAt, rec.Revoked, revokedAt)
	return err
}

// GetMonitorShareByToken 根据 token 获取分享链接（验证有效性）
func (s *Store) GetMonitorShareByToken(ctx context.Context, token string) (*MonitorShareRecord, error) {
	if token == "" {
		return nil, errors.New("token required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var (
		rec       MonitorShareRecord
		expire    sql.NullTime
		revokedAt sql.NullTime
	)
	// 使用 Go 时间比较替代 UTC_TIMESTAMP()，兼容 MySQL 和 SQLite
	err := s.db.QueryRowContext(ctx, `SELECT id,account_id,token,expire_at,created_by,created_at,revoked,revoked_at
		FROM monitor_shares
		WHERE token=? AND revoked=FALSE AND (expire_at IS NULL OR expire_at>?)`,
		token, time.Now().UTC()).Scan(&rec.ID, &rec.AccountID, &rec.Token, &expire, &rec.CreatedBy, &rec.CreatedAt, &rec.Revoked, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if expire.Valid {
		rec.ExpireAt = expire.Time
	} else {
		rec.ExpireAt = time.Time{}
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		rec.RevokedAt = &t
	}
	rec.CreatedAt = rec.CreatedAt.UTC()
	rec.AccountID = normalizeAccount(rec.AccountID)
	return &rec, nil
}

// GetMonitorShareByID 获取分享记录（不校验有效性，用于管理操作）
func (s *Store) GetMonitorShareByID(ctx context.Context, id string) (*MonitorShareRecord, error) {
	if id == "" {
		return nil, errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var (
		rec       MonitorShareRecord
		expire    sql.NullTime
		revokedAt sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `SELECT id,account_id,token,expire_at,created_by,created_at,revoked,revoked_at
		FROM monitor_shares WHERE id=?`, id).
		Scan(&rec.ID, &rec.AccountID, &rec.Token, &expire, &rec.CreatedBy, &rec.CreatedAt, &rec.Revoked, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if expire.Valid {
		rec.ExpireAt = expire.Time
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		rec.RevokedAt = &t
	}
	rec.CreatedAt = rec.CreatedAt.UTC()
	rec.AccountID = normalizeAccount(rec.AccountID)
	return &rec, nil
}

// ListMonitorShares 列出分享链接
func (s *Store) ListMonitorShares(ctx context.Context, params QueryMonitorSharesParams) ([]MonitorShareRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	query := `SELECT id,account_id,token,expire_at,created_by,created_at,revoked,revoked_at FROM monitor_shares`
	conds := make([]string, 0, 2)
	args := make([]interface{}, 0, 4)
	if params.AccountID != "" {
		conds = append(conds, "account_id=?")
		args = append(args, normalizeAccount(params.AccountID))
	}
	if !params.IncludeRevoked {
		conds = append(conds, "revoked=FALSE")
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if params.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, params.Limit)
		if params.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, params.Offset)
		}
	} else if params.Offset > 0 {
		// MySQL 需要 LIMIT 才能使用 OFFSET。
		// SQLite 使用 -1 表示无限制，MySQL 使用最大值。
		if s.IsSQLite() {
			query += " LIMIT -1 OFFSET ?"
		} else {
			query += " LIMIT 18446744073709551615 OFFSET ?"
		}
		args = append(args, params.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []MonitorShareRecord
	for rows.Next() {
		var (
			rec       MonitorShareRecord
			expire    sql.NullTime
			revokedAt sql.NullTime
		)
		if err := rows.Scan(&rec.ID, &rec.AccountID, &rec.Token, &expire, &rec.CreatedBy, &rec.CreatedAt, &rec.Revoked, &revokedAt); err != nil {
			return nil, err
		}
		if expire.Valid {
			rec.ExpireAt = expire.Time
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			rec.RevokedAt = &t
		}
		rec.CreatedAt = rec.CreatedAt.UTC()
		rec.AccountID = normalizeAccount(rec.AccountID)
		res = append(res, rec)
	}
	return res, nil
}

// RevokeMonitorShare 撤销分享链接
func (s *Store) RevokeMonitorShare(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	res, err := s.db.ExecContext(ctx, `UPDATE monitor_shares SET revoked=TRUE, revoked_at=IFNULL(revoked_at, ?) WHERE id=?`, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		if _, err := s.GetMonitorShareByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	return nil
}

// DeleteMonitorShare 删除分享链接（物理删除）
func (s *Store) DeleteMonitorShare(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitor_shares WHERE id=?`, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}
