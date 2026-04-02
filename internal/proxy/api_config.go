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

// getConfig 获取新账号应继承的当前运行时默认配置。
// 这里使用服务级运行时字段，而不是 default account 的持久化快照，
// 以避免显式 Builder 配置被数据库中的陈旧默认值覆盖。
func (p *Server) getConfig() Config {
	p.mu.RLock()
	retries := p.retries
	fail := p.failLimit
	health := p.healthEvery
	p.mu.RUnlock()

	if retries == 0 {
		retries = 3
	}
	if fail == 0 {
		fail = 3
	}
	if health == 0 {
		health = 30 * time.Second
	}
	return Config{Retries: retries, FailLimit: fail, HealthEvery: health}
}

func (p *Server) updateConfigForAccount(acc *Account, retries, failLimit int, healthEvery time.Duration) error {
	if acc == nil {
		return errors.New("account required")
	}
	if retries < 1 || retries > 10 || failLimit < 1 || failLimit > 10 || healthEvery < 5*time.Second || healthEvery > 300*time.Second {
		return errors.New("invalid config values")
	}

	p.mu.Lock()
	acc.Config = Config{Retries: retries, FailLimit: failLimit, HealthEvery: healthEvery}
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
		if err := p.store.UpdateConfig(context.Background(), acc.ID, cfg, active); err != nil {
			return err
		}
	}
	if acc.ID == store.DefaultAccountID {
		// 旧 /admin/api/config 仍然保留给默认账号兼容使用，但系统级运行时配置已经迁移到 settings 表。
		// 这里同步写回 settings，确保旧入口修改后依然具备热更新和重启持久化能力。
		if err := p.syncDefaultAccountConfigToSettings(retries, failLimit, healthEvery); err != nil {
			return err
		}
	}
	return nil
}

func (p *Server) syncDefaultAccountConfigToSettings(retries, failLimit int, healthEvery time.Duration) error {
	if p == nil || p.store == nil || p.settingsCache == nil {
		return nil
	}
	updates := []struct {
		key   string
		value any
	}{
		{key: "proxy.retry_max", value: retries},
		{key: "health.fail_threshold", value: failLimit},
		{key: "health.check_interval_sec", value: int(healthEvery / time.Second)},
	}

	for _, item := range updates {
		def, ok := LookupRuntimeSettingDefinition(item.key)
		if !ok {
			continue
		}
		setting, err := p.store.GetSetting(item.key, "system", "")
		if err != nil && err != store.ErrNotFound {
			return err
		}
		if setting == nil {
			setting = buildRuntimeSettingRecord(def, item.value)
		} else {
			setting.Value = item.value
		}
		if err := p.store.UpsertSetting(setting); err != nil {
			return err
		}
		p.settingsCache.UpdateLocal(item.key, item.value, int64(setting.Version))
	}
	return nil
}
