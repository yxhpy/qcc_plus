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

func setNoStoreHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
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
		APIKeyConfig:      n.APIKeyConfig,
		HealthCheckMethod: n.HealthCheckMethod,
		HealthCheckModel:  n.HealthCheckModel,
		ModelMapping:      encodeModelMapping(n.ModelMapping),
		SourceProtocol:    chooseNonEmpty(n.SourceProtocol, "claude"),
		WireAPI:           normalizeNodeWireAPI(n.SourceProtocol, n.WireAPI),
		AuthProfile:       n.AuthProfile,
		Capabilities:      n.Capabilities,
		AccountID:         chooseNonEmpty(n.AccountID, store.DefaultAccountID),
		Weight:            n.Weight,
		MaxConcurrency:    n.MaxConcurrency,
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

// encodeModelMapping 将 map[string]string 序列化为 JSON 字符串，空映射返回空字符串。
func encodeModelMapping(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// decodeModelMapping 将 JSON 字符串反序列化为 map[string]string。
func decodeModelMapping(s string) map[string]string {
	if s == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
