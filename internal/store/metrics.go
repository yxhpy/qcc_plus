package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	retentionRaw    = 7 * 24 * time.Hour
	retentionHourly = 30 * 24 * time.Hour
	retentionDaily  = 365 * 24 * time.Hour
)

// InsertMetrics 写入原始监控数据。调用方应保证时间为 UTC，未指定则自动取当前时间。
func (s *Store) InsertMetrics(ctx context.Context, rec MetricsRecord) error {
	rec.AccountID = normalizeAccount(rec.AccountID)
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}
	// requests_total、requests_success、requests_failed 允许部分缺省，自动推导。
	if rec.RequestsTotal == 0 {
		rec.RequestsTotal = rec.RequestsSuccess + rec.RequestsFailed
	}
	if rec.RequestsSuccess == 0 && rec.RequestsTotal > 0 {
		rec.RequestsSuccess = rec.RequestsTotal - rec.RequestsFailed
	}
	if rec.ResponseTimeCount == 0 && rec.RequestsTotal > 0 {
		rec.ResponseTimeCount = rec.RequestsTotal
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO node_metrics_raw (
		account_id, node_id, ts, requests_total, requests_success, requests_failed,
		retry_attempts_total, retry_success,
		response_time_sum_ms, response_time_count, bytes_total,
		input_tokens_total, output_tokens_total, first_byte_time_sum_ms, stream_duration_sum_ms)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.AccountID, rec.NodeID, rec.Timestamp, rec.RequestsTotal, rec.RequestsSuccess, rec.RequestsFailed,
		rec.RetryAttemptsTotal, rec.RetrySuccess,
		rec.ResponseTimeSumMs, rec.ResponseTimeCount, rec.BytesTotal,
		rec.InputTokensTotal, rec.OutputTokensTotal, rec.FirstByteTimeSumMs, rec.StreamDurationSumMs)
	return err
}

