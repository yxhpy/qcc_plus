package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
	"qcc_plus/internal/tunnel"
)

// Server 负责在多个上游节点间切换并提供管理页面。
type Server struct {
	mu          sync.RWMutex
	accounts    map[string]*Account // proxyAPIKey -> Account
	accountByID map[string]*Account // accountID -> Account
	nodeIndex   map[string]*Node    // nodeID -> Node
	nodeAccount map[string]*Account // nodeID -> Account

	defaultAccount *Account
	defaultAccName string
	// 兼容旧单租户字段
	nodes    map[string]*Node
	activeID string

	sessionMgr *SessionManager

	listenAddr       string
	transport        http.RoundTripper
	logger           *log.Logger
	retries          int
	failLimit        int
	healthEvery      time.Duration
	windowSize       int
	alphaErr         float64
	betaLatency      float64
	cooldown         time.Duration
	minHealthy       time.Duration
	healthRT         http.RoundTripper
	cliRunner        CliRunner
	store            *store.Store
	adminKey         string
	notifyMgr        *notify.Manager
	metricsScheduler *MetricsScheduler
	healthScheduler  *HealthScheduler
	healthQueue      chan healthJob
	healthWorkers    int
	healthStop       chan struct{}
	settingsCache    *SettingsCache
	settingsStopCh   chan struct{}
	settingsWg       sync.WaitGroup

	claudeConfigCache map[string]claudeConfigEntry
	claudeConfigMu    sync.RWMutex

	tunnelMgr *tunnel.Manager
	tunnelMu  sync.Mutex

	wsHub *WSHub
}

// Start 运行反向代理并阻塞直到关闭。
func (p *Server) Start() error {
	if p.healthScheduler != nil {
		if err := p.healthScheduler.Start(); err != nil {
			return err
		}
		defer p.healthScheduler.Stop()
	}
	if p.metricsScheduler != nil {
		if err := p.metricsScheduler.Start(); err != nil {
			return err
		}
		defer p.metricsScheduler.Stop()
	}

	go p.healthLoop()
	server := &http.Server{
		Addr:         p.listenAddr,
		Handler:      p.handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // 支持流式响应
	}

	p.logger.Printf("Claude Code proxy listening on %s", p.listenAddr)
	p.logger.Printf("Admin panel: http://%s/admin", p.listenAddr)
	p.logger.Printf("默认登录凭证:")
	p.logger.Printf("  - 管理员: username=admin, password=admin123")
	p.logger.Printf("  - 默认账号: username=%s, password=default123", chooseNonEmpty(p.defaultAccName, "default"))
	p.logger.Printf("⚠️  生产环境请立即修改默认密码！")
	p.logger.Printf("API Keys:")
	p.logger.Printf("  - Admin API Key: %s", p.adminKey)

	p.mu.RLock()
	for _, acc := range p.accounts {
		p.logger.Printf("  - Account '%s': proxy_api_key=%s", acc.Name, acc.ProxyAPIKey)
	}
	p.mu.RUnlock()

	return server.ListenAndServe()
}

// Stop 用于优雅关闭后台任务。
func (p *Server) Stop() {
	if p.healthScheduler != nil {
		p.healthScheduler.Stop()
	}
	if p.metricsScheduler != nil {
		p.metricsScheduler.Stop()
	}
	if p.healthStop != nil {
		select {
		case <-p.healthStop:
			// already closed
		default:
			close(p.healthStop)
		}
	}
	if p.settingsStopCh != nil {
		close(p.settingsStopCh)
		p.settingsWg.Wait()
	}
}

// Handler 暴露 HTTP 处理器，便于测试或自定义服务器。
func (p *Server) Handler() http.Handler {
	return p.handler()
}

