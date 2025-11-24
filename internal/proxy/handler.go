package proxy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"qcc_plus/internal/version"
	"qcc_plus/web"
)

func spaFileExists(fsys fs.FS, name string) bool {
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		name = "index.html"
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

func spaHandler(fsys fs.FS) http.HandlerFunc {
	// 读取 index.html 内容用于 SPA 路由
	indexContent, _ := fs.ReadFile(fsys, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if spaFileExists(fsys, path) {
			// 服务静态文件
			f, err := fsys.Open(path)
			if err != nil {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			}
			defer f.Close()
			stat, _ := f.Stat()
			http.ServeContent(w, r, path, stat.ModTime(), f.(io.ReadSeeker))
			return
		}
		if len(indexContent) == 0 {
			http.Error(w, "index not found", http.StatusNotFound)
			return
		}
		// 对于 SPA 路由，直接返回 index.html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexContent)
	}
}

func (p *Server) handler() http.Handler {
	spaFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic(fmt.Sprintf("web assets missing: %v", err))
	}
	spa := spaHandler(spaFS)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/login", p.handleLogin)
	apiMux.HandleFunc("/logout", p.handleLogout)
	apiMux.HandleFunc("/admin/api/accounts", p.requireSession(p.handleAccounts))
	apiMux.HandleFunc("/admin/api/nodes", p.requireSession(p.handleNodes))
	apiMux.HandleFunc("/admin/api/config", p.requireSession(p.handleConfig))
	apiMux.HandleFunc("/admin/api/nodes/activate", p.requireSession(p.handleActivate))
	apiMux.HandleFunc("/admin/api/nodes/disable", p.requireSession(p.handleDisable))
	apiMux.HandleFunc("/admin/api/nodes/enable", p.requireSession(p.handleEnable))
	apiMux.HandleFunc("/admin/api/tunnel", p.requireSession(p.handleTunnelConfig))
	apiMux.HandleFunc("/admin/api/tunnel/start", p.requireSession(p.handleTunnelStart))
	apiMux.HandleFunc("/admin/api/tunnel/stop", p.requireSession(p.handleTunnelStop))
	apiMux.HandleFunc("/admin/api/tunnel/zones", p.requireSession(p.handleTunnelZones))
	apiMux.HandleFunc("/api/notification/channels", p.requireSession(p.handleNotificationChannels))
	apiMux.HandleFunc("/api/notification/channels/", p.requireSession(p.handleNotificationChannelByID))
	apiMux.HandleFunc("/api/notification/subscriptions", p.requireSession(p.handleNotificationSubscriptions))
	apiMux.HandleFunc("/api/notification/subscriptions/", p.requireSession(p.handleNotificationSubscriptionByID))
	apiMux.HandleFunc("/api/notification/event-types", p.requireSession(p.listEventTypes))
	apiMux.HandleFunc("/api/notification/test", p.requireSession(p.testNotification))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/version" {
			p.handleVersion(w, r)
			return
		}

		if path == "/changelog" {
			accept := r.Header.Get("Accept")
			if r.Header.Get("Sec-Fetch-Dest") == "document" || strings.Contains(accept, "text/html") {
				spa(w, r)
				return
			}
			p.handleChangelog(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/notification/") {
			apiMux.ServeHTTP(w, r)
			return
		}

		// API and auth endpoints
		if strings.HasPrefix(path, "/admin/api/") ||
			(path == "/login" && r.Method == http.MethodPost) ||
			path == "/logout" {
			apiMux.ServeHTTP(w, r)
			return
		}

		// SPA routes (admin UI and assets)
		if path == "/" || path == "/login" || path == "/admin" || path == "/index.html" ||
			strings.HasPrefix(path, "/admin/") || strings.HasPrefix(path, "/assets/") ||
			path == "/vite.svg" || path == "/favicon.ico" ||
			strings.HasPrefix(path, "/qcc-icon-") {
			spa(w, r)
			return
		}

		// Proxy endpoints (unchanged)
		proxyKey := extractAPIKey(r)
		account := p.getAccountByProxyKey(proxyKey)
		if account == nil {
			account = p.defaultAccount
		}
		if account == nil {
			http.Error(w, "account not found", http.StatusUnauthorized)
			return
		}

		node, err := p.getActiveNodeForAccount(account)
		if err != nil {
			http.Error(w, "no active upstream node", http.StatusServiceUnavailable)
			return
		}

		usage := &usage{}
		proxy := p.newReverseProxy(node, usage)
		p.logger.Printf("%s %s via %s (account=%s)", r.Method, r.URL.String(), node.Name, account.ID)

		start := time.Now()
		mw := &metricsWriter{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), accountContextKey{}, account)
		ctx = context.WithValue(ctx, nodeContextKey{}, node)
		proxy.ServeHTTP(mw, r.WithContext(ctx))

		p.recordMetrics(node.ID, start, mw, usage)
		if mw.status != http.StatusOK {
			errMsg := mw.Header().Get("X-Retry-Error")
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", mw.status)
			}
			p.handleFailure(node.ID, errMsg)
		}
	})
}

