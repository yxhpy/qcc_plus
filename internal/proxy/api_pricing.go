package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"qcc_plus/internal/store"
)

// handlePricing 处理模型定价管理 API
// GET    /api/pricing       - 列出所有定价
// GET    /api/pricing?id=xx - 获取指定模型定价
// POST   /api/pricing       - 创建或更新定价
// DELETE /api/pricing?id=xx - 删除定价
func (p *Server) handlePricing(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		modelID := r.URL.Query().Get("id")
		if modelID != "" {
			// 获取单个模型定价
			pricing, err := p.store.GetModelPricing(r.Context(), modelID)
			if err == store.ErrNotFound {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "pricing not found"})
				return
			}
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, pricing)
			return
		}
		// 列出所有定价
		activeOnly := r.URL.Query().Get("active_only") == "true"
		pricings, err := p.store.ListModelPricing(r.Context(), activeOnly)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"pricing": pricings})

	case http.MethodPost:
		// 仅管理员可以管理定价
		if !isAdmin(r.Context()) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin required"})
			return
		}
		var req store.ModelPricingRecord
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.ModelID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_id required"})
			return
		}
		if req.ModelName == "" {
			req.ModelName = req.ModelID
		}
		if err := p.store.UpsertModelPricing(r.Context(), req); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"model_id": req.ModelID})

	case http.MethodDelete:
		// 仅管理员可以删除定价
		if !isAdmin(r.Context()) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin required"})
			return
		}
		modelID := r.URL.Query().Get("id")
		if modelID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		if err := p.store.DeleteModelPricing(r.Context(), modelID); err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "pricing not found"})
			return
		} else if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": modelID})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleUsageLogs 处理使用日志查询 API
// GET /api/usage/logs - 查询使用日志
// 参数: account_id, node_id, model_id, from, to, limit, offset
func (p *Server) handleUsageLogs(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	acc := accountFromCtx(r)
	params := store.QueryUsageParams{}

	// 非管理员只能查询自己账号的数据
	if !isAdmin(r.Context()) {
		if acc == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account missing"})
			return
		}
		params.AccountID = acc.ID
	} else {
		params.AccountID = r.URL.Query().Get("account_id")
	}

	params.NodeID = r.URL.Query().Get("node_id")
	params.ModelID = r.URL.Query().Get("model_id")

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			params.From = t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			params.To = t
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			params.Limit = l
		}
	}
	if params.Limit == 0 || params.Limit > 1000 {
		params.Limit = 100 // 默认限制
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			params.Offset = o
		}
	}

	logs, err := p.store.QueryUsageLogs(r.Context(), params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"logs": logs, "count": len(logs)})
}

// handleUsageSummary 处理使用统计汇总 API
// GET /api/usage/summary - 获取使用汇总
// 参数: account_id, node_id, model_id, from, to, group_by (model, node)
func (p *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	acc := accountFromCtx(r)
	params := store.QueryUsageParams{}

	// 非管理员只能查询自己账号的数据
	if !isAdmin(r.Context()) {
		if acc == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account missing"})
			return
		}
		params.AccountID = acc.ID
	} else {
		params.AccountID = r.URL.Query().Get("account_id")
	}

	params.NodeID = r.URL.Query().Get("node_id")
	params.ModelID = r.URL.Query().Get("model_id")

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			params.From = t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			params.To = t
		}
	}

	groupBy := r.URL.Query().Get("group_by")

	switch groupBy {
	case "model":
		summaries, err := p.store.GetUsageSummaryByModel(r.Context(), params)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"summaries": summaries})

	case "node":
		summaries, err := p.store.GetUsageSummaryByNode(r.Context(), params)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"summaries": summaries})

	default:
		// 返回总体汇总
		summary, err := p.store.GetUsageSummary(r.Context(), params)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}
}

// handleUsageCleanup 清理旧的使用日志
// POST /api/usage/cleanup - 清理使用日志
// 参数 (JSON body): retention_days
func (p *Server) handleUsageCleanup(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return
	}

	// 仅管理员可以执行清理
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin required"})
		return
	}

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.RetentionDays = 365 // 默认保留一年
	}

	if err := p.store.CleanupUsageLogs(r.Context(), req.RetentionDays); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":        "cleanup completed",
		"retention_days": req.RetentionDays,
	})
}