// startSettingsWatcher 周期刷新设置缓存，用于跨实例热更新。
func (p *Server) startSettingsWatcher(interval time.Duration) {
	if p == nil || p.settingsCache == nil || interval <= 0 {
		return
	}
	if p.settingsStopCh != nil {
		return
	}
	p.settingsStopCh = make(chan struct{})
	p.settingsWg.Add(1)
	go func() {
		defer p.settingsWg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.settingsCache.Refresh()
			case <-p.settingsStopCh:
				return
			}
		}
	}()
}

// applySettingsFromCache 将缓存中的关键配置应用到运行时。
func (p *Server) applySettingsFromCache() {
	if p == nil || p.settingsCache == nil {
		return
	}
	if v, ok := p.settingsCache.Get("health.check_interval_sec"); ok {
		switch n := v.(type) {
		case float64:
			p.updateHealthInterval(time.Duration(n) * time.Second)
		case int:
			p.updateHealthInterval(time.Duration(n) * time.Second)
		case int64:
			p.updateHealthInterval(time.Duration(n) * time.Second)
		}
	}
	if v, ok := p.settingsCache.Get("proxy.retry_max"); ok {
		switch n := v.(type) {
		case float64:
			p.updateRetryMax(int(n))
		case int:
			p.updateRetryMax(n)
		case int64:
			p.updateRetryMax(int(n))
		}
	}
	if v, ok := p.settingsCache.Get("health.fail_threshold"); ok {
		switch n := v.(type) {
		case float64:
			p.updateFailLimit(int(n))
		case int:
			p.updateFailLimit(n)
		case int64:
			p.updateFailLimit(int(n))
		}
	}
}

// 创建默认账号及默认节点（如必要）。
func (p *Server) createDefaultAccount(defaultUpstream *url.URL, defaultCfg store.Config, name, proxyKey, upstreamKey string) error {
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
	healthBackoffMin := 5 * time.Second
	healthBackoffMax := 60 * time.Second
	if p.defaultAccount != nil {
		if p.defaultAccount.Config.HealthBackoffMin > 0 {
			healthBackoffMin = p.defaultAccount.Config.HealthBackoffMin
		}
		if p.defaultAccount.Config.HealthBackoffMax > 0 {
			healthBackoffMax = p.defaultAccount.Config.HealthBackoffMax
		}
	}
	healthConcurrency := p.healthWorkers
	if healthConcurrency == 0 {
		healthConcurrency = 4
	}
	cfg := Config{
		Retries:           defaultCfg.Retries,
		FailLimit:         defaultCfg.FailLimit,
		HealthEvery:       defaultCfg.HealthEvery,
		HealthBackoffMin:  healthBackoffMin,
		HealthBackoffMax:  healthBackoffMax,
		HealthConcurrency: healthConcurrency,
		WindowSize:        windowSize,
		AlphaErr:          alphaErr,
		BetaLatency:       betaLat,
		Cooldown:          cooldown,
		MinHealthy:        minHealthy,
	}
	acc := &Account{
		ID:          store.DefaultAccountID,
		Name:        chooseNonEmpty(name, "default"),
		Password:    "default123",
		ProxyAPIKey: proxyKey,
		IsAdmin:     true,
		Config:      cfg,
		Nodes:       make(map[string]*Node),
		FailedSet:   make(map[string]struct{}),
	}
	if defaultUpstream == nil {
		return errors.New("default upstream required for initial account")
	}
	method := normalizeHealthCheckMethod(defaultHealthCheckMethod)
	if healthMethodRequiresAPIKey(method) && upstreamKey == "" {
		p.logger.Printf("health check mode %s requires api key, fallback to head for default node", method)
		method = HealthCheckMethodHEAD
	}
	node := &Node{
		ID:                "default",
		Name:              chooseNonEmpty(defaultUpstream.Host, "default"),
		URL:               defaultUpstream,
		APIKey:            upstreamKey,
		HealthCheckMethod: method,
		AccountID:         acc.ID,
		CreatedAt:         time.Now(),
		Weight:            1,
		Window:            NewMetricsWindow(cfg.WindowSize),
	}
	node.Score = CalculateScore(node, cfg.AlphaErr, cfg.BetaLatency)
	acc.Nodes[node.ID] = node
	acc.ActiveID = node.ID
	p.registerAccount(acc)
	if p.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.store.CreateAccount(ctx, store.AccountRecord{ID: acc.ID, Name: acc.Name, Password: acc.Password, ProxyAPIKey: acc.ProxyAPIKey, IsAdmin: true, CreatedAt: node.CreatedAt, UpdatedAt: node.CreatedAt})
		_ = p.store.UpsertNode(ctx, store.NodeRecord{ID: node.ID, Name: node.Name, BaseURL: node.URL.String(), APIKey: node.APIKey, HealthCheckMethod: node.HealthCheckMethod, AccountID: acc.ID, Weight: node.Weight, CreatedAt: node.CreatedAt})
		_ = p.store.SetActive(ctx, acc.ID, node.ID)
		_ = p.store.UpdateConfig(ctx, acc.ID, defaultCfg, node.ID)
	}
	return nil
}

