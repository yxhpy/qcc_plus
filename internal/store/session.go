package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// UpsertSession 创建或更新会话记录。
func (s *Store) UpsertSession(ctx context.Context, session SessionRecord) error {
	if session.Token == "" || session.AccountID == "" {
		return errors.New("token and account_id are required")
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var stmt string
	if s.IsSQLite() {
		stmt = `INSERT INTO sessions (token, account_id, is_admin, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(token) DO UPDATE SET
				account_id=excluded.account_id,
				is_admin=excluded.is_admin,
				created_at=excluded.created_at,
				expires_at=excluded.expires_at`
	} else {
		stmt = `INSERT INTO sessions (token, account_id, is_admin, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				account_id=VALUES(account_id),
				is_admin=VALUES(is_admin),
				created_at=VALUES(created_at),
				expires_at=VALUES(expires_at)`
	}

	_, err := s.db.ExecContext(ctx, stmt, session.Token, normalizeAccount(session.AccountID), session.IsAdmin, session.CreatedAt, session.ExpiresAt)
	return err
}

// GetSessionByToken 根据 token 获取会话。
func (s *Store) GetSessionByToken(ctx context.Context, token string) (*SessionRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var session SessionRecord
	err := s.db.QueryRowContext(ctx, `SELECT token, account_id, is_admin, created_at, expires_at FROM sessions WHERE token=?`, token).
		Scan(&session.Token, &session.AccountID, &session.IsAdmin, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &session, nil
}

// DeleteSession 删除指定 token 的会话。
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token=?`, token)
	return err
}

// DeleteExpiredSessions 清理到给定时间点之前已过期的会话。
func (s *Store) DeleteExpiredSessions(ctx context.Context, expiredBefore time.Time) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=?`, expiredBefore)
	return err
}
