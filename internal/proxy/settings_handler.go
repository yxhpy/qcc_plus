package proxy

import (
	"encoding/json"
	"net/http"
	"strings"

	"qcc_plus/internal/store"
)

// SettingsHandler 配置管理 API
type SettingsHandler struct {
	store store.SettingsStore
	cache *SettingsCache
}

// GetRuntimeDefinitions GET /api/settings/runtime-definitions
// 返回前端可编辑的系统级运行时配置目录。
func (h *SettingsHandler) GetRuntimeDefinitions(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": RuntimeSettingDefinitions(),
	})
}

// ListSettings GET /api/settings?scope=system&category=monitor&account_id=xxx
func (h *SettingsHandler) ListSettings(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings store not enabled"})
		return
	}

	scope := r.URL.Query().Get("scope")
	category := r.URL.Query().Get("category")
	accountID := r.URL.Query().Get("account_id")

	settings, err := h.store.ListSettings(scope, category, accountID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	for i := range settings {
		if settings[i].IsSecret {
			settings[i].Value = "******"
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":    settings,
		"version": h.getGlobalVersion(),
	})
}

// HandleSetting dispatches GET/PUT/DELETE for /api/settings/:key
func (h *SettingsHandler) HandleSetting(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/settings/")
	key = strings.TrimSuffix(key, "/")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.GetSetting(w, r, key)
	case http.MethodPut:
		h.UpdateSetting(w, r, key)
	case http.MethodDelete:
		h.DeleteSetting(w, r, key)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// GetVersion GET /api/settings/version
// 返回当前全局版本号，前端轮询比较
func (h *SettingsHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings store not enabled"})
		return
	}
	version, _ := h.store.GetGlobalVersion()
	writeJSON(w, http.StatusOK, map[string]any{"version": version})
}

// GetSetting GET /api/settings/:key
func (h *SettingsHandler) GetSetting(w http.ResponseWriter, r *http.Request, key string) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings store not enabled"})
		return
	}
	scope := r.URL.Query().Get("scope")
	accountID := r.URL.Query().Get("account_id")

	setting, err := h.store.GetSetting(key, scope, accountID)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if setting.IsSecret {
		setting.Value = "******"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":    setting,
		"version": h.getGlobalVersion(),
	})
}

// UpdateSetting PUT /api/settings/:key
// 请求体: {"value": any, "scope": "system", "account_id": null, "version": 1}
// 响应: {"success": true, "new_version": 2} 或 {"error": "version_conflict", "current_version": 3}
func (h *SettingsHandler) UpdateSetting(w http.ResponseWriter, r *http.Request, key string) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings store not enabled"})
		return
	}

	var req struct {
		Value       any     `json:"value"`
		Scope       string  `json:"scope"`
		AccountID   *string `json:"account_id"`
		DataType    string  `json:"data_type"`
		Category    string  `json:"category"`
		Description *string `json:"description"`
		IsSecret    *bool   `json:"is_secret"`
		Version     int     `json:"version"`
		UpdatedBy   *string `json:"updated_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	scope := req.Scope
	if scope == "" {
		scope = "system"
	}
	accountID := ""
	if req.AccountID != nil {
		accountID = *req.AccountID
	}

	existing, err := h.store.GetSetting(key, scope, accountID)
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 创建新配置（无版本要求）
	if existing == nil {
		setting := &store.Setting{
			Key:         key,
			Scope:       scope,
			AccountID:   req.AccountID,
			Value:       req.Value,
			DataType:    req.DataType,
			Category:    req.Category,
			Description: req.Description,
			IsSecret:    false,
			UpdatedBy:   req.UpdatedBy,
		}
		if req.IsSecret != nil {
			setting.IsSecret = *req.IsSecret
		}
		if err := h.store.UpsertSetting(setting); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if h.cache != nil {
			h.cache.UpdateLocal(key, req.Value, int64(setting.Version))
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "new_version": setting.Version})
		return
	}

	if req.Version == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "version required"})
		return
	}
	if req.Version != existing.Version {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "version_conflict", "current_version": existing.Version})
		return
	}

	setting := &store.Setting{
		Key:       key,
		Scope:     scope,
		AccountID: req.AccountID,
		Value:     req.Value,
		DataType:  existing.DataType,
		Category:  existing.Category,
		IsSecret:  existing.IsSecret,
		Version:   req.Version,
		UpdatedBy: req.UpdatedBy,
	}
	if req.DataType != "" {
		setting.DataType = req.DataType
	}
	if req.Category != "" {
		setting.Category = req.Category
	}
	if req.Description != nil {
		setting.Description = req.Description
	} else {
		setting.Description = existing.Description
	}
	if req.IsSecret != nil {
		setting.IsSecret = *req.IsSecret
	}

	if err := h.store.UpdateSetting(setting); err != nil {
		if err == store.ErrVersionConflict {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "version_conflict", "current_version": existing.Version})
			return
		}
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if h.cache != nil {
		h.cache.UpdateLocal(key, req.Value, int64(setting.Version))
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "new_version": setting.Version})
}

// BatchUpdate POST /api/settings/batch
func (h *SettingsHandler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings store not enabled"})
		return
	}

	var req struct {
		Settings []store.Setting `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	for i := range req.Settings {
		req.Settings[i].Key = strings.TrimSpace(req.Settings[i].Key)
		if req.Settings[i].Key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key required"})
			return
		}
		if req.Settings[i].Scope == "" {
			req.Settings[i].Scope = "system"
		}
	}

	if err := h.store.BatchUpdateSettings(req.Settings); err != nil {
		if err == store.ErrVersionConflict {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "version_conflict"})
			return
		}
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if h.cache != nil {
		h.cache.Refresh()
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "version": h.getGlobalVersion()})
}

// DeleteSetting DELETE /api/settings/:key
func (h *SettingsHandler) DeleteSetting(w http.ResponseWriter, r *http.Request, key string) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings store not enabled"})
		return
	}
	scope := r.URL.Query().Get("scope")
	accountID := r.URL.Query().Get("account_id")

	if err := h.store.DeleteSetting(key, scope, accountID); err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if h.cache != nil {
		h.cache.Refresh()
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": key})
}

func (h *SettingsHandler) getGlobalVersion() int64 {
	if h.store == nil {
		return 0
	}
	v, err := h.store.GetGlobalVersion()
	if err != nil {
		return 0
	}
	return v
}
