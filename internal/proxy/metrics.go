package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type metricsWriter struct {
	http.ResponseWriter
	firstWrite bool
	firstAt    time.Time
	lastAt     time.Time
	bytes      int64
	status     int
}

func (mw *metricsWriter) Header() http.Header { return mw.ResponseWriter.Header() }

func (mw *metricsWriter) WriteHeader(code int) {
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

func (p *Server) recordMetrics(nodeID string, start time.Time, mw *metricsWriter, u *usage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	node, ok := p.nodeIndex[nodeID]
	if !ok {
		return
	}
	acc := p.nodeAccount[nodeID]
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
		node.Metrics.FailStreak++
	}
	if mw != nil && mw.status == http.StatusOK {
		node.Metrics.FailStreak = 0
		node.LastError = ""
		node.Failed = false
		if acc != nil {
			delete(acc.FailedSet, nodeID)
		}
	}
	if p.store != nil {
		rec := toRecord(node)
		_ = p.store.UpsertNode(context.Background(), rec)
	}
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
