package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"qcc_plus/internal/store"
	"qcc_plus/internal/tunnel"
)

func (p *Server) handleTunnelConfig(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if p.store == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "未启用存储，无法保存隧道配置"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, p.GetTunnelStatus())
	case http.MethodPut:
		var req struct {
			APIToken  string `json:"api_token"`
			Subdomain string `json:"subdomain"`
			Zone      string `json:"zone"`
			Enabled   *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		p.tunnelMu.Lock()
		running := p.tunnelMgr != nil
		p.tunnelMu.Unlock()
		if running && (req.APIToken != "" || req.Subdomain != "" || req.Zone != "") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "隧道运行中，请先停止后再修改配置"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		existing, err := p.store.GetTunnelConfig(ctx)
		if err != nil && !isNotFound(err) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if existing == nil {
			existing = &store.TunnelConfig{ID: "default"}
		}

		if req.APIToken != "" {
			existing.APIToken = strings.TrimSpace(req.APIToken)
		}
		if req.Subdomain != "" {
			existing.Subdomain = strings.TrimSpace(req.Subdomain)
		}
		existing.Zone = strings.TrimSpace(req.Zone)
		if req.Enabled != nil {
			existing.Enabled = *req.Enabled
		}

		if existing.Subdomain == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subdomain 不能为空"})
			return
		}

		if err := p.store.SaveTunnelConfig(ctx, *existing); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, p.GetTunnelStatus())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *Server) handleTunnelStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := p.StartTunnel(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p.GetTunnelStatus())
}

func (p *Server) handleTunnelStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := p.StopTunnel(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p.GetTunnelStatus())
}

func (p *Server) handleTunnelZones(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isAdmin(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if p.store == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "未启用存储，无法获取域名列表"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := p.store.GetTunnelConfig(ctx)
	if err != nil {
		if isNotFound(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请先保存 Cloudflare API Token"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if cfg.APIToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请先保存 Cloudflare API Token"})
		return
	}

	client := tunnel.NewClient(cfg.APIToken)
	zones, err := client.ListZones(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	res := make([]string, 0, len(zones))
	for _, z := range zones {
		res = append(res, z.Name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"zones": res})
}

func isNotFound(err error) bool {
	return err != nil && (errors.Is(err, store.ErrNotFound) || err.Error() == store.ErrNotFound.Error())
}
