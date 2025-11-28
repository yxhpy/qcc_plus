package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"qcc_plus/internal/store"
)

func (p *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	acc := accountFromCtx(r)
	if acc == nil {
		acc = p.defaultAccount
	}
	if isAdmin(r.Context()) {
		if aid := r.URL.Query().Get("account_id"); aid != "" {
			if target := p.getAccountByID(aid); target != nil {
				acc = target
			} else {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
				return
			}
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
		cfg := acc.Config
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"retries":             cfg.Retries,
			"fail_limit":          cfg.FailLimit,
			"health_interval_sec": int(cfg.HealthEvery.Seconds()),
			"window_size":         cfg.WindowSize,
			"alpha_err":           cfg.AlphaErr,
			"beta_latency":        cfg.BetaLatency,
		})
	case http.MethodPut:
		var req struct {
			Retries           int `json:"retries"`
			FailLimit         int `json:"fail_limit"`
			HealthIntervalSec int `json:"health_interval_sec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		healthEvery := time.Duration(req.HealthIntervalSec) * time.Second
		if err := p.updateConfigForAccount(acc, req.Retries, req.FailLimit, healthEvery); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// getConfig 获取默认账号配置。
func (p *Server) getConfig() Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.defaultAccount != nil {
		return p.defaultAccount.Config
	}
	retries := p.retries
	if retries == 0 {
		retries = 3
	}
	fail := p.failLimit
	if fail == 0 {
		fail = 3
	}
	health := p.healthEvery
	if health == 0 {
		health = 30 * time.Second
	}
	windowSize := p.windowSize
	if windowSize == 0 {
		windowSize = 200
	}
	alphaErr := p.alphaErr
	if alphaErr == 0 {
		alphaErr = 5.0
	}
	betaLat := p.betaLatency
	if betaLat == 0 {
		betaLat = 0.5
	}
	cooldown := p.cooldown
	if cooldown == 0 {
		cooldown = 30 * time.Second
	}
	minHealthy := p.minHealthy
	if minHealthy == 0 {
		minHealthy = 15 * time.Second
	}
	return Config{Retries: retries, FailLimit: fail, HealthEvery: health, WindowSize: windowSize, AlphaErr: alphaErr, BetaLatency: betaLat, Cooldown: cooldown, MinHealthy: minHealthy}
}

func (p *Server) updateConfigForAccount(acc *Account, retries, failLimit int, healthEvery time.Duration) error {
	if acc == nil {
		return errors.New("account required")
	}
	if retries < 1 || retries > 10 || failLimit < 1 || failLimit > 10 || healthEvery < 5*time.Second || healthEvery > 300*time.Second {
		return errors.New("invalid config values")
	}

	p.mu.Lock()
	cfg := acc.Config
	cfg.Retries = retries
	cfg.FailLimit = failLimit
	cfg.HealthEvery = healthEvery
	acc.Config = cfg
	active := acc.ActiveID
	if acc.ID == store.DefaultAccountID {
		if rt, ok := p.transport.(*retryTransport); ok {
			rt.attempts = retries
		}
		p.retries = retries
		p.failLimit = failLimit
		p.healthEvery = healthEvery
	}
	p.mu.Unlock()

	if p.store != nil {
		cfg := store.Config{Retries: retries, FailLimit: failLimit, HealthEvery: healthEvery}
		return p.store.UpdateConfig(context.Background(), acc.ID, cfg, active)
	}
	return nil
}
