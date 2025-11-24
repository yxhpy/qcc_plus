package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"

	"qcc_plus/internal/store"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func extractUsageFromHeader(h http.Header) *usage {
	if h == nil {
		return nil
	}
	input := headerInt(h.Get("X-Usage-Input-Tokens"))
	output := headerInt(h.Get("X-Usage-Output-Tokens"))
	if input == 0 && output == 0 {
		return nil
	}
	return &usage{input: input, output: output}
}

func headerInt(val string) int64 {
	if val == "" {
		return 0
	}
	i, _ := strconv.ParseInt(val, 10, 64)
	return i
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func toRecord(n *Node) store.NodeRecord {
	return store.NodeRecord{
		ID:                n.ID,
		Name:              n.Name,
		BaseURL:           n.URL.String(),
		APIKey:            n.APIKey,
		HealthCheckMethod: n.HealthCheckMethod,
		AccountID:         chooseNonEmpty(n.AccountID, store.DefaultAccountID),
		Weight:            n.Weight,
		Failed:            n.Failed,
		Disabled:          n.Disabled,
		LastError:         n.LastError,
		CreatedAt:         n.CreatedAt,
		Requests:          n.Metrics.Requests,
		FailCount:         n.Metrics.FailCount,
		FailStreak:        n.Metrics.FailStreak,
		TotalBytes:        n.Metrics.TotalBytes,
		TotalInput:        n.Metrics.TotalInputTokens,
		TotalOutput:       n.Metrics.TotalOutputTokens,
		StreamDurMs:       n.Metrics.StreamDur.Milliseconds(),
		FirstByteMs:       n.Metrics.FirstByteDur.Milliseconds(),
		LastPingMs:        n.Metrics.LastPingMS,
		LastPingErr:       n.Metrics.LastPingErr,
		LastHealthCheckAt: n.Metrics.LastHealthCheckAt,
	}
}