// 从持久层加载账号、节点与配置。
func (p *Server) loadAccountsFromStore(defaultUpstream *url.URL, defaultCfg Config, defaultUpstreamKey string) error {
	if p.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	accounts, err := p.store.ListAccounts(ctx)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return nil
	}
	for _, a := range accounts {
		cfg := defaultCfg
		if cfg.WindowSize == 0 {
			cfg.WindowSize = 200
		}
		if cfg.AlphaErr == 0 {
			cfg.AlphaErr = 5.0
		}
		if cfg.BetaLatency == 0 {
			cfg.BetaLatency = 0.5
		}
		if cfg.HealthBackoffMin == 0 {
			cfg.HealthBackoffMin = 5 * time.Second
		}
		if cfg.HealthBackoffMax == 0 {
			cfg.HealthBackoffMax = 60 * time.Second
		}
		if cfg.HealthConcurrency == 0 {
			cfg.HealthConcurrency = 4
		}
		// 加载节点与活动节点
		recs, cfgLoaded, active, err := p.store.LoadAllByAccount(ctx, a.ID)
		if err != nil {
			return err
		}
		if cfgLoaded.Retries > 0 {
			cfg.Retries = cfgLoaded.Retries
		}
		if cfgLoaded.FailLimit > 0 {
			cfg.FailLimit = cfgLoaded.FailLimit
		}
		if cfgLoaded.HealthEvery > 0 {
			cfg.HealthEvery = cfgLoaded.HealthEvery
		}

		password := a.Password
		if password == "" {
			if a.ID == store.DefaultAccountID {
				password = "default123"
			} else if a.IsAdmin {
				password = "admin123"
			}
		}

		acc := &Account{
			ID:          a.ID,
			Name:        chooseNonEmpty(a.Name, a.ID),
			Password:    password,
			ProxyAPIKey: a.ProxyAPIKey,
			IsAdmin:     a.IsAdmin,
			Config:      cfg,
			Nodes:       make(map[string]*Node),
			FailedSet:   make(map[string]struct{}),
			ActiveID:    active,
		}

		// 如果账号没有节点且是默认账号，创建一个默认节点以保证可用。
		if len(recs) == 0 && a.ID == store.DefaultAccountID && defaultUpstream != nil {
			method := normalizeHealthCheckMethod(defaultHealthCheckMethod)
			if healthMethodRequiresAPIKey(method) && defaultUpstreamKey == "" {
				p.logger.Printf("health check mode %s requires api key, fallback to head for default node", method)
				method = HealthCheckMethodHEAD
			}
			node := &Node{
				ID:                "default",
				Name:              chooseNonEmpty(defaultUpstream.Host, "default"),
				URL:               defaultUpstream,
				APIKey:            defaultUpstreamKey,
				HealthCheckMethod: method,
				AccountID:         acc.ID,
				CreatedAt:         time.Now(),
				Weight:            1,
				Window:            NewMetricsWindow(cfg.WindowSize),
			}
			node.Score = CalculateScore(node, cfg.AlphaErr, cfg.BetaLatency)
			acc.Nodes[node.ID] = node
			acc.ActiveID = node.ID
			_ = p.store.UpsertNode(context.Background(), store.NodeRecord{ID: node.ID, Name: node.Name, BaseURL: node.URL.String(), HealthCheckMethod: node.HealthCheckMethod, AccountID: acc.ID, Weight: node.Weight, CreatedAt: node.CreatedAt})
			_ = p.store.SetActive(context.Background(), acc.ID, node.ID)
		} else {
			for _, r := range recs {
				u, _ := url.Parse(r.BaseURL)
				hcMethod := normalizeHealthCheckMethod(chooseNonEmpty(r.HealthCheckMethod, defaultHealthCheckMethod))
				if healthMethodRequiresAPIKey(hcMethod) && r.APIKey == "" {
					p.logger.Printf("health check mode %s requires api key, fallback to head for node %s", hcMethod, r.Name)
					hcMethod = HealthCheckMethodHEAD
				}
				n := &Node{
					ID:                r.ID,
					Name:              r.Name,
					URL:               u,
					APIKey:            r.APIKey,
					HealthCheckMethod: hcMethod,
					AccountID:         r.AccountID,
					CreatedAt:         r.CreatedAt,
					Weight:            r.Weight,
					Failed:            r.Failed,
					Disabled:          r.Disabled,
					LastError:         r.LastError,
					Window:            NewMetricsWindow(cfg.WindowSize),
					Metrics: metrics{
						Requests:          r.Requests,
						FailCount:         r.FailCount,
						FailStreak:        r.FailStreak,
						TotalBytes:        r.TotalBytes,
						TotalInputTokens:  r.TotalInput,
						TotalOutputTokens: r.TotalOutput,
						StreamDur:         time.Duration(r.StreamDurMs) * time.Millisecond,
						FirstByteDur:      time.Duration(r.FirstByteMs) * time.Millisecond,
						LastPingMS:        r.LastPingMs,
						LastPingErr:       r.LastPingErr,
						LastHealthCheckAt: r.LastHealthCheckAt,
					},
				}
				n.Score = CalculateScore(n, cfg.AlphaErr, cfg.BetaLatency)
				acc.Nodes[n.ID] = n
				// 重启后恢复失败节点到 FailedSet，确保健康检查能够探活这些节点
				if n.Failed {
					acc.FailedSet[n.ID] = struct{}{}
				}
			}
		}
		p.registerAccount(acc)
	}
	// 如果未找到默认账号，则返回 nil 让上层创建。
	return nil
}

