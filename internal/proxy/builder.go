package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
)

// Builder 使用流式接口构建 Server 实例。
type Builder struct {
	upstreamRaw        string
	upstreamKey        string
	upstreamName       string
	listenAddr         string
	transport          http.RoundTripper
	logger             *log.Logger
	retries            int
	failLimit          int
	healthEvery        time.Duration
	storeDSN           string
	adminKey           string
	defaultAccountName string
	defaultProxyKey    string
	cliRunner          CliRunner
}

// NewBuilder 构建带默认监听地址和日志的 Builder。
func NewBuilder() *Builder {
	return &Builder{listenAddr: ":8000", logger: log.Default(), upstreamName: "default", retries: 3, failLimit: 3, healthEvery: 30 * time.Second}
}

func chooseNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// WithUpstream 设置默认上游地址（必填）。
func (b *Builder) WithUpstream(upstream string) *Builder {
	b.upstreamRaw = upstream
	return b
}

// WithAPIKey 为默认上游设置 API Key（可选）。
func (b *Builder) WithAPIKey(key string) *Builder {
	b.upstreamKey = key
	return b
}

// WithNodeName 设置默认节点名称（可选，默认 default）。
func (b *Builder) WithNodeName(name string) *Builder {
	if name != "" {
		b.upstreamName = name
	}
	return b
}

// WithListenAddr 覆盖监听地址。
func (b *Builder) WithListenAddr(addr string) *Builder {
	b.listenAddr = addr
	return b
}

// WithTransport 注入自定义 RoundTripper；默认为 http.DefaultTransport。
func (b *Builder) WithTransport(t http.RoundTripper) *Builder {
	b.transport = t
	return b
}

// WithCLIRunner 用于测试时覆盖 CLI 健康检查执行逻辑。
func (b *Builder) WithCLIRunner(r CliRunner) *Builder {
	b.cliRunner = r
	return b
}

// WithFailLimit 设置连续失败（非 200）后标记节点不可用的阈值。
func (b *Builder) WithFailLimit(n int) *Builder {
	if n > 0 {
		b.failLimit = n
	}
	return b
}

// WithHealthEvery 设置失败节点的探活间隔。
func (b *Builder) WithHealthEvery(d time.Duration) *Builder {
	if d > 0 {
		b.healthEvery = d
	}
	return b
}

// WithAdminKey 设置管理员访问密钥。
func (b *Builder) WithAdminKey(key string) *Builder {
	b.adminKey = key
	return b
}

// WithDefaultAccountName 设置默认账号名称（仅在首次初始化时生效）。
func (b *Builder) WithDefaultAccountName(name string) *Builder {
	if name != "" {
		b.defaultAccountName = name
	}
	return b
}

// WithDefaultAccount 设置默认账号信息。
func (b *Builder) WithDefaultAccount(name, proxyKey string) *Builder {
	if name != "" {
		b.defaultAccountName = name
	}
	if proxyKey != "" {
		b.defaultProxyKey = proxyKey
	}
	return b
}

// WithStoreDSN 传入 MySQL DSN 以启用持久化。
func (b *Builder) WithStoreDSN(dsn string) *Builder {
	b.storeDSN = dsn
	return b
}

// WithRetry 设置非 200 状态时的重试次数（最少 1）。
func (b *Builder) WithRetry(times int) *Builder {
	if times > 0 {
		b.retries = times
	}
	return b
}

// WithLogger 设置日志器；默认 log.Default()。
func (b *Builder) WithLogger(l *log.Logger) *Builder {
	b.logger = l
	return b
}

