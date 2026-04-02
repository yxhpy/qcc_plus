package proxy

import "net/http"

func (p *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if p == nil || p.sessionMgr == nil {
		writeJSON(w, http.StatusOK, map[string]any{"account": nil})
		return
	}

	sess := getSessionFromCookie(p.sessionMgr, r)
	if sess == nil {
		writeJSON(w, http.StatusOK, map[string]any{"account": nil})
		return
	}

	acc := p.getAccountByID(sess.AccountID)
	if acc == nil {
		writeJSON(w, http.StatusOK, map[string]any{"account": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"account": map[string]any{
			"id":            acc.ID,
			"name":          acc.Name,
			"proxy_api_key": acc.ProxyAPIKey,
			"is_admin":      acc.IsAdmin,
		},
	})
}