func (p *Server) registerAccount(acc *Account) {
	if acc == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accountByID[acc.ID] = acc
	if acc.ProxyAPIKey != "" {
		p.accounts[acc.ProxyAPIKey] = acc
	}
	if acc.ID == store.DefaultAccountID {
		if p.defaultAccName != "" && acc.Name != p.defaultAccName {
			acc.Name = p.defaultAccName
			if p.store != nil {
				_ = p.store.UpdateAccount(context.Background(), store.AccountRecord{
					ID:   acc.ID,
					Name: acc.Name,
				})
			}
		}
		p.defaultAccount = acc
		if rt, ok := p.transport.(*retryTransport); ok && acc.Config.Retries > 0 {
			rt.attempts = acc.Config.Retries
		}
		p.retries = acc.Config.Retries
		p.failLimit = acc.Config.FailLimit
		p.healthEvery = acc.Config.HealthEvery
		p.cooldown = acc.Config.Cooldown
		p.minHealthy = acc.Config.MinHealthy
		if acc.Config.HealthConcurrency > 0 {
			p.healthWorkers = acc.Config.HealthConcurrency
		}
		if acc.Config.WindowSize > 0 {
			p.windowSize = acc.Config.WindowSize
		} else if p.windowSize == 0 {
			p.windowSize = 200
		}
		if acc.Config.AlphaErr != 0 {
			p.alphaErr = acc.Config.AlphaErr
		} else if p.alphaErr == 0 {
			p.alphaErr = 5.0
		}
		if acc.Config.BetaLatency != 0 {
			p.betaLatency = acc.Config.BetaLatency
		} else if p.betaLatency == 0 {
			p.betaLatency = 0.5
		}
	}
	for id, n := range acc.Nodes {
		p.nodeIndex[id] = n
		p.nodeAccount[id] = acc
		// 确保节点属于账号
		n.AccountID = acc.ID
	}
}

