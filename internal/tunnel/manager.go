package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Manager 管理隧道生命周期。
type Manager struct {
	cfg       TunnelConfig
	client    *Client
	logger    *log.Logger
	accountID string
	zone      Zone
	tunnel    *Tunnel
	record    *DNSRecord
	cmd       *exec.Cmd
	publicURL string

	mu sync.Mutex
}

// NewManager 创建隧道管理器。
func NewManager(cfg TunnelConfig) (*Manager, error) {
	if cfg.APIToken == "" {
		return nil, errors.New("CF_API_TOKEN 不能为空")
	}
	if cfg.Subdomain == "" {
		return nil, errors.New("TUNNEL_SUBDOMAIN 不能为空")
	}
	m := &Manager{
		cfg:    cfg,
		client: NewClient(cfg.APIToken),
		logger: log.Default(),
	}
	return m, nil
}

// Start 创建隧道、配置 DNS，并启动 cloudflared。
func (m *Manager) Start(ctx context.Context, localAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil {
		return errors.New("隧道已运行")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if localAddr == "" {
		localAddr = m.cfg.LocalAddr
	}
	if localAddr == "" {
		return errors.New("本地服务地址不能为空")
	}

	shortCtx := func(d time.Duration) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, d)
	}

	// 1) 获取账号 ID
	cctx, cancel := shortCtx(10 * time.Second)
	accountID, err := m.client.GetAccountID(cctx)
	cancel()
	if err != nil {
		return fmt.Errorf("获取 Cloudflare 账号失败: %w", err)
	}
	m.accountID = accountID

	// 2) 选择 Zone
	cctx, cancel = shortCtx(15 * time.Second)
	zones, err := m.client.ListZones(cctx)
	cancel()
	if err != nil {
		return fmt.Errorf("获取域名列表失败: %w", err)
	}
	if len(zones) == 0 {
		return errors.New("账号下没有可用域名")
	}
	m.zone = zones[0]
	if m.cfg.Zone != "" {
		found := false
		for _, z := range zones {
			if strings.EqualFold(z.Name, m.cfg.Zone) {
				m.zone = z
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("未找到指定域名: %s", m.cfg.Zone)
		}
	}

	hostname := fmt.Sprintf("%s.%s", m.cfg.Subdomain, m.zone.Name)

	// 3) 创建隧道
	secret, err := randomSecret()
	if err != nil {
		return fmt.Errorf("生成隧道密钥失败: %w", err)
	}
	tunnelName := fmt.Sprintf("qcc-%s-%d", m.cfg.Subdomain, time.Now().Unix())
	cctx, cancel = shortCtx(15 * time.Second)
	tun, err := m.client.CreateTunnel(cctx, m.accountID, tunnelName, secret)
	cancel()
	if err != nil {
		return fmt.Errorf("创建隧道失败: %w", err)
	}
	m.tunnel = tun

	// 4) 配置隧道转发到本地
	cctx, cancel = shortCtx(15 * time.Second)
	if err := m.client.updateTunnelConfig(cctx, m.accountID, m.tunnel.ID, hostname, localAddr); err != nil {
		cancel()
		_ = m.cleanupTunnel()
		return fmt.Errorf("配置隧道转发失败: %w", err)
	}
	cancel()

	// 5) 配置 DNS
	target := fmt.Sprintf("%s.cfargotunnel.com", m.tunnel.ID)
	cctx, cancel = shortCtx(10 * time.Second)
	existing, err := m.client.FindDNSRecord(cctx, m.zone.ID, hostname)
	cancel()
	if err != nil {
		_ = m.cleanupTunnel()
		return fmt.Errorf("查询 DNS 记录失败: %w", err)
	}
	if existing != nil && !strings.EqualFold(existing.Content, target) {
		cctx, cancel = shortCtx(10 * time.Second)
		_ = m.client.DeleteDNSRecord(cctx, m.zone.ID, existing.ID)
		cancel()
		existing = nil
	}
	if existing == nil {
		cctx, cancel = shortCtx(10 * time.Second)
		if err := m.client.CreateDNSRecord(cctx, m.zone.ID, hostname, m.tunnel.ID); err != nil {
			cancel()
			_ = m.cleanupTunnel()
			return fmt.Errorf("创建 DNS 记录失败: %w", err)
		}
		cancel()
		cctx, cancel = shortCtx(8 * time.Second)
		created, _ := m.client.FindDNSRecord(cctx, m.zone.ID, hostname)
		cancel()
		if created != nil {
			m.record = created
		} else {
			m.record = &DNSRecord{Name: hostname, Content: target, Type: "CNAME"}
		}
	} else {
		m.record = existing
	}

	// 6) 获取 token 并启动 cloudflared
	cctx, cancel = shortCtx(10 * time.Second)
	token, err := m.client.GetTunnelToken(cctx, m.accountID, m.tunnel.ID)
	cancel()
	if err != nil {
		_ = m.cleanupTunnel()
		return fmt.Errorf("获取隧道 token 失败: %w", err)
	}

	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--no-autoupdate", "run", "--token", token)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = m.cleanupTunnel()
		return fmt.Errorf("启动 cloudflared 失败: %w", err)
	}
	m.cmd = cmd
	m.publicURL = "https://" + hostname
	m.logger.Printf("Cloudflare Tunnel 已启动: %s -> %s (zone: %s)", m.publicURL, localAddr, m.zone.Name)

	return nil
}

// Stop 停止 cloudflared 并清理远端资源。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string

	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- m.cmd.Wait() }()
		select {
		case <-time.After(5 * time.Second):
			_ = m.cmd.Process.Kill()
		case <-done:
		}
		m.cmd = nil
	}

	if err := m.cleanupDNS(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := m.cleanupTunnel(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

// GetPublicURL 返回公网访问地址。
func (m *Manager) GetPublicURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publicURL
}

func (m *Manager) cleanupDNS() error {
	if m.record == nil || m.zone.ID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if m.record.ID == "" {
		found, _ := m.client.FindDNSRecord(ctx, m.zone.ID, m.record.Name)
		if found != nil {
			m.record.ID = found.ID
		}
	}
	if m.record.ID == "" {
		return nil
	}
	if err := m.client.DeleteDNSRecord(ctx, m.zone.ID, m.record.ID); err != nil {
		return fmt.Errorf("删除 DNS 记录失败: %w", err)
	}
	m.record = nil
	return nil
}

func (m *Manager) cleanupTunnel() error {
	if m.tunnel == nil || m.accountID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.client.DeleteTunnel(ctx, m.accountID, m.tunnel.ID); err != nil {
		return fmt.Errorf("删除隧道失败: %w", err)
	}
	m.tunnel = nil
	return nil
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