// requireSession 会话中间件，未登录则跳转登录页（页面请求）或返回 401（API 请求）。
func (p *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if p.sessionMgr == nil {
			http.Error(w, "session manager missing", http.StatusInternalServerError)
			return
		}

		// 判断是否为 API 请求
		isAPIRequest := strings.HasPrefix(r.URL.Path, "/admin/api/") || strings.HasPrefix(r.URL.Path, "/api/notification/")

		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value == "" {
			if isAPIRequest {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}
		sess := p.sessionMgr.Get(cookie.Value)
		if sess == nil {
			if isAPIRequest {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session invalid"})
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}
		acc := p.getAccountByID(sess.AccountID)
		if acc == nil {
			if p.defaultAccount != nil {
				acc = p.defaultAccount
			}
		}
		if acc == nil {
			p.sessionMgr.Delete(cookie.Value)
			if isAPIRequest {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "account not found"})
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}
		ctx := context.WithValue(r.Context(), accountContextKey{}, acc)
		if sess.IsAdmin {
			ctx = context.WithValue(ctx, isAdminContextKey{}, true)
		}
		next(w, r.WithContext(ctx))
	}
}

// extractAPIKey 从请求中提取代理 API Key。
func extractAPIKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if key := r.Header.Get("x-api-key"); key != "" {
		return key
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// requireAuth 鉴权中间件，支持管理员密钥或账号代理密钥。
func (p *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminKey := r.Header.Get("x-admin-key")
		if adminKey == "" {
			adminKey = r.URL.Query().Get("admin_key")
		}
		if adminKey == p.adminKey {
			ctx := context.WithValue(r.Context(), isAdminContextKey{}, true)
			if p.defaultAccount != nil {
				ctx = context.WithValue(ctx, accountContextKey{}, p.defaultAccount)
			}
			next(w, r.WithContext(ctx))
			return
		}

		proxyKey := extractAPIKey(r)
		account := p.getAccountByProxyKey(proxyKey)
		if account == nil {
			account = p.defaultAccount
		}
		if account == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), accountContextKey{}, account)
		if account.IsAdmin {
			ctx = context.WithValue(ctx, isAdminContextKey{}, true)
		}
		next(w, r.WithContext(ctx))
	}
}

func (p *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, version.GetVersionInfo())
}

func (p *Server) handleChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	paths := []string{"CHANGELOG.md"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "CHANGELOG.md"))
	}

	var content []byte
	var readErr error
	for _, path := range paths {
		content, readErr = os.ReadFile(path)
		if readErr == nil {
			break
		}
	}

	if readErr != nil {
		http.Error(w, "更新日志不存在，请确认仓库包含 CHANGELOG.md", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
