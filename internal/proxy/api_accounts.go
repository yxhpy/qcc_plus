package proxy

import (
	"context"
	"encoding/json"
	"net/http"

	"qcc_plus/internal/store"
)

// /admin/api/accounts
func (p *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if !isAdmin(r.Context()) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		var req struct {
			Name        string `json:"name"`
			Password    string `json:"password"`
			ProxyAPIKey string `json:"proxy_api_key"`
			IsAdmin     bool   `json:"is_admin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Password != "" && len(req.Password) < 6 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "密码至少6位"})
			return
		}
		acc, err := p.createAccount(req.Name, req.ProxyAPIKey, req.Password, req.IsAdmin)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": acc.ID})
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"accounts": p.listAccounts(r.Context())})
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if !canManageAccount(r.Context(), id) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Name        string `json:"name"`
			ProxyAPIKey string `json:"proxy_api_key"`
			Password    string `json:"password"`
			IsAdmin     *bool  `json:"is_admin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Password != "" && len(req.Password) < 6 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "密码至少6位"})
			return
		}
		p.mu.Lock()
		acc := p.accountByID[id]
		if acc == nil {
			p.mu.Unlock()
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if req.Name != "" {
			acc.Name = req.Name
		}
		if req.ProxyAPIKey != "" && req.ProxyAPIKey != acc.ProxyAPIKey {
			if exist := p.accounts[req.ProxyAPIKey]; exist != nil && exist.ID != acc.ID {
				p.mu.Unlock()
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "proxy_api_key already exists"})
				return
			}
			delete(p.accounts, acc.ProxyAPIKey)
			acc.ProxyAPIKey = req.ProxyAPIKey
			p.accounts[acc.ProxyAPIKey] = acc
		}
		if req.Password != "" {
			acc.Password = req.Password
		}
		if req.IsAdmin != nil {
			if !isAdmin(r.Context()) && !*req.IsAdmin {
				// 非管理员不能修改 is_admin 以外的权限
			} else {
				acc.IsAdmin = *req.IsAdmin
			}
		}
		p.mu.Unlock()
		if p.store != nil {
			_ = p.store.UpdateAccount(context.Background(), store.AccountRecord{
				ID:          acc.ID,
				Name:        acc.Name,
				Password:    acc.Password,
				ProxyAPIKey: acc.ProxyAPIKey,
				IsAdmin:     acc.IsAdmin,
			})
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": acc.ID})
	case http.MethodDelete:
		if !isAdmin(r.Context()) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		if id == store.DefaultAccountID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete default account"})
			return
		}
		p.mu.Lock()
		acc := p.accountByID[id]
		if acc == nil {
			p.mu.Unlock()
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		for nid := range acc.Nodes {
			delete(p.nodeIndex, nid)
			delete(p.nodeAccount, nid)
		}
		delete(p.accounts, acc.ProxyAPIKey)
		delete(p.accountByID, id)
		p.mu.Unlock()
		if p.store != nil {
			_ = p.store.DeleteAccount(context.Background(), id)
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *Server) listAccounts(ctx context.Context) []map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(p.accountByID))
	for _, acc := range p.accountByID {
		if !isAdmin(ctx) && ctx != nil {
			if caller, ok := ctx.Value(accountContextKey{}).(*Account); ok && caller.ID != acc.ID {
				continue
			}
		}
		out = append(out, map[string]interface{}{
			"id":            acc.ID,
			"name":          acc.Name,
			"proxy_api_key": acc.ProxyAPIKey,
			"is_admin":      acc.IsAdmin,
		})
	}
	return out
}
