package proxy

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"qcc_plus/internal/timeutil"
)

// ModelRecoveryItem 模型恢复状态的 API 响应项。
type ModelRecoveryItem struct {
	NodeID         string  `json:"node_id"`
	NodeName       string  `json:"node_name"`
	ModelID        string  `json:"model_id"`
	AccountID      string  `json:"account_id"`
	Error          string  `json:"error"`
	FailedAt       string  `json:"failed_at"`
	OfflineSec     float64 `json:"offline_sec"`
	OfflineHuman   string  `json:"offline_human"`
	LastCheck      string  `json:"last_check,omitempty"`
	CheckCount     int     `json:"check_count"`
	NonRecoverable bool    `json:"non_recoverable"`
}

// ModelRecoveryResponse 模型恢复状态 API 响应。
type ModelRecoveryResponse struct {
	Total int                 `json:"total"`
	Items []ModelRecoveryItem `json:"items"`
}

// handleModelRecovery 返回所有正在恢复中的模型状态。
func (p *Server) handleModelRecovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if p.modelRecovery == nil {
		writeJSON(w, http.StatusOK, ModelRecoveryResponse{Total: 0, Items: []ModelRecoveryItem{}})
		return
	}

	// 获取当前账号（非管理员只能看自己的）
	acc := accountFromCtx(r)
	isAdmin := isAdminCtx(r)

	var failedModels []*FailedModelInfo
	if isAdmin {
		// 管理员可以查看所有账号
		accountID := r.URL.Query().Get("account_id")
		if accountID != "" {
			failedModels = p.modelRecovery.GetAllFailedByAccount(accountID)
		} else {
			failedModels = p.modelRecovery.GetAllFailed()
		}
	} else if acc != nil {
		failedModels = p.modelRecovery.GetAllFailedByAccount(acc.ID)
	}

	items := make([]ModelRecoveryItem, 0, len(failedModels))
	for _, fm := range failedModels {
		// 查找节点名称
		nodeName := fm.NodeID
		errorDetail := fm.Error
		p.mu.RLock()
		if node := p.nodeIndex[fm.NodeID]; node != nil {
			nodeName = node.Name
			errorDetail = preferDetailedError(errorDetail, node.LastError)
		}
		p.mu.RUnlock()

		offlineDur := fm.OfflineDuration()
		item := ModelRecoveryItem{
			NodeID:         fm.NodeID,
			NodeName:       nodeName,
			ModelID:        fm.ModelID,
			AccountID:      fm.AccountID,
			Error:          errorDetail,
			FailedAt:       timeutil.FormatBeijingTime(fm.FailedAt),
			OfflineSec:     math.Round(offlineDur.Seconds()*10) / 10,
			OfflineHuman:   formatDuration(offlineDur),
			CheckCount:     fm.CheckCount,
			NonRecoverable: fm.NonRecoverable,
		}
		if !fm.LastCheck.IsZero() {
			item.LastCheck = timeutil.FormatBeijingTime(fm.LastCheck)
		}
		items = append(items, item)
	}

	// 按离线时间降序排列（离线最久的排前面）
	sort.Slice(items, func(i, j int) bool {
		return items[i].OfflineSec > items[j].OfflineSec
	})

	writeJSON(w, http.StatusOK, ModelRecoveryResponse{
		Total: len(items),
		Items: items,
	})
}

// handleModelRecoveryDismiss 手动清除某个模型的恢复跟踪记录。
func (p *Server) handleModelRecoveryDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	modelID := r.URL.Query().Get("model_id")

	if nodeID == "" || modelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id and model_id required"})
		return
	}

	if p.modelRecovery != nil {
		p.modelRecovery.MarkRecovered(nodeID, modelID)
		p.logger.Printf("[model-recovery] manually dismissed model %s on node %s", modelID, nodeID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleModelRecoverySetNonRecoverable 设置某个恢复项是否不可恢复。
func (p *Server) handleModelRecoverySetNonRecoverable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	modelID := r.URL.Query().Get("model_id")
	value := r.URL.Query().Get("value")
	if nodeID == "" || modelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id and model_id required"})
		return
	}
	nonRecoverable := value == "1" || value == "true"
	if p.modelRecovery != nil {
		p.modelRecovery.SetNonRecoverable(nodeID, modelID, nonRecoverable)
		if p.store != nil {
			_ = p.store.SetFailedModelNonRecoverable(r.Context(), nodeID, modelID, nonRecoverable)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// formatDuration 将 Duration 格式化为人类可读的中文字符串。
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "刚刚"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		return fmt.Sprintf("%d分钟", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%d小时%d分钟", hours, mins)
		}
		return fmt.Sprintf("%d小时", hours)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%d天%d小时", days, hours)
	}
	return fmt.Sprintf("%d天", days)
}