// Build 校验输入并生成 Server。
func (b *Builder) Build() (*Server, error) {
	if b.upstreamRaw == "" {
		return nil, ErrUpstreamMissing
	}
	parsed, err := url.Parse(b.upstreamRaw)
	if err != nil {
		return nil, err
	}
	transport := b.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	logger := b.logger
	if logger == nil {
		logger = log.Default()
	}
	healthRT := transport
	transport = &retryTransport{base: transport, attempts: b.retries, logger: logger}

	var st *store.Store
	if b.storeDSN != "" {
		st, err = store.Open(b.storeDSN)
		if err != nil {
			return nil, err
		}
	}

	adminKey := b.adminKey
	if adminKey == "" {
		adminKey = "admin"
	}
	defaultAccountName := b.defaultAccountName
	if defaultAccountName == "" {
		defaultAccountName = "default"
	}
	defaultProxyKey := b.defaultProxyKey
	if defaultProxyKey == "" {
		defaultProxyKey = "default-proxy-key"
	}
	runner := defaultCLIRunner
	if b.cliRunner != nil {
		runner = b.cliRunner
	}

	srv := &Server{
		accounts:       make(map[string]*Account),
		accountByID:    make(map[string]*Account),
		nodeIndex:      make(map[string]*Node),
		nodeAccount:    make(map[string]*Account),
		listenAddr:     b.listenAddr,
		transport:      transport,
		healthRT:       healthRT,
		cliRunner:      runner,
		logger:         logger,
		store:          st,
		adminKey:       adminKey,
		defaultAccName: defaultAccountName,
		sessionMgr:     NewSessionManager(defaultSessionTTL),
	}

	if st != nil {
		srv.notifyMgr = notify.NewManager(notify.NewStoreAdapter(st), notify.WithLogger(logger))
	}

	if rt, ok := transport.(*retryTransport); ok {
		rt.notifyMgr = srv.notifyMgr
	}

	defaultCfg := store.Config{Retries: b.retries, FailLimit: b.failLimit, HealthEvery: b.healthEvery}
	srv.retries = defaultCfg.Retries
	srv.failLimit = defaultCfg.FailLimit
	srv.healthEvery = defaultCfg.HealthEvery

	if st != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		accounts, err := st.ListAccounts(ctx)
		if err != nil {
			return nil, err
		}

		hasAdmin := false
		now := time.Now()
		for _, acc := range accounts {
			if acc.ID == store.DefaultAccountID {
				record := store.AccountRecord{
					ID:          acc.ID,
					Name:        chooseNonEmpty(acc.Name, defaultAccountName),
					Password:    chooseNonEmpty(acc.Password, "default123"),
					ProxyAPIKey: chooseNonEmpty(acc.ProxyAPIKey, defaultProxyKey),
					IsAdmin:     acc.IsAdmin,
				}
				if record.Password != acc.Password || record.ProxyAPIKey != acc.ProxyAPIKey || record.Name != acc.Name {
					_ = st.UpdateAccount(ctx, record)
				}
				continue
			}

			if acc.IsAdmin {
				hasAdmin = true
				record := store.AccountRecord{
					ID:          acc.ID,
					Name:        chooseNonEmpty(acc.Name, "admin"),
					Password:    chooseNonEmpty(acc.Password, "admin123"),
					ProxyAPIKey: chooseNonEmpty(acc.ProxyAPIKey, adminKey),
					IsAdmin:     true,
				}
				if record.Password != acc.Password || record.ProxyAPIKey != acc.ProxyAPIKey || record.Name != acc.Name {
					_ = st.UpdateAccount(ctx, record)
				}
			}
		}

		if !hasAdmin {
			adminAccount := store.AccountRecord{
				ID:          fmt.Sprintf("admin-%d", now.UnixNano()),
				Name:        "admin",
				Password:    "admin123",
				ProxyAPIKey: adminKey,
				IsAdmin:     true,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := st.CreateAccount(ctx, adminAccount); err != nil {
				return nil, fmt.Errorf("failed to create admin account: %w", err)
			}
		}

		// default 账号自动创建已禁用，避免重启后恢复已手动删除的 default 账号。如需该账号，请在存储层手动创建。

		if err := srv.loadAccountsFromStore(parsed, defaultCfg, b.upstreamKey); err != nil {
			return nil, err
		}
	} else {
		// 内存模式：创建管理员与默认账号，并附加默认节点。
		adminAccount := &Account{
			ID:          "admin-mem",
			Name:        "admin",
			Password:    "admin123",
			ProxyAPIKey: adminKey,
			IsAdmin:     true,
			Config:      Config{Retries: defaultCfg.Retries, FailLimit: defaultCfg.FailLimit, HealthEvery: defaultCfg.HealthEvery},
			Nodes:       make(map[string]*Node),
			FailedSet:   make(map[string]struct{}),
		}

		defaultAccount := &Account{
			ID:          store.DefaultAccountID,
			Name:        defaultAccountName,
			Password:    "default123",
			ProxyAPIKey: defaultProxyKey,
			IsAdmin:     false,
			Config:      Config{Retries: defaultCfg.Retries, FailLimit: defaultCfg.FailLimit, HealthEvery: defaultCfg.HealthEvery},
			Nodes:       make(map[string]*Node),
			FailedSet:   make(map[string]struct{}),
		}

		node := &Node{
			ID:        "default",
			Name:      b.upstreamName,
			URL:       parsed,
			APIKey:    b.upstreamKey,
			AccountID: defaultAccount.ID,
			CreatedAt: time.Now(),
			Weight:    1,
		}
		defaultAccount.Nodes[node.ID] = node
		defaultAccount.ActiveID = node.ID

		srv.accounts[adminAccount.ProxyAPIKey] = adminAccount
		srv.accounts[defaultAccount.ProxyAPIKey] = defaultAccount
		srv.accountByID[adminAccount.ID] = adminAccount
		srv.accountByID[defaultAccount.ID] = defaultAccount
		srv.nodeIndex[node.ID] = node
		srv.nodeAccount[node.ID] = defaultAccount
		srv.defaultAccount = defaultAccount
		srv.activeID = node.ID
	}

	if srv.defaultAccount != nil && srv.defaultAccount.ActiveID == "" {
		for id := range srv.defaultAccount.Nodes {
			srv.defaultAccount.ActiveID = id
			srv.activeID = id
			break
		}
	}

	if srv.store != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cfg, err := srv.store.GetTunnelConfig(ctx)
			cancel()
			if err == nil && cfg != nil && cfg.Enabled {
				if err := srv.StartTunnel(); err != nil {
					srv.logger.Printf("auto start tunnel failed: %v", err)
				}
			}
		}()
	}

	return srv, nil
}