// QueryMetrics 按时间范围和粒度获取监控数据，默认返回最近 24 小时的原始数据。
// Granularity 支持 raw/hour/day/month，对应不同表；Timestamp 字段表示所在桶的起始时间。
func (s *Store) QueryMetrics(ctx context.Context, q MetricsQuery) ([]MetricsRecord, error) {
	gran := q.Granularity
	if gran == "" {
		gran = MetricsGranularityRaw
	}
	table, timeCol, createdCol, err := metricsTableInfo(gran)
	if err != nil {
		return nil, err
	}
	if q.To.IsZero() {
		q.To = time.Now().UTC()
	}
	if q.From.IsZero() {
		// 默认窗口：原始 24h，小时 7d，天 30d，月 12m。
		switch gran {
		case MetricsGranularityRaw:
			q.From = q.To.Add(-24 * time.Hour)
		case MetricsGranularityHourly:
			q.From = q.To.Add(-7 * 24 * time.Hour)
		case MetricsGranularityDaily:
			q.From = q.To.AddDate(0, 0, -30)
		case MetricsGranularityMonthly:
			q.From = q.To.AddDate(-1, 0, 0)
		}
	}
	limit := q.Limit
	if q.Offset > 0 && limit == 0 {
		limit = 500
	}

	q.AccountID = normalizeAccount(q.AccountID)
	var args []interface{}
	b := &strings.Builder{}
	fmt.Fprintf(b, `SELECT account_id, node_id, %s AS ts, requests_total, requests_success, requests_failed,
		retry_attempts_total, retry_success,
		response_time_sum_ms, response_time_count, bytes_total, input_tokens_total, output_tokens_total,
		first_byte_time_sum_ms, stream_duration_sum_ms, %s AS created_at
		FROM %s WHERE account_id=?`, timeCol, createdCol, table)
	args = append(args, q.AccountID)
	if q.NodeID != "" {
		b.WriteString(" AND node_id=?")
		args = append(args, q.NodeID)
	}
	if !q.From.IsZero() {
		fmt.Fprintf(b, " AND %s >= ?", timeCol)
		args = append(args, q.From.UTC())
	}
	if !q.To.IsZero() {
		fmt.Fprintf(b, " AND %s < ?", timeCol)
		args = append(args, q.To.UTC())
	}
	b.WriteString(" ORDER BY " + timeCol + " ASC")
	if limit > 0 {
		b.WriteString(" LIMIT ?")
		args = append(args, limit)
	}
	if q.Offset > 0 {
		b.WriteString(" OFFSET ?")
		args = append(args, q.Offset)
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MetricsRecord
	for rows.Next() {
		var r MetricsRecord
		if err := rows.Scan(&r.AccountID, &r.NodeID, &r.Timestamp, &r.RequestsTotal, &r.RequestsSuccess, &r.RequestsFailed,
			&r.RetryAttemptsTotal, &r.RetrySuccess,
			&r.ResponseTimeSumMs, &r.ResponseTimeCount, &r.BytesTotal, &r.InputTokensTotal, &r.OutputTokensTotal,
			&r.FirstByteTimeSumMs, &r.StreamDurationSumMs, &r.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

// GetNode24hTrend 获取指定节点最近 24 小时的小时级聚合数据，按时间升序返回。
// 该函数会同时查询已聚合的小时数据和当前小时的原始数据，确保数据实时性。
func (s *Store) GetNode24hTrend(ctx context.Context, accountID, nodeID string) ([]MetricsRecord, error) {
	accountID = normalizeAccount(accountID)
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	// 当前小时的起始时间
	currentHourStart := now.Truncate(time.Hour)

	// 1. 查询已聚合的小时数据（不包含当前小时）
	hourlyQuery := `
        SELECT bucket_start, requests_total, requests_success, requests_failed,
               response_time_sum_ms, response_time_count
        FROM node_metrics_hourly
        WHERE account_id = ? AND node_id = ? AND bucket_start >= ? AND bucket_start < ?
        ORDER BY bucket_start ASC
    `

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, hourlyQuery, accountID, nodeID, from, currentHourStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []MetricsRecord
	for rows.Next() {
		var rec MetricsRecord
		if err := rows.Scan(&rec.Timestamp, &rec.RequestsTotal, &rec.RequestsSuccess, &rec.RequestsFailed, &rec.ResponseTimeSumMs, &rec.ResponseTimeCount); err != nil {
			return nil, err
		}
		rec.AccountID = accountID
		rec.NodeID = nodeID
		res = append(res, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. 查询当前小时的原始数据并聚合
	rawQuery := `
        SELECT SUM(requests_total), SUM(requests_success), SUM(requests_failed),
               SUM(response_time_sum_ms), SUM(response_time_count)
        FROM node_metrics_raw
        WHERE account_id = ? AND node_id = ? AND ts >= ? AND ts < ?
    `

	var rec MetricsRecord
	err = s.db.QueryRowContext(ctx, rawQuery, accountID, nodeID, currentHourStart, now).Scan(
		&rec.RequestsTotal, &rec.RequestsSuccess, &rec.RequestsFailed,
		&rec.ResponseTimeSumMs, &rec.ResponseTimeCount)
	if err == nil && rec.RequestsTotal > 0 {
		rec.AccountID = accountID
		rec.NodeID = nodeID
		rec.Timestamp = currentHourStart
		res = append(res, rec)
	}

	return res, nil
}

// GetNodes24hTrend 批量获取多个节点最近 24 小时的趋势数据，结果按 node_id 和时间升序排列。
// 该函数会同时查询已聚合的小时数据和当前小时的原始数据，确保数据实时性。
func (s *Store) GetNodes24hTrend(ctx context.Context, accountID string, nodeIDs []string) (map[string][]MetricsRecord, error) {
	result := make(map[string][]MetricsRecord)
	if len(nodeIDs) == 0 {
		return result, nil
	}
	accountID = normalizeAccount(accountID)
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	// 当前小时的起始时间
	currentHourStart := now.Truncate(time.Hour)

	placeholders := strings.Repeat("?,", len(nodeIDs))
	placeholders = strings.TrimSuffix(placeholders, ",")

	// 1. 查询已聚合的小时数据（不包含当前小时）
	hourlyQuery := fmt.Sprintf(`
        SELECT node_id, bucket_start, requests_total, requests_success, requests_failed,
               response_time_sum_ms, response_time_count
        FROM node_metrics_hourly
        WHERE account_id = ? AND node_id IN (%s) AND bucket_start >= ? AND bucket_start < ?
        ORDER BY node_id ASC, bucket_start ASC
    `, placeholders)

	hourlyArgs := make([]interface{}, 0, len(nodeIDs)+3)
	hourlyArgs = append(hourlyArgs, accountID)
	for _, id := range nodeIDs {
		hourlyArgs = append(hourlyArgs, id)
	}
	hourlyArgs = append(hourlyArgs, from, currentHourStart)

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, hourlyQuery, hourlyArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rec MetricsRecord
		var nodeID string
		if err := rows.Scan(&nodeID, &rec.Timestamp, &rec.RequestsTotal, &rec.RequestsSuccess, &rec.RequestsFailed, &rec.ResponseTimeSumMs, &rec.ResponseTimeCount); err != nil {
			return nil, err
		}
		rec.AccountID = accountID
		rec.NodeID = nodeID
		result[nodeID] = append(result[nodeID], rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. 查询当前小时的原始数据并聚合
	rawQuery := fmt.Sprintf(`
        SELECT node_id,
               SUM(requests_total) AS requests_total,
               SUM(requests_success) AS requests_success,
               SUM(requests_failed) AS requests_failed,
               SUM(response_time_sum_ms) AS response_time_sum_ms,
               SUM(response_time_count) AS response_time_count
        FROM node_metrics_raw
        WHERE account_id = ? AND node_id IN (%s) AND ts >= ? AND ts < ?
        GROUP BY node_id
    `, placeholders)

	rawArgs := make([]interface{}, 0, len(nodeIDs)+3)
	rawArgs = append(rawArgs, accountID)
	for _, id := range nodeIDs {
		rawArgs = append(rawArgs, id)
	}
	rawArgs = append(rawArgs, currentHourStart, now)

	rawRows, err := s.db.QueryContext(ctx, rawQuery, rawArgs...)
	if err != nil {
		return nil, err
	}
	defer rawRows.Close()

	for rawRows.Next() {
		var rec MetricsRecord
		var nodeID string
		if err := rawRows.Scan(&nodeID, &rec.RequestsTotal, &rec.RequestsSuccess, &rec.RequestsFailed, &rec.ResponseTimeSumMs, &rec.ResponseTimeCount); err != nil {
			return nil, err
		}
		// 只有当有数据时才添加当前小时的记录
		if rec.RequestsTotal > 0 {
			rec.AccountID = accountID
			rec.NodeID = nodeID
			rec.Timestamp = currentHourStart // 使用当前小时的起始时间作为时间戳
			result[nodeID] = append(result[nodeID], rec)
		}
	}
	if err := rawRows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// AggregateMetrics 将低粒度数据聚合到更高粒度。
// target 取值：hour(原始->小时)、day(小时->天)、month(天->月)。
func (s *Store) AggregateMetrics(ctx context.Context, accountID string, target MetricsGranularity, from, to time.Time) error {
	srcTable, srcTimeCol, dstTable, bucketExpr, err := s.aggregationPlan(target)
	if err != nil {
		return err
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		switch target {
		case MetricsGranularityHourly:
			from = to.Add(-24 * time.Hour)
		case MetricsGranularityDaily:
			from = to.AddDate(0, 0, -7)
		case MetricsGranularityMonthly:
			from = to.AddDate(0, -1, 0)
		}
	}
	var args []interface{}
	b := &strings.Builder{}
	fmt.Fprintf(b, `INSERT INTO %s (
		account_id, node_id, bucket_start, requests_total, requests_success, requests_failed,
		retry_attempts_total, retry_success,
		response_time_sum_ms, response_time_count, bytes_total, input_tokens_total, output_tokens_total,
		first_byte_time_sum_ms, stream_duration_sum_ms)
		SELECT account_id, node_id, %s AS bucket_start,
			SUM(requests_total), SUM(requests_success), SUM(requests_failed),
			SUM(retry_attempts_total), SUM(retry_success),
			SUM(response_time_sum_ms), SUM(response_time_count), SUM(bytes_total),
			SUM(input_tokens_total), SUM(output_tokens_total), SUM(first_byte_time_sum_ms), SUM(stream_duration_sum_ms)
		FROM %s WHERE %s >= ? AND %s < ?`, dstTable, bucketExpr, srcTable, srcTimeCol, srcTimeCol)
	args = append(args, from.UTC(), to.UTC())
	if accountID != "" {
		accountID = normalizeAccount(accountID)
		b.WriteString(" AND account_id=?")
		args = append(args, accountID)
	}
	b.WriteString(" GROUP BY account_id, node_id, bucket_start")
	if s.IsSQLite() {
		b.WriteString(" ON CONFLICT(account_id, node_id, bucket_start) DO UPDATE SET ")
		b.WriteString("requests_total=excluded.requests_total, requests_success=excluded.requests_success, requests_failed=excluded.requests_failed, ")
		b.WriteString("retry_attempts_total=excluded.retry_attempts_total, retry_success=excluded.retry_success, ")
		b.WriteString("response_time_sum_ms=excluded.response_time_sum_ms, response_time_count=excluded.response_time_count, ")
		b.WriteString("bytes_total=excluded.bytes_total, input_tokens_total=excluded.input_tokens_total, output_tokens_total=excluded.output_tokens_total, ")
		b.WriteString("first_byte_time_sum_ms=excluded.first_byte_time_sum_ms, stream_duration_sum_ms=excluded.stream_duration_sum_ms")
	} else {
		b.WriteString(" ON DUPLICATE KEY UPDATE ")
		b.WriteString("requests_total=VALUES(requests_total), requests_success=VALUES(requests_success), requests_failed=VALUES(requests_failed), ")
		b.WriteString("retry_attempts_total=VALUES(retry_attempts_total), retry_success=VALUES(retry_success), ")
		b.WriteString("response_time_sum_ms=VALUES(response_time_sum_ms), response_time_count=VALUES(response_time_count), ")
		b.WriteString("bytes_total=VALUES(bytes_total), input_tokens_total=VALUES(input_tokens_total), output_tokens_total=VALUES(output_tokens_total), ")
		b.WriteString("first_byte_time_sum_ms=VALUES(first_byte_time_sum_ms), stream_duration_sum_ms=VALUES(stream_duration_sum_ms)")
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	_, err = s.db.ExecContext(ctx, b.String(), args...)
	return err
}

// CleanupMetrics 按保留策略清理数据；accountID 为空时清理全部租户。
func (s *Store) CleanupMetrics(ctx context.Context, accountID string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	account := accountID
	if account != "" {
		account = normalizeAccount(account)
	}

	cuts := []struct {
		table string
		col   string
		keep  time.Duration
	}{
		{"node_metrics_raw", "ts", retentionRaw},
		{"node_metrics_hourly", "bucket_start", retentionHourly},
		{"node_metrics_daily", "bucket_start", retentionDaily},
	}
	for _, c := range cuts {
		cutoff := now.Add(-c.keep)
		b := &strings.Builder{}
		fmt.Fprintf(b, "DELETE FROM %s WHERE %s < ?", c.table, c.col)
		args := []interface{}{cutoff}
		if account != "" {
			b.WriteString(" AND account_id=?")
			args = append(args, account)
		}
		ctx, cancel := withTimeout(ctx)
		if _, err := s.db.ExecContext(ctx, b.String(), args...); err != nil {
			cancel()
			return err
		}
		cancel()
	}
	return nil
}

// metricsTableInfo 返回查询用的表、时间列名与 created_at 列（原始表为实际列，其余为 NULL）。
func metricsTableInfo(gr MetricsGranularity) (table, timeCol, createdCol string, err error) {
	switch gr {
	case MetricsGranularityRaw:
		return "node_metrics_raw", "ts", "created_at", nil
	case MetricsGranularityHourly:
		return "node_metrics_hourly", "bucket_start", "NULL", nil
	case MetricsGranularityDaily:
		return "node_metrics_daily", "bucket_start", "NULL", nil
	case MetricsGranularityMonthly:
		return "node_metrics_monthly", "bucket_start", "NULL", nil
	default:
		return "", "", "", fmt.Errorf("unsupported granularity: %s", gr)
	}
}

// aggregationPlan 定义从低粒度到目标粒度的聚合路径。
func (s *Store) aggregationPlan(target MetricsGranularity) (srcTable, srcTimeCol, dstTable, bucketExpr string, err error) {
	switch target {
	case MetricsGranularityHourly:
		if s.IsSQLite() {
			return "node_metrics_raw", "ts", "node_metrics_hourly", "strftime('%Y-%m-%d %H:00:00', ts)", nil
		}
		return "node_metrics_raw", "ts", "node_metrics_hourly", "DATE_FORMAT(ts, '%Y-%m-%d %H:00:00')", nil
	case MetricsGranularityDaily:
		if s.IsSQLite() {
			return "node_metrics_hourly", "bucket_start", "node_metrics_daily", "date(bucket_start)", nil
		}
		return "node_metrics_hourly", "bucket_start", "node_metrics_daily", "DATE(bucket_start)", nil
	case MetricsGranularityMonthly:
		if s.IsSQLite() {
			return "node_metrics_daily", "bucket_start", "node_metrics_monthly", "strftime('%Y-%m-01 00:00:00', bucket_start)", nil
		}
		return "node_metrics_daily", "bucket_start", "node_metrics_monthly", "DATE_FORMAT(bucket_start, '%Y-%m-01 00:00:00')", nil
	default:
		return "", "", "", "", fmt.Errorf("unsupported target granularity: %s", target)
	}
}
