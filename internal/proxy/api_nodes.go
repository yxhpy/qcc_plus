package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

func (p *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	acc := accountFromCtx(r)
	if acc == nil {
		acc = p.defaultAccount
	}
	if isAdmin(r.Context()) {
		if aid := r.URL.Query().Get("account_id"); aid != "" {
			target := p.getAccountByID(aid)
			if target == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
				return
			}
			acc = target
		}
	} else if q := r.URL.Query().Get("account_id"); q != "" && acc != nil && q != acc.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if acc == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account missing"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"nodes": p.listNodes(acc)})
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		node := p.getNode(id)
		if node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		if !isAdmin(r.Context()) && node.AccountID != acc.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		var req struct {
			BaseURL           string  `json:"base_url"`
			APIKey            *string `json:"api_key"`
			Name              string  `json:"name"`
			Weight            int     `json:"weight"`
			HealthCheckMethod *string `json:"health_check_method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := p.updateNode(id, req.Name, req.BaseURL, req.APIKey, req.Weight, req.HealthCheckMethod); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		node := p.getNode(id)
		if node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		if !isAdmin(r.Context()) && node.AccountID != acc.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if err := p.deleteNode(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
	case http.MethodPost:
		var req struct {
			BaseURL           string `json:"base_url"`
			APIKey            string `json:"api_key"`
			Name              string `json:"name"`
			Weight            int    `json:"weight"`
			HealthCheckMethod string `json:"health_check_method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		node, err := p.addNodeWithMethod(acc, req.Name, req.BaseURL, req.APIKey, req.Weight, req.HealthCheckMethod)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": node.ID})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// 列出节点，标注是否激活和是否含密钥。
func (p *Server) listNodes(acc *Account) []map[string]interface{} {
	if acc == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(acc.Nodes))
	for id, n := range acc.Nodes {
		healthMethod := n.HealthCheckMethod
		if healthMethod == "" {
			healthMethod = HealthCheckMethodAPI
		}
		avgPerToken := "-"
		if n.Metrics.TotalOutputTokens > 0 {
			avgPerToken = fmt.Sprintf("%.2f", float64(n.Metrics.StreamDur.Milliseconds())/float64(n.Metrics.TotalOutputTokens))
		} else if n.Metrics.TotalBytes > 0 {
			avgPerToken = fmt.Sprintf("%.2f*", float64(n.Metrics.StreamDur.Milliseconds())/float64(n.Metrics.TotalBytes))
		}
		healthRate := 100.0
		if n.Metrics.Requests > 0 {
			healthRate = (float64(n.Metrics.Requests-n.Metrics.FailCount) / float64(n.Metrics.Requests)) * 100
			if healthRate < 0 {
				healthRate = 0
			}
		}
		lastHealthCheckAt := ""
		if !n.Metrics.LastHealthCheckAt.IsZero() {
			lastHealthCheckAt = n.Metrics.LastHealthCheckAt.Format(time.RFC3339)
		}
		out = append(out, map[string]interface{}{
			"id":                    id,
			"name":                  n.Name,
			"base_url":              n.URL.String(),
			"health_check_method":   healthMethod,
			"active":                id == acc.ActiveID,
			"has_api_key":           n.APIKey != "",
			"created_at":            n.CreatedAt.Format(time.RFC3339),
			"requests":              n.Metrics.Requests,
			"fail_count":            n.Metrics.FailCount,
			"fail_streak":           n.Metrics.FailStreak,
			"health_rate":           fmt.Sprintf("%.1f%%", healthRate),
			"ping_ms":               n.Metrics.LastPingMS,
			"ping_error":            n.Metrics.LastPingErr,
			"last_ping_ms":          n.Metrics.LastPingMS,
			"last_ping_error":       n.Metrics.LastPingErr,
			"last_health_check_at":  lastHealthCheckAt,
			"input_tokens":          n.Metrics.TotalInputTokens,
			"output_tokens":         n.Metrics.TotalOutputTokens,
			"total_bytes":           n.Metrics.TotalBytes,
			"stream_dur_ms":         n.Metrics.StreamDur.Milliseconds(),
			"first_byte_ms":         n.Metrics.FirstByteDur.Milliseconds(),
			"avg_recv_ms_per_token": avgPerToken,
			"weight":                n.Weight,
			"failed":                n.Failed,
			"disabled":              n.Disabled,
			"last_error":            n.LastError,
		})
	}

	// 按权重排序（权重越小优先级越高），相同权重按创建时间排序
	sort.Slice(out, func(i, j int) bool {
		wi := out[i]["weight"].(int)
		wj := out[j]["weight"].(int)
		if wi != wj {
			return wi < wj
		}
		// 权重相同，按创建时间排序
		ti, _ := time.Parse(time.RFC3339, out[i]["created_at"].(string))
		tj, _ := time.Parse(time.RFC3339, out[j]["created_at"].(string))
		return ti.Before(tj)
	})

	return out
}
