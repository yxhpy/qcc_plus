package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"qcc_plus/internal/client"
	"qcc_plus/internal/proxy"
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "proxy" {
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
			Build()
		if err != nil {
			log.Fatal(err)
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
