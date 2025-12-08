package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CreateNotificationChannel 新建通知渠道。
func (s *Store) CreateNotificationChannel(ctx context.Context, rec NotificationChannelRecord) error {
	if rec.ID == "" || rec.AccountID == "" || rec.ChannelType == "" {
		return errors.New("id, account_id, channel_type are required")
	}
	now := time.Now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO notification_channels (id,account_id,channel_type,name,config,enabled,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		rec.ID, rec.AccountID, rec.ChannelType, rec.Name, rec.Config, rec.Enabled, rec.CreatedAt, rec.UpdatedAt)
	return err
}

// UpdateNotificationChannel 更新渠道配置。
func (s *Store) UpdateNotificationChannel(ctx context.Context, rec NotificationChannelRecord) error {
	if rec.ID == "" {
		return errors.New("id required")
	}
	rec.UpdatedAt = time.Now()
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	res, err := s.db.ExecContext(ctx, `UPDATE notification_channels SET name=?, config=?, enabled=?, updated_at=?, channel_type=?, account_id=? WHERE id=?`,
		rec.Name, rec.Config, rec.Enabled, rec.UpdatedAt, rec.ChannelType, rec.AccountID, rec.ID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetNotificationChannel 根据 ID 获取渠道。
func (s *Store) GetNotificationChannel(ctx context.Context, id string) (*NotificationChannelRecord, error) {
	if id == "" {
		return nil, errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var rec NotificationChannelRecord
	err := s.db.QueryRowContext(ctx, `SELECT id,account_id,channel_type,name,config,enabled,created_at,updated_at FROM notification_channels WHERE id=?`, id).
		Scan(&rec.ID, &rec.AccountID, &rec.ChannelType, &rec.Name, &rec.Config, &rec.Enabled, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// ListNotificationChannels 返回账号的所有渠道。
func (s *Store) ListNotificationChannels(ctx context.Context, accountID string) ([]NotificationChannelRecord, error) {
	accountID = normalizeAccount(accountID)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT id,account_id,channel_type,name,config,enabled,created_at,updated_at FROM notification_channels WHERE account_id=? ORDER BY created_at ASC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []NotificationChannelRecord
	for rows.Next() {
		var rec NotificationChannelRecord
		if err := rows.Scan(&rec.ID, &rec.AccountID, &rec.ChannelType, &rec.Name, &rec.Config, &rec.Enabled, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, rec)
	}
	return res, nil
}

// GetNotificationSubscription 根据 ID 获取订阅记录。
func (s *Store) GetNotificationSubscription(ctx context.Context, id string) (*NotificationSubscriptionRecord, error) {
	if id == "" {
		return nil, errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var rec NotificationSubscriptionRecord
	err := s.db.QueryRowContext(ctx, `SELECT id,account_id,channel_id,event_type,enabled,created_at,updated_at FROM notification_subscriptions WHERE id=?`, id).
		Scan(&rec.ID, &rec.AccountID, &rec.ChannelID, &rec.EventType, &rec.Enabled, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// ListNotificationSubscriptions 返回账号下的订阅列表，可选按 channel 过滤。
func (s *Store) ListNotificationSubscriptions(ctx context.Context, accountID, channelID string) ([]NotificationSubscriptionRecord, error) {
	accountID = normalizeAccount(accountID)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	query := `SELECT id,account_id,channel_id,event_type,enabled,created_at,updated_at FROM notification_subscriptions WHERE account_id=?`
	args := []interface{}{accountID}
	if channelID != "" {
		query += " AND channel_id=?"
		args = append(args, channelID)
	}
	query += " ORDER BY created_at ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []NotificationSubscriptionRecord
	for rows.Next() {
		var rec NotificationSubscriptionRecord
		if err := rows.Scan(&rec.ID, &rec.AccountID, &rec.ChannelID, &rec.EventType, &rec.Enabled, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, rec)
	}
	return res, nil
}

// DeleteNotificationSubscription 删除指定订阅。
func (s *Store) DeleteNotificationSubscription(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	res, err := s.db.ExecContext(ctx, `DELETE FROM notification_subscriptions WHERE id=?`, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteNotificationChannel 删除渠道及其订阅。
func (s *Store) DeleteNotificationChannel(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_subscriptions WHERE channel_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM notification_channels WHERE id=?`, id)
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

// UpsertNotificationSubscription 创建或更新订阅。
func (s *Store) UpsertNotificationSubscription(ctx context.Context, rec NotificationSubscriptionRecord) error {
	if rec.ID == "" {
		return errors.New("id required")
	}
	if rec.AccountID == "" || rec.ChannelID == "" || rec.EventType == "" {
		return errors.New("account_id, channel_id, event_type are required")
	}
	now := time.Now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	var err error
	if s.IsSQLite() {
		_, err = s.db.ExecContext(ctx, `INSERT INTO notification_subscriptions (id,account_id,channel_id,event_type,enabled,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled, updated_at=excluded.updated_at`,
			rec.ID, rec.AccountID, rec.ChannelID, rec.EventType, rec.Enabled, rec.CreatedAt, rec.UpdatedAt)
	} else {
		_, err = s.db.ExecContext(ctx, `INSERT INTO notification_subscriptions (id,account_id,channel_id,event_type,enabled,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?)
			ON DUPLICATE KEY UPDATE enabled=VALUES(enabled), updated_at=VALUES(updated_at)`,
			rec.ID, rec.AccountID, rec.ChannelID, rec.EventType, rec.Enabled, rec.CreatedAt, rec.UpdatedAt)
	}
	return err
}

// ListEnabledSubscriptionsForEvent 返回账号对指定事件启用的订阅与渠道。
func (s *Store) ListEnabledSubscriptionsForEvent(ctx context.Context, accountID, eventType string) ([]SubscriptionWithChannel, error) {
	accountID = normalizeAccount(accountID)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	// Use = 1 for boolean comparison - works in both SQLite and MySQL
	rows, err := s.db.QueryContext(ctx, `
SELECT ns.id, ns.account_id, ns.channel_id, ns.event_type, ns.enabled, ns.created_at, ns.updated_at,
       nc.id, nc.account_id, nc.channel_type, nc.name, nc.config, nc.enabled, nc.created_at, nc.updated_at
FROM notification_subscriptions ns
JOIN notification_channels nc ON ns.channel_id = nc.id
WHERE ns.account_id=? AND ns.event_type=? AND ns.enabled=1 AND nc.enabled=1`, accountID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SubscriptionWithChannel
	for rows.Next() {
		var sub NotificationSubscriptionRecord
		var ch NotificationChannelRecord
		if err := rows.Scan(
			&sub.ID, &sub.AccountID, &sub.ChannelID, &sub.EventType, &sub.Enabled, &sub.CreatedAt, &sub.UpdatedAt,
			&ch.ID, &ch.AccountID, &ch.ChannelType, &ch.Name, &ch.Config, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, SubscriptionWithChannel{Subscription: sub, Channel: ch})
	}
	return res, nil
}

// InsertNotificationHistory 写入通知历史。
func (s *Store) InsertNotificationHistory(ctx context.Context, rec NotificationHistoryRecord) error {
	if rec.ID == "" {
		return errors.New("id required")
	}
	if rec.AccountID == "" || rec.ChannelID == "" || rec.EventType == "" {
		return errors.New("account_id, channel_id, event_type are required")
	}
	now := time.Now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO notification_history
		(id, account_id, channel_id, event_type, title, content, status, error, sent_at, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.AccountID, rec.ChannelID, rec.EventType, rec.Title, rec.Content, rec.Status, nullOrString(rec.Error), rec.SentAt, rec.CreatedAt)
	return err
}
