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
	if r.HealthCheckModel == "" {
		r.HealthCheckModel = defaultHealthCheckModel
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

	var err error
	if s.IsSQLite() {
		_, err = s.db.ExecContext(ctx, `INSERT INTO nodes (id,name,base_url,api_key,health_check_method,health_check_model,account_id,weight,failed,disabled,last_error,created_at,requests,fail_count,fail_streak,total_bytes,total_input,total_output,stream_dur_ms,first_byte_ms,last_ping_ms,last_ping_err,last_health_check_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
				name=excluded.name,
				base_url=excluded.base_url,
				api_key=excluded.api_key,
				health_check_method=excluded.health_check_method,
				health_check_model=excluded.health_check_model,
				account_id=excluded.account_id,
				weight=excluded.weight,
				failed=excluded.failed,
				disabled=excluded.disabled,
				last_error=excluded.last_error,
				requests=excluded.requests,
				fail_count=excluded.fail_count,
				fail_streak=excluded.fail_streak,
				total_bytes=excluded.total_bytes,
				total_input=excluded.total_input,
				total_output=excluded.total_output,
				stream_dur_ms=excluded.stream_dur_ms,
				first_byte_ms=excluded.first_byte_ms,
				last_ping_ms=excluded.last_ping_ms,
				last_ping_err=excluded.last_ping_err,
				last_health_check_at=excluded.last_health_check_at`,
			r.ID, r.Name, r.BaseURL, r.APIKey, r.HealthCheckMethod, r.HealthCheckModel, r.AccountID, r.Weight, r.Failed, r.Disabled, r.LastError, r.CreatedAt, r.Requests, r.FailCount, r.FailStreak, r.TotalBytes, r.TotalInput, r.TotalOutput, r.StreamDurMs, r.FirstByteMs, r.LastPingMs, r.LastPingErr, healthAt)
	} else {
		_, err = s.db.ExecContext(ctx, `INSERT INTO nodes (id,name,base_url,api_key,health_check_method,health_check_model,account_id,weight,failed,disabled,last_error,created_at,requests,fail_count,fail_streak,total_bytes,total_input,total_output,stream_dur_ms,first_byte_ms,last_ping_ms,last_ping_err,last_health_check_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON DUPLICATE KEY UPDATE
				name=VALUES(name),
				base_url=VALUES(base_url),
				api_key=VALUES(api_key),
				health_check_method=VALUES(health_check_method),
				health_check_model=VALUES(health_check_model),
				account_id=VALUES(account_id),
				weight=VALUES(weight),
				failed=VALUES(failed),
				disabled=VALUES(disabled),
				last_error=VALUES(last_error),
				requests=VALUES(requests),
				fail_count=VALUES(fail_count),
				fail_streak=VALUES(fail_streak),
				total_bytes=VALUES(total_bytes),
				total_input=VALUES(total_input),
				total_output=VALUES(total_output),
				stream_dur_ms=VALUES(stream_dur_ms),
				first_byte_ms=VALUES(first_byte_ms),
				last_ping_ms=VALUES(last_ping_ms),
				last_ping_err=VALUES(last_ping_err),
				last_health_check_at=VALUES(last_health_check_at)`,
			r.ID, r.Name, r.BaseURL, r.APIKey, r.HealthCheckMethod, r.HealthCheckModel, r.AccountID, r.Weight, r.Failed, r.Disabled, r.LastError, r.CreatedAt, r.Requests, r.FailCount, r.FailStreak, r.TotalBytes, r.TotalInput, r.TotalOutput, r.StreamDurMs, r.FirstByteMs, r.LastPingMs, r.LastPingErr, healthAt)
	}
	return err
}

func (s *Store) GetNodesByAccount(ctx context.Context, accountID string) ([]NodeRecord, error) {
	accountID = normalizeAccount(accountID)
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,base_url,api_key,health_check_method,health_check_model,account_id,weight,failed,disabled,last_error,created_at,requests,fail_count,fail_streak,total_bytes,total_input,total_output,stream_dur_ms,first_byte_ms,last_ping_ms,last_ping_err,last_health_check_at FROM nodes WHERE account_id=? ORDER BY weight ASC, created_at ASC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []NodeRecord
	for rows.Next() {
		var r NodeRecord
		var lastHealthAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.Name, &r.BaseURL, &r.APIKey, &r.HealthCheckMethod, &r.HealthCheckModel, &r.AccountID, &r.Weight, &r.Failed, &r.Disabled, &r.LastError, &r.CreatedAt, &r.Requests, &r.FailCount, &r.FailStreak, &r.TotalBytes, &r.TotalInput, &r.TotalOutput, &r.StreamDurMs, &r.FirstByteMs, &r.LastPingMs, &r.LastPingErr, &lastHealthAt); err != nil {
			return nil, err
		}
		if r.HealthCheckMethod == "" {
			r.HealthCheckMethod = "api"
		}
		if r.HealthCheckModel == "" {
			r.HealthCheckModel = defaultHealthCheckModel
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