func (p *Server) getAccountByProxyKey(key string) *Account {
	if key == "" {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.accounts[key]
}

func (p *Server) getAccountByID(id string) *Account {
	if id == "" {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.accountByID[id]
}

func (p *Server) publishTunnelEvent(eventType, title, content string) {
	if p.notifyMgr == nil {
		return
	}
	accID := store.DefaultAccountID
	if p.defaultAccount != nil && p.defaultAccount.ID != "" {
		accID = p.defaultAccount.ID
	}
	p.notifyMgr.Publish(notify.Event{
		AccountID:  accID,
		EventType:  eventType,
		Title:      title,
		Content:    content,
		DedupKey:   "tunnel",
		OccurredAt: time.Now(),
	})
}

// StartTunnel 根据存储配置启动 Cloudflare Tunnel。
func (p *Server) StartTunnel() error {
	if p.store == nil {
		err := errors.New("未启用存储，无法读取隧道配置")
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}

	p.tunnelMu.Lock()
	if p.tunnelMgr != nil {
		p.tunnelMu.Unlock()
		err := errors.New("隧道已运行")
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}
	p.tunnelMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cfg, err := p.store.GetTunnelConfig(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			err = errors.New("尚未保存隧道配置")
		}
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}
	if cfg.APIToken == "" || cfg.Subdomain == "" {
		err := errors.New("api_token 与 subdomain 不能为空")
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}

	mgr, err := tunnel.NewManager(tunnel.TunnelConfig{
		APIToken:  cfg.APIToken,
		Subdomain: cfg.Subdomain,
		LocalAddr: buildLocalURL(p.listenAddr),
		Zone:      cfg.Zone,
	})
	if err != nil {
		_ = p.updateTunnelStatus(ctx, cfg, "error", errString(err), cfg.PublicURL, cfg.Enabled)
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}

	localURL := buildLocalURL(p.listenAddr)
	if err := mgr.Start(context.Background(), localURL); err != nil {
		_ = p.updateTunnelStatus(ctx, cfg, "error", errString(err), cfg.PublicURL, cfg.Enabled)
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}

	cfg.PublicURL = mgr.GetPublicURL()
	cfg.Status = "running"
	cfg.LastError = ""
	cfg.Enabled = true
	if err := p.store.SaveTunnelConfig(ctx, *cfg); err != nil {
		_ = mgr.Stop()
		p.publishTunnelEvent(notify.EventSystemTunnelError, "隧道启动失败", fmt.Sprintf("**错误**: %s", err.Error()))
		return err
	}

	p.tunnelMu.Lock()
	p.tunnelMgr = mgr
	p.tunnelMu.Unlock()

	p.publishTunnelEvent(notify.EventSystemTunnelStarted, "隧道已启动", fmt.Sprintf("**子域名**: %s\n**公网地址**: %s", cfg.Subdomain, cfg.PublicURL))
	return nil
}

// StopTunnel 停止隧道并更新状态。
func (p *Server) StopTunnel() error {
	p.tunnelMu.Lock()
	mgr := p.tunnelMgr
	p.tunnelMgr = nil
	p.tunnelMu.Unlock()

	var stopErr error
	if mgr != nil {
		stopErr = mgr.Stop()
	}

	if p.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cfg, err := p.store.GetTunnelConfig(ctx)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if cfg == nil {
			cfg = &store.TunnelConfig{ID: "default"}
		}
		cfg.Status = "stopped"
		cfg.PublicURL = ""
		cfg.Enabled = false
		cfg.LastError = errString(stopErr)
		_ = p.store.SaveTunnelConfig(ctx, *cfg)
	}

	p.publishTunnelEvent(notify.EventSystemTunnelStopped, "隧道已停止", fmt.Sprintf("**状态**: %s", chooseNonEmpty(errString(stopErr), "正常停止")))
	return stopErr
}

