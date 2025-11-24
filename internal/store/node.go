package store

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) UpsertNode(ctx context.Context, r NodeRecord) error {
	r.AccountID = normalizeAccount(r.AccountID)
	if r.HealthCheckMethod == "" {
		r.HealthCheckMethod = "api"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	healthAt := sql.NullTime{}
	if !r.LastHealthCheckAt.IsZero() {
		healthAt.Valid = true
		healthAt.Time = r.LastHealthCheckAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO nodes (id,name,base_url,api_key,health_check_method,account_id,weight,failed,disabled,last_error,created_at,requests,fail_count,fail_streak,total_bytes,total_input,total_output,stream_dur_ms,first_byte_ms,last_ping_ms,last_ping_err,last_health_check_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			name=VALUES(name),
			base_url=VALUES(base_url),
			api_key=VALUES(api_key),
			health_check_method=VALUES(health_check_method),
			account_id=VALUES(account_id),
			weight=VALUES(weight),
			failed=VALUES(failed),
			disabled=VALUES(disabled),
			last_error=VALUES(last_error),
			last_ping_ms=VALUES(last_ping_ms),
			last_ping_err=VALUES(last_ping_err),
			last_health_check_at=VALUES(last_health_check_at)`,
		r.ID, r.Name, r.BaseURL, r.APIKey, r.HealthCheckMethod, r.AccountID, r.Weight, r.Failed, r.Disabled, r.LastError, r.CreatedAt, r.Requests, r.FailCount, r.FailStreak, r.TotalBytes, r.TotalInput, r.TotalOutput, r.StreamDurMs, r.FirstByteMs, r.LastPingMs, r.LastPingErr, healthAt)
	return err
}

func (s *Store) GetNodesByAccount(ctx context.Context, accountID string) ([]NodeRecord, error) {
	accountID = normalizeAccount(accountID)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,base_url,api_key,health_check_method,account_id,weight,failed,disabled,last_error,created_at,requests,fail_count,fail_streak,total_bytes,total_input,total_output,stream_dur_ms,first_byte_ms,last_ping_ms,last_ping_err,last_health_check_at FROM nodes WHERE account_id=?`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []NodeRecord
	for rows.Next() {
		var r NodeRecord
		var lastHealthAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.Name, &r.BaseURL, &r.APIKey, &r.HealthCheckMethod, &r.AccountID, &r.Weight, &r.Failed, &r.Disabled, &r.LastError, &r.CreatedAt, &r.Requests, &r.FailCount, &r.FailStreak, &r.TotalBytes, &r.TotalInput, &r.TotalOutput, &r.StreamDurMs, &r.FirstByteMs, &r.LastPingMs, &r.LastPingErr, &lastHealthAt); err != nil {
			return nil, err
		}
		if r.HealthCheckMethod == "" {
			r.HealthCheckMethod = "api"
		}
		if lastHealthAt.Valid {
			r.LastHealthCheckAt = lastHealthAt.Time
		}
		records = append(records, r)
	}
	return records, nil
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id=?`, id)
	return err
}
