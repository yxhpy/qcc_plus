package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CreateAccount 新建账号。
func (s *Store) CreateAccount(ctx context.Context, a AccountRecord) error {
	if a.ID == "" || a.Name == "" {
		return errors.New("id and name are required")
	}
	a.ID = normalizeAccount(a.ID)
	now := time.Now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = now
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO accounts (id,name,password,proxy_api_key,is_admin,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
		a.ID, a.Name, nullOrString(a.Password), nullOrString(a.ProxyAPIKey), a.IsAdmin, a.CreatedAt, a.UpdatedAt)
	return err
}

// GetAccountByProxyKey 根据代理 API Key 获取账号。
func (s *Store) GetAccountByProxyKey(ctx context.Context, proxyKey string) (*AccountRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var (
		rec       AccountRecord
		passNull  sql.NullString
		proxyNull sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `SELECT id,name,password,proxy_api_key,is_admin,created_at,updated_at FROM accounts WHERE proxy_api_key=?`, proxyKey).
		Scan(&rec.ID, &rec.Name, &passNull, &proxyNull, &rec.IsAdmin, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rec.Password = passNull.String
	rec.ProxyAPIKey = proxyNull.String
	return &rec, nil
}

// GetAccountByID 按账号 ID 获取账号。
func (s *Store) GetAccountByID(ctx context.Context, id string) (*AccountRecord, error) {
	id = normalizeAccount(id)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var (
		rec       AccountRecord
		passNull  sql.NullString
		proxyNull sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `SELECT id,name,password,proxy_api_key,is_admin,created_at,updated_at FROM accounts WHERE id=?`, id).
		Scan(&rec.ID, &rec.Name, &passNull, &proxyNull, &rec.IsAdmin, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rec.Password = passNull.String
	rec.ProxyAPIKey = proxyNull.String
	return &rec, nil
}

// ListAccounts 返回所有账号。
func (s *Store) ListAccounts(ctx context.Context) ([]AccountRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,password,proxy_api_key,is_admin,created_at,updated_at FROM accounts ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []AccountRecord
	for rows.Next() {
		var (
			rec       AccountRecord
			passNull  sql.NullString
			proxyNull sql.NullString
		)
		if err := rows.Scan(&rec.ID, &rec.Name, &passNull, &proxyNull, &rec.IsAdmin, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		rec.Password = passNull.String
		rec.ProxyAPIKey = proxyNull.String
		res = append(res, rec)
	}
	return res, nil
}

// UpdateAccount 更新账号信息。
func (s *Store) UpdateAccount(ctx context.Context, a AccountRecord) error {
	if a.ID == "" {
		return errors.New("id required")
	}
	a.ID = normalizeAccount(a.ID)
	a.UpdatedAt = time.Now()
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	res, err := s.db.ExecContext(ctx, `UPDATE accounts SET name=?, password=?, proxy_api_key=?, is_admin=?, updated_at=? WHERE id=?`,
		a.Name, nullOrString(a.Password), nullOrString(a.ProxyAPIKey), a.IsAdmin, a.UpdatedAt, a.ID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	return err
}

// DeleteAccount 删除账号（同时清理其节点与配置）。
func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id required")
	}
	id = normalizeAccount(id)
	if id == DefaultAccountID {
		return errors.New("cannot delete default account")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE account_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM config WHERE account_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}
