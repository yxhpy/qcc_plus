package proxy

import (
	"encoding/json"
	"net/http"
)

func (p *Server) handleErrorPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if p.errorPolicy == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": ErrorPolicySnapshot{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p.errorPolicy.Snapshot()})
}

func (p *Server) handleErrorPolicyToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if p.errorPolicy == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "error policy not enabled"})
		return
	}

	var req struct {
		Type    string `json:"type"` // builtin | observed
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	switch req.Type {
	case "builtin":
		p.errorPolicy.SetBuiltinRuleEnabled(req.ID, req.Enabled)
	case "observed":
		p.errorPolicy.SetObservedAutoSwitch(req.ID, req.Enabled)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown toggle type"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
