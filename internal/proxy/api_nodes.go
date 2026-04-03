package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"qcc_plus/internal/timeutil"
)

type nodeMutationRequest struct {
	BaseURL           string              `json:"base_url"`
	APIKey            *string             `json:"api_key"`
	APIKeys           *[]NamedAPIKey      `json:"api_keys"`
	Name              string              `json:"name"`
	Weight            int                 `json:"weight"`
	MaxConcurrency    *int                `json:"max_concurrency"`
	HealthCheckMethod *string             `json:"health_check_method"`
	HealthCheckModel  *string             `json:"health_check_model"`
	ModelMapping      *map[string]string  `json:"model_mapping"`
	SourceProtocol    *string             `json:"source_protocol"`
	WireAPI           *string             `json:"wire_api"`
	AuthProfile       *string             `json:"auth_profile"`
	Capabilities      *string             `json:"capabilities"`
}

func (p *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	acc, ok := p.resolveNodesAccount(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
			node := p.getNode(id)
			if node == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
				return
			}
			if !canManageAccount(r.Context(), node.AccountID) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"node": p.buildNodeView(acc, node, true)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"nodes": p.listNodes(acc)})

	case http.MethodPost:
		var req nodeMutationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		apiKey := ""
		if req.APIKey != nil {
			apiKey = strings.TrimSpace(*req.APIKey)
		}
		var apiKeys []NamedAPIKey
		if req.APIKeys != nil {
			apiKeys = *req.APIKeys
		}

		node, err := p.addNodeWithMethodAndKeys(
			acc,
			req.Name,
			req.BaseURL,
			apiKey,
			apiKeys,
			req.Weight,
			derefString(req.HealthCheckMethod),
			derefString(req.HealthCheckModel),
			derefStringMap(req.ModelMapping),
			derefString(req.SourceProtocol),
			derefString(req.WireAPI),
			derefString(req.AuthProfile),
			derefString(req.Capabilities),
			derefInt(req.MaxConcurrency),
		)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": node.ID})

	case http.MethodPut:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		node := p.getNode(id)
		if node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		if !canManageAccount(r.Context(), node.AccountID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}

		var req nodeMutationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		var apiKeys []NamedAPIKey
		if req.APIKeys != nil {
			apiKeys = *req.APIKeys
		}
		if err := p.updateNodeWithKeys(
			id,
			req.Name,
			req.BaseURL,
			req.APIKey,
			apiKeys,
			req.Weight,
			req.HealthCheckMethod,
			req.HealthCheckModel,
			req.ModelMapping,
			req.SourceProtocol,
			req.WireAPI,
			req.AuthProfile,
			req.Capabilities,
			req.MaxConcurrency,
		); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id})

	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		node := p.getNode(id)
		if node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		if !canManageAccount(r.Context(), node.AccountID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if err := p.deleteNode(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": id})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *Server) handleCopyNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	node := p.getNode(req.ID)
	if node == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if !canManageAccount(r.Context(), node.AccountID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	copied, err := p.copyNode(req.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   copied.ID,
		"node": p.buildNodeView(p.getAccountByID(copied.AccountID), copied, true),
	})
}

func (p *Server) resolveNodesAccount(w http.ResponseWriter, r *http.Request) (*Account, bool) {
	acc := accountFromCtx(r)
	if acc == nil {
		acc = p.defaultAccount
	}
	requested := strings.TrimSpace(r.URL.Query().Get("account_id"))

	if isAdmin(r.Context()) {
		if requested != "" {
			target := p.getAccountByID(requested)
			if target == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
				return nil, false
			}
			acc = target
		}
	} else if requested != "" && (acc == nil || requested != acc.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return nil, false
	}

	if acc == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account missing"})
		return nil, false
	}
	return acc, true
}

// 列出节点，标注是否激活和是否含密钥。
func (p *Server) listNodes(acc *Account) []map[string]any {
	if acc == nil {
		return nil
	}
	p.mu.RLock()
	nodes := make([]*Node, 0, len(acc.Nodes))
	for _, n := range acc.Nodes {
		nodes = append(nodes, n)
	}
	activeID := acc.ActiveID
	p.mu.RUnlock()

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Weight != nodes[j].Weight {
			return nodes[i].Weight < nodes[j].Weight
		}
		ti := nodes[i].CreatedAt
		tj := nodes[j].CreatedAt
		if ti.IsZero() || tj.IsZero() {
			return ti.IsZero() && !tj.IsZero()
		}
		return ti.Before(tj)
	})

	out := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		view := p.buildNodeView(acc, node, false)
		view["is_active"] = node.ID == activeID
		out = append(out, view)
	}
	return out
}

