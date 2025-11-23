package tunnel

// TunnelConfig 描述 Cloudflare Tunnel 所需的基础配置。
type TunnelConfig struct {
	APIToken  string // Cloudflare API Token
	Subdomain string // 期望暴露的子域名前缀
	LocalAddr string // 本地服务地址，例如 http://127.0.0.1:8000
	Zone      string // 可选：指定使用的根域名，不填则使用账号下第一个
}

// Zone 表示 Cloudflare Zone。
type Zone struct {
	ID   string
	Name string
}

// Tunnel 表示 Cloudflare Tunnel。
type Tunnel struct {
	ID     string
	Name   string
	Secret string
}

// DNSRecord 表示一个 DNS 记录。
type DNSRecord struct {
	ID      string
	Name    string
	Content string
	Type    string
}
