package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"qcc_plus/internal/store"
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

	listenAddr  string
	transport   http.RoundTripper
	logger      *log.Logger
	retries     int
	failLimit   int
	healthEvery time.Duration
	healthRT    http.RoundTripper
	store       *store.Store
	adminKey    string
}

// Start 运行反向代理并阻塞直到关闭。
func (p *Server) Start() error {
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

// Handler 暴露 HTTP 处理器，便于测试或自定义服务器。
func (p *Server) Handler() http.Handler {
	return p.handler()
}

// 创建默认账号及默认节点（如必要）。
func (p *Server) createDefaultAccount(defaultUpstream *url.URL, defaultCfg store.Config, name, proxyKey, upstreamKey string) error {
	acc := &Account{
		ID:          store.DefaultAccountID,
		Name:        chooseNonEmpty(name, "default"),
		Password:    "default123",
		ProxyAPIKey: proxyKey,
		IsAdmin:     true,
		Config:      Config{Retries: defaultCfg.Retries, FailLimit: defaultCfg.FailLimit, HealthEvery: defaultCfg.HealthEvery},
		Nodes:       make(map[string]*Node),
		FailedSet:   make(map[string]struct{}),
	}
	if defaultUpstream == nil {
		return errors.New("default upstream required for initial account")
	}
	node := &Node{
		ID:        "default",
		Name:      chooseNonEmpty(defaultUpstream.Host, "default"),
		URL:       defaultUpstream,
		APIKey:    upstreamKey,
		AccountID: acc.ID,
		CreatedAt: time.Now(),
		Weight:    1,
	}
	acc.Nodes[node.ID] = node
	acc.ActiveID = node.ID
	p.registerAccount(acc)
	if p.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.store.CreateAccount(ctx, store.AccountRecord{ID: acc.ID, Name: acc.Name, Password: acc.Password, ProxyAPIKey: acc.ProxyAPIKey, IsAdmin: true, CreatedAt: node.CreatedAt, UpdatedAt: node.CreatedAt})
		_ = p.store.UpsertNode(ctx, store.NodeRecord{ID: node.ID, Name: node.Name, BaseURL: node.URL.String(), APIKey: node.APIKey, AccountID: acc.ID, Weight: node.Weight, CreatedAt: node.CreatedAt})
		_ = p.store.SetActive(ctx, acc.ID, node.ID)
		_ = p.store.UpdateConfig(ctx, acc.ID, defaultCfg, node.ID)
	}
	return nil
}

// 从持久层加载账号、节点与配置。
func (p *Server) loadAccountsFromStore(defaultUpstream *url.URL, defaultCfg store.Config, defaultUpstreamKey string) error {
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
		cfg := Config{Retries: defaultCfg.Retries, FailLimit: defaultCfg.FailLimit, HealthEvery: defaultCfg.HealthEvery}
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
			node := &Node{
				ID:        "default",
				Name:      chooseNonEmpty(defaultUpstream.Host, "default"),
				URL:       defaultUpstream,
				APIKey:    defaultUpstreamKey,
				AccountID: acc.ID,
				CreatedAt: time.Now(),
				Weight:    1,
			}
			acc.Nodes[node.ID] = node
			acc.ActiveID = node.ID
			_ = p.store.UpsertNode(context.Background(), store.NodeRecord{ID: node.ID, Name: node.Name, BaseURL: node.URL.String(), AccountID: acc.ID, Weight: node.Weight, CreatedAt: node.CreatedAt})
			_ = p.store.SetActive(context.Background(), acc.ID, node.ID)
		} else {
			for _, r := range recs {
				u, _ := url.Parse(r.BaseURL)
				n := &Node{
					ID:        r.ID,
					Name:      r.Name,
					URL:       u,
					APIKey:    r.APIKey,
					AccountID: r.AccountID,
					CreatedAt: r.CreatedAt,
					Weight:    r.Weight,
					Failed:    r.Failed,
					Disabled:  r.Disabled,
					LastError: r.LastError,
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
					},
				}
				acc.Nodes[n.ID] = n
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