func (p *Server) buildNodeView(acc *Account, n *Node, includeSecrets bool) map[string]any {
	if n == nil {
		return nil
	}

	protocol := chooseNonEmpty(n.SourceProtocol, SourceProtocolClaude)
	healthMethod := normalizeHealthCheckMethod(n.HealthCheckMethod)
	healthModel := effectiveHealthCheckModelForProtocol(protocol, n.HealthCheckModel)

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

	keyItems := cloneNamedAPIKeys(n.APIKeyItems)
	if len(keyItems) == 0 && strings.TrimSpace(n.APIKey) != "" {
		keyItems = parseLegacyAPIKeys(n.APIKey)
	}
	keyNames := make([]string, 0, len(keyItems))
	for _, item := range keyItems {
		if trimmed := strings.TrimSpace(item.Name); trimmed != "" {
			keyNames = append(keyNames, trimmed)
		}
	}

	keyCount := len(keyItems)
	if keyCount == 0 && strings.TrimSpace(n.APIKey) != "" {
		keyCount = 1
	}
	activeKeyCount := keyCount
	if n.APIKeys != nil && n.APIKeys.KeyCount() > 0 {
		keyCount = n.APIKeys.KeyCount()
		activeKeyCount = n.APIKeys.ActiveKeyCount()
	}

	var activeConns int64
	if p.nodeScorer != nil {
		activeConns = p.nodeScorer.GetActiveConns(n.ID)
	}
	degraded := false
	if p.nodeScorer != nil {
		degraded = p.nodeScorer.IsDegraded(n.ID)
	}

	errorSeverity := ""
	if n.LastError != "" && n.Failed {
		errorSeverity = "node_down"
	}
	if n.APIKeys != nil && n.APIKeys.AllKeysDisabled() {
		errorSeverity = "key_invalid"
	}
	if degraded {
		errorSeverity = "degraded"
	}

	nodeStatus := "online"
	if n.Disabled {
		nodeStatus = "disabled"
	} else if n.Failed {
		nodeStatus = "offline"
	} else if degraded {
		nodeStatus = "degraded"
	}

	baseURL := ""
	if n.URL != nil {
		baseURL = n.URL.String()
	}

	view := map[string]any{
		"id":                    n.ID,
		"name":                  n.Name,
		"base_url":              baseURL,
		"health_check_method":   healthMethod,
		"health_check_model":    healthModel,
		"model_mapping":         n.ModelMapping,
		"source_protocol":       protocol,
		"wire_api":              normalizeNodeWireAPI(protocol, n.WireAPI),
		"auth_profile":          n.AuthProfile,
		"capabilities":          n.Capabilities,
		"status":                nodeStatus,
		"is_active":             acc != nil && n.ID == acc.ActiveID,
		"has_api_key":           keyCount > 0 || strings.TrimSpace(n.APIKey) != "",
		"key_count":             keyCount,
		"active_key_count":      activeKeyCount,
		"key_names":             keyNames,
		"active_conns":          activeConns,
		"degraded":              degraded,
		"error_severity":        errorSeverity,
		"created_at":            timeutil.FormatBeijingTime(n.CreatedAt),
		"requests":              n.Metrics.Requests,
		"fail_count":            n.Metrics.FailCount,
		"fail_streak":           n.Metrics.FailStreak,
		"health_rate":           healthRate,
		"ping_ms":               n.Metrics.LastPingMS,
		"ping_error":            n.Metrics.LastPingErr,
		"last_ping_ms":          n.Metrics.LastPingMS,
		"last_ping_error":       n.Metrics.LastPingErr,
		"last_health_check_at":  timeutil.FormatBeijingTime(n.Metrics.LastHealthCheckAt),
		"input_tokens":          n.Metrics.TotalInputTokens,
		"output_tokens":         n.Metrics.TotalOutputTokens,
		"total_bytes":           n.Metrics.TotalBytes,
		"stream_dur_ms":         n.Metrics.StreamDur.Milliseconds(),
		"first_byte_ms":         n.Metrics.FirstByteDur.Milliseconds(),
		"avg_recv_ms_per_token": avgPerToken,
		"weight":                n.Weight,
		"max_concurrency":       n.MaxConcurrency,
		"failed":                n.Failed,
		"disabled":              n.Disabled,
		"last_error":            n.LastError,
	}
	if includeSecrets {
		view["api_key"] = n.APIKey
		view["api_keys"] = keyItems
	}
	return view
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func derefStringMap(v *map[string]string) map[string]string {
	if v == nil {
		return nil
	}
	return *v
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