// GetTunnelStatus 汇总持久化与运行时状态。
func (p *Server) GetTunnelStatus() TunnelStatus {
	status := TunnelStatus{Status: "stopped"}
	if p.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cfg, err := p.store.GetTunnelConfig(ctx)
		cancel()
		if err == nil && cfg != nil {
			status.APITokenSet = cfg.APIToken != ""
			status.Subdomain = cfg.Subdomain
			status.Zone = cfg.Zone
			status.Enabled = cfg.Enabled
			status.PublicURL = cfg.PublicURL
			status.Status = chooseNonEmpty(cfg.Status, status.Status)
			status.LastError = cfg.LastError
		}
	}

	p.tunnelMu.Lock()
	running := p.tunnelMgr != nil
	public := ""
	if p.tunnelMgr != nil {
		public = p.tunnelMgr.GetPublicURL()
	}
	p.tunnelMu.Unlock()

	if running {
		status.Status = "running"
		status.Enabled = true
		if public != "" {
			status.PublicURL = public
		}
		status.LastError = ""
	}
	return status
}

// SaveTunnelConfig 便于启动时写入隧道配置。
func (p *Server) SaveTunnelConfig(ctx context.Context, cfg store.TunnelConfig) error {
	if p.store == nil {
		return errors.New("未启用存储")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return p.store.SaveTunnelConfig(ctx, cfg)
}

func (p *Server) updateTunnelStatus(ctx context.Context, cfg *store.TunnelConfig, status, lastErr, publicURL string, enabled bool) error {
	if p.store == nil || cfg == nil {
		return nil
	}
	cfg.Status = status
	cfg.LastError = lastErr
	cfg.PublicURL = publicURL
	cfg.Enabled = enabled
	return p.store.SaveTunnelConfig(ctx, *cfg)
}

// updateHealthInterval 在运行时调整健康检查间隔（秒级）。
func (p *Server) updateHealthInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	p.mu.Lock()
	for _, acc := range p.accountByID {
		if acc != nil {
			acc.Config.HealthEvery = interval
		}
	}
	p.healthEvery = interval
	p.mu.Unlock()
}

// updateRetryMax 在运行时调整重试次数。
func (p *Server) updateRetryMax(max int) {
	if max <= 0 {
		return
	}
	p.mu.Lock()
	for _, acc := range p.accountByID {
		if acc != nil {
			acc.Config.Retries = max
		}
	}
	p.retries = max
	p.mu.Unlock()

	if rt, ok := p.transport.(*retryTransport); ok {
		rt.attempts = max
	}
}

// updateFailLimit 在运行时调整失败阈值。
func (p *Server) updateFailLimit(limit int) {
	if limit <= 0 {
		return
	}
	p.mu.Lock()
	for _, acc := range p.accountByID {
		if acc != nil {
			acc.Config.FailLimit = limit
		}
	}
	p.failLimit = limit
	p.mu.Unlock()
}

func buildLocalURL(listenAddr string) string {
	if strings.HasPrefix(listenAddr, "http://") || strings.HasPrefix(listenAddr, "https://") {
		return listenAddr
	}
	host := listenAddr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return host
}
