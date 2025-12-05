package proxy

import (
	"context"
	"net/http"
	"sort"
	"time"

	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
)

type MonitorDashboardResponse struct {
	AccountID   string        `json:"account_id"`
	AccountName string        `json:"account_name"`
	Nodes       []MonitorNode `json:"nodes"`
	UpdatedAt   string        `json:"updated_at"`
}

// ProxySummary 代理流量指标
type ProxySummary struct {
	SuccessRate     float64 `json:"success_rate"`      // 代理请求成功率
	AvgResponseTime int64   `json:"avg_response_time"` // 平均响应时间(ms)
	TotalRequests   int64   `json:"total_requests"`    // 总请求数
	FailedRequests  int64   `json:"failed_requests"`   // 失败请求数
}

// HealthSummary 健康检查指标
type HealthSummary struct {
	Status      string  `json:"status"`        // up/down/stale
	LastCheckAt *string `json:"last_check_at"` // 最近检查时间
	LastPingMs  int64   `json:"last_ping_ms"`  // 最近检查延时
	LastPingErr string  `json:"last_ping_err"` // 最近检查错误
	CheckMethod string  `json:"check_method"`  // 检查方式 api/head/cli
}

type MonitorNode struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	URL         string        `json:"url"`
	Status      string        `json:"status"` // 综合状态: online/degraded/offline/unknown/disabled
	Weight      int           `json:"weight"`
	IsActive    bool          `json:"is_active"`
	CircuitOpen bool          `json:"circuit_open"` // 熔断器是否打开
	Disabled    bool          `json:"disabled"`
	LastError   string        `json:"last_error"`
	Traffic     ProxySummary  `json:"traffic"` // 代理流量指标
	Health      HealthSummary `json:"health"`  // 健康检查指标
	Trend24h    []TrendPoint  `json:"trend_24h"`
}

type TrendPoint struct {
	Timestamp   string  `json:"timestamp"`
	SuccessRate float64 `json:"success_rate"`
	AvgTime     int64   `json:"avg_time"`
}

type nodeSnapshot struct {
	ID        string
	Name      string
	URL       string
	Weight    int
	Failed    bool
	Disabled  bool
	LastError string
	Method    string
	Metrics   metrics
	CreatedAt time.Time
}

func (p *Server) handleMonitorDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	target := caller
	if aid := r.URL.Query().Get("account_id"); aid != "" {
		if !isAdmin(r.Context()) && aid != caller.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		acc := p.getAccountByID(aid)
		if acc == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
		target = acc
	}

	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}

	resp := p.buildMonitorDashboardResponse(r.Context(), target)
	if resp == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "build dashboard failed"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (p *Server) buildMonitorDashboardResponse(ctx context.Context, target *Account) *MonitorDashboardResponse {
	if target == nil {
		return nil
	}

	// 检查是否隐藏禁用节点
	hideDisabled := true // 默认隐藏
	if p.settingsCache != nil {
		hideDisabled = p.settingsCache.GetBool("monitor.hide_disabled_nodes", true)
	}

	var (
		snapshots   []nodeSnapshot
		activeID    string
		accountID   string
		accountName string
		nodeIDs     []string
	)

	p.mu.RLock()
	activeID = target.ActiveID
	accountID = target.ID
	accountName = target.Name
	for _, n := range target.Nodes {
		// 如果启用了隐藏禁用节点，则跳过
		if hideDisabled && n.Disabled {
			continue
		}
		urlStr := ""
		if n.URL != nil {
			urlStr = n.URL.String()
		}
		nodeIDs = append(nodeIDs, n.ID)
		snapshots = append(snapshots, nodeSnapshot{
			ID:        n.ID,
			Name:      n.Name,
			URL:       urlStr,
			Weight:    n.Weight,
			Failed:    n.Failed,
			Disabled:  n.Disabled,
			LastError: n.LastError,
			Method:    n.HealthCheckMethod,
			Metrics:   n.Metrics,
			CreatedAt: n.CreatedAt,
		})
	}
	p.mu.RUnlock()

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Weight != snapshots[j].Weight {
			return snapshots[i].Weight < snapshots[j].Weight
		}
		ti := snapshots[i].CreatedAt
		tj := snapshots[j].CreatedAt
		if ti.IsZero() || tj.IsZero() {
			return ti.IsZero() && !tj.IsZero()
		}
		return ti.Before(tj)
	})

	trendRecords := make(map[string][]store.MetricsRecord, len(snapshots))
	if p.store != nil && len(nodeIDs) > 0 {
		recs, err := p.store.GetNodes24hTrend(ctx, accountID, nodeIDs)
		if err != nil {
			if p.logger != nil {
				p.logger.Printf("get trend failed account=%s: %v", accountID, err)
			}
		} else {
			trendRecords = recs
		}
	}

	now := time.Now()
	healthInterval := target.Config.HealthEvery
	if healthInterval <= 0 {
		healthInterval = p.healthEvery
	}
	nodes := make([]MonitorNode, 0, len(snapshots))
	for _, snap := range snapshots {
		traffic := summarizeTraffic(snap.Metrics)
		health := summarizeHealth(snap.Metrics, snap.Method, healthInterval, now)

		status := "unknown"
		if snap.Disabled {
			status = "disabled"
		} else if snap.Failed || health.Status == "down" {
			status = "offline"
		} else if health.Status == "stale" {
			status = "degraded"
		} else {
			status = "online"
		}

		lastError := snap.LastError
		if lastError == "" {
			lastError = snap.Metrics.LastPingErr
		}

		// 检查熔断器状态
		circuitOpen := false
		if cb := p.getCircuitBreaker(snap.ID); cb != nil {
			circuitOpen = cb.GetState() == StateOpen
		}

		nodes = append(nodes, MonitorNode{
			ID:          snap.ID,
			Name:        snap.Name,
			URL:         snap.URL,
			Status:      status,
			Weight:      snap.Weight,
			IsActive:    snap.ID == activeID,
			CircuitOpen: circuitOpen,
			Disabled:    snap.Disabled,
			LastError:   lastError,
			Traffic:     traffic,
			Health:      health,
			Trend24h:    buildTrendPoints(trendRecords[snap.ID]),
		})
	}

	name := accountName
	if name == "" {
		name = accountID
	}

	resp := MonitorDashboardResponse{
		AccountID:   accountID,
		AccountName: name,
		Nodes:       nodes,
		UpdatedAt:   timeutil.FormatBeijingTime(time.Now()),
	}
	return &resp
}

