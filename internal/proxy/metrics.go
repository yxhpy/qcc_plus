package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
)

type metricsWriter struct {
	http.ResponseWriter
	firstWrite  bool
	wroteHeader bool
	firstAt     time.Time
	lastAt      time.Time
	bytes       int64
	status      int
}

func (mw *metricsWriter) Header() http.Header { return mw.ResponseWriter.Header() }

func (mw *metricsWriter) WriteHeader(code int) {
	if mw.wroteHeader {
		return
	}
	mw.wroteHeader = true
	mw.status = code
	mw.ResponseWriter.WriteHeader(code)
}

func (mw *metricsWriter) Write(b []byte) (int, error) {
	if !mw.firstWrite {
		mw.firstWrite = true
		mw.firstAt = time.Now()
		if mw.status == 0 {
			mw.status = http.StatusOK
		}
	}
	mw.lastAt = time.Now()
	mw.bytes += int64(len(b))
	return mw.ResponseWriter.Write(b)
}

func (p *Server) recordMetrics(nodeID string, start time.Time, mw *metricsWriter, u *usage, retryAttempts, retrySuccess int64) {
	end := time.Now()
	var (
		nodeRec      store.NodeRecord
		metricsRec   *store.MetricsRecord
		nodeName     string
		nodeIDCopy   string
		nodeFailed   bool
		nodeDisabled bool
		requests     int64
		failCount    int64
		firstByteDur time.Duration
		streamDur    time.Duration
		lastPingMS   int64
		healthErr    string
		healthAt     time.Time
		method       string
		accountID    string
	)

	p.mu.Lock()
	node, ok := p.nodeIndex[nodeID]
	if !ok {
		p.mu.Unlock()
		return
	}
	acc := p.nodeAccount[nodeID]
	accountID = node.AccountID
	if acc != nil && acc.ID != "" {
		accountID = acc.ID
	}
	if accountID == "" {
		accountID = store.DefaultAccountID
	}
	node.Metrics.Requests++
	if mw != nil && mw.firstWrite {
		node.Metrics.FirstByteDur += mw.firstAt.Sub(start)
		node.Metrics.StreamDur += mw.lastAt.Sub(mw.firstAt)
		node.Metrics.TotalBytes += mw.bytes
	}
	if u != nil {
		node.Metrics.TotalInputTokens += u.input
		node.Metrics.TotalOutputTokens += u.output
	}
	if mw != nil && mw.status != http.StatusOK {
		node.Metrics.FailCount++
	}
	var wasFailedBeforeSuccess bool
	if mw != nil && mw.status == http.StatusOK {
		wasFailedBeforeSuccess = node.Failed
		node.Metrics.FailStreak = 0
		node.LastError = ""
		node.Failed = false
		if acc != nil {
			delete(acc.FailedSet, nodeID)
		}
	}
	if p.store != nil {
		nodeRec = toRecord(node)
		metricsRec = buildMetricsRecord(accountID, nodeID, start, end, mw, u, retryAttempts, retrySuccess)
	}
	nodeName = node.Name
	nodeIDCopy = node.ID
	nodeFailed = node.Failed
	nodeDisabled = node.Disabled
	requests = node.Metrics.Requests
	failCount = node.Metrics.FailCount
	firstByteDur = node.Metrics.FirstByteDur
	streamDur = node.Metrics.StreamDur
	lastPingMS = node.Metrics.LastPingMS
	healthErr = node.Metrics.LastPingErr
	healthAt = node.Metrics.LastHealthCheckAt
	method = node.HealthCheckMethod
	p.mu.Unlock()

	if p.store != nil {
		_ = p.store.UpsertNode(context.Background(), nodeRec)
		if metricsRec != nil {
			_ = p.store.InsertMetrics(context.Background(), *metricsRec)
		}
	}

	if p.wsHub != nil {
		traffic := summarizeTraffic(metrics{
			Requests:     requests,
			StreamDur:    streamDur,
			FirstByteDur: firstByteDur,
			FailCount:    failCount,
			LastPingMS:   lastPingMS,
			LastPingErr:  healthErr,
		})
		healthInterval := p.healthEvery
		if acc != nil && acc.Config.HealthEvery > 0 {
			healthInterval = acc.Config.HealthEvery
		}
		health := summarizeHealth(metrics{
			Requests:          requests,
			StreamDur:         streamDur,
			FirstByteDur:      firstByteDur,
			FailCount:         failCount,
			LastPingMS:        lastPingMS,
			LastPingErr:       healthErr,
			LastHealthCheckAt: healthAt,
		}, method, healthInterval, time.Now())

		status := "unknown"
		if nodeDisabled {
			status = "disabled"
		} else if nodeFailed || health.Status == "down" {
			status = "offline"
		} else if health.Status == "stale" {
			status = "degraded"
		} else {
			status = "online"
		}

		timestamp := timeutil.FormatBeijingTime(time.Now())
		if health.LastCheckAt != nil {
			timestamp = *health.LastCheckAt
		}

		// 如果节点从失败状态恢复，推送 node_status 事件通知前端
		if wasFailedBeforeSuccess {
			p.wsHub.Broadcast(accountID, "node_status", map[string]interface{}{
				"node_id":   nodeIDCopy,
				"node_name": nodeName,
				"status":    "online",
				"timestamp": timestamp,
			})
		}

		p.wsHub.Broadcast(accountID, "node_metrics", map[string]interface{}{
			"node_id":   nodeIDCopy,
			"node_name": nodeName,
			"status":    status,
			"traffic":   traffic,
			"health":    health,
			"timestamp": timestamp,
		})
	}
}

