package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"qcc_plus/internal/client"
	"qcc_plus/internal/proxy"
	"qcc_plus/internal/store"
	"qcc_plus/internal/version"
)

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "proxy" {
		info := version.GetVersionInfo()
		log.Printf("qcc_plus version: %s (commit=%s, build_utc=%s, build_bj=%s, go=%s)", info.Version, info.GitCommit, info.BuildDate, info.BuildDateBeijing, info.GoVersion)

		upstreamRaw := firstNonEmpty(os.Getenv("UPSTREAM_BASE_URL"), os.Getenv("ANTHROPIC_BASE_URL"), "https://api.anthropic.com")
		upstreamKey := firstNonEmpty(os.Getenv("UPSTREAM_API_KEY"), os.Getenv("ANTHROPIC_API_KEY"))
		nodeName := getenvDefault("UPSTREAM_NAME", "default")
		listenAddr := getenvDefault("LISTEN_ADDR", ":8000")
		retryMax := 3
		if v := os.Getenv("PROXY_RETRY_MAX"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				retryMax = n
			}
		}
		failLimit := retryMax
		if v := os.Getenv("PROXY_FAIL_THRESHOLD"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				failLimit = n
			}
		}
		healthEvery := 30 * time.Second
		if v := os.Getenv("PROXY_HEALTH_INTERVAL_SEC"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				healthEvery = time.Duration(n) * time.Second
			}
		}
		mysqlDSN := os.Getenv("PROXY_MYSQL_DSN")
		adminKey := os.Getenv("ADMIN_API_KEY")
		if adminKey == "" {
			adminKey = "admin"
			log.Println("WARNING: ADMIN_API_KEY not set, using default: 'admin' (change it in production)")
		}

		defaultAccountName := getenvDefault("DEFAULT_ACCOUNT_NAME", "default")
		defaultProxyKey := os.Getenv("DEFAULT_PROXY_API_KEY")
		if defaultProxyKey == "" {
			defaultProxyKey = "default-proxy-key"
			log.Println("WARNING: DEFAULT_PROXY_API_KEY not set, using default: 'default-proxy-key' (change it in production)")
		}

		srv, err := proxy.NewBuilder().
			WithUpstream(upstreamRaw).
			WithAPIKey(upstreamKey).
			WithNodeName(nodeName).
			WithListenAddr(listenAddr).
			WithRetry(retryMax).
			WithFailLimit(failLimit).
			WithHealthEvery(healthEvery).
			WithStoreDSN(mysqlDSN).
			WithAdminKey(adminKey).
			WithDefaultAccount(defaultAccountName, defaultProxyKey).
			WithTransport(nil).
			WithEnv().
			Build()
		if err != nil {
			log.Fatal(err)
		}

		cfToken := os.Getenv("CF_API_TOKEN")
		tunnelSubdomain := os.Getenv("TUNNEL_SUBDOMAIN")
		tunnelZone := os.Getenv("TUNNEL_ZONE")
		tunnelEnabled := os.Getenv("TUNNEL_ENABLED") == "1" || strings.EqualFold(os.Getenv("TUNNEL_ENABLED"), "true")

		if tunnelEnabled && cfToken != "" && tunnelSubdomain != "" {
			if err := srv.SaveTunnelConfig(context.Background(), store.TunnelConfig{
				ID:        "default",
				APIToken:  cfToken,
				Subdomain: tunnelSubdomain,
				Zone:      tunnelZone,
				Enabled:   true,
			}); err != nil {
				log.Printf("保存隧道配置失败: %v", err)
			}
			if err := srv.StartTunnel(); err != nil {
				log.Printf("启动 Cloudflare Tunnel 失败: %v", err)
			} else {
				log.Printf("Cloudflare Tunnel 已开启，公网地址: %s", srv.GetTunnelStatus().PublicURL)
			}
		}

		if err := srv.Start(); err != nil {
			log.Fatal(err)
		}
		return
	}

	cfg, err := client.LoadConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if err := client.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