func calculateSuccessRate(successCount, failCount int64) float64 {
	total := successCount + failCount
	if total == 0 {
		return 100.0
	}
	return float64(successCount) / float64(total) * 100.0
}

func calculateAvgResponseTime(sumMS, count int64) int64 {
	if count == 0 {
		return 0
	}
	return sumMS / count
}

func buildTrendPoints(records []store.MetricsRecord) []TrendPoint {
	points := make([]TrendPoint, 0, len(records))
	for _, rec := range records {
		points = append(points, TrendPoint{
			Timestamp:   timeutil.FormatBeijingTime(rec.Timestamp),
			SuccessRate: calculateSuccessRate(rec.RequestsSuccess, rec.RequestsFailed),
			AvgTime:     calculateAvgResponseTime(rec.ResponseTimeSumMs, rec.ResponseTimeCount),
		})
	}
	return points
}

func summarizeTraffic(m metrics) ProxySummary {
	successCount := m.Requests - m.FailCount
	if successCount < 0 {
		successCount = 0
	}
	totalDuration := m.FirstByteDur + m.StreamDur

	return ProxySummary{
		SuccessRate:     calculateSuccessRate(successCount, m.FailCount),
		AvgResponseTime: calculateAvgResponseTime(totalDuration.Milliseconds(), m.Requests),
		TotalRequests:   m.Requests,
		FailedRequests:  m.FailCount,
	}
}

func summarizeHealth(m metrics, method string, interval time.Duration, now time.Time) HealthSummary {
	healthStatus := computeHealthStatus(m.LastHealthCheckAt, m.LastPingErr, interval, now)
	var lastCheck *string
	if !m.LastHealthCheckAt.IsZero() {
		formatted := timeutil.FormatBeijingTime(m.LastHealthCheckAt)
		lastCheck = &formatted
	}

	return HealthSummary{
		Status:      healthStatus,
		LastCheckAt: lastCheck,
		LastPingMs:  m.LastPingMS,
		LastPingErr: m.LastPingErr,
		CheckMethod: normalizeHealthCheckMethod(method),
	}
}

func computeHealthStatus(lastCheckAt time.Time, lastPingErr string, interval time.Duration, now time.Time) string {
	if interval > 0 && !lastCheckAt.IsZero() {
		if now.Sub(lastCheckAt) > 2*interval {
			return "stale"
		}
	}
	if lastPingErr != "" {
		return "down"
	}
	return "up"
}