func buildMetricsRecord(accountID, nodeID string, start, end time.Time, mw *metricsWriter, u *usage, retryAttempts, retrySuccess int64) *store.MetricsRecord {
	rec := &store.MetricsRecord{
		AccountID:          accountID,
		NodeID:             nodeID,
		Timestamp:          end.UTC(),
		RequestsTotal:      1,
		RequestsSuccess:    1,
		RequestsFailed:     0,
		RetryAttemptsTotal: retryAttempts,
		RetrySuccess:       retrySuccess,
		ResponseTimeSumMs:  end.Sub(start).Milliseconds(),
		ResponseTimeCount:  1,
	}
	if mw != nil {
		rec.BytesTotal = mw.bytes
		if mw.status != http.StatusOK {
			rec.RequestsFailed = 1
			rec.RequestsSuccess = 0
		}
		if mw.firstWrite {
			rec.FirstByteTimeSumMs = mw.firstAt.Sub(start).Milliseconds()
			rec.StreamDurationSumMs = mw.lastAt.Sub(mw.firstAt).Milliseconds()
			rec.ResponseTimeSumMs = mw.lastAt.Sub(start).Milliseconds()
		}
	}
	if u != nil {
		rec.InputTokensTotal = u.input
		rec.OutputTokensTotal = u.output
	}
	return rec
}

// 从响应体或 SSE 数据中粗略提取 usage 字段（JSON 格式）。
func parseUsage(b []byte) (int64, int64) {
	key := []byte("\"usage\"")
	idx := bytes.LastIndex(b, key)
	if idx < 0 {
		return 0, 0
	}
	braceStart := bytes.IndexByte(b[idx:], '{')
	if braceStart < 0 {
		return 0, 0
	}
	braceStart += idx
	depth := 0
	for i := braceStart; i < len(b); i++ {
		switch b[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				usageObj := b[braceStart : i+1]
				var tmp struct {
					InputTokens  int64 `json:"input_tokens"`
					OutputTokens int64 `json:"output_tokens"`
				}
				if err := json.Unmarshal(usageObj, &tmp); err == nil {
					return tmp.InputTokens, tmp.OutputTokens
				}
				break
			}
		}
	}
	return 0, 0
}
