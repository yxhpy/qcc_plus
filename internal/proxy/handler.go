package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	apiMux.HandleFunc("/api/nodes/", p.requireSession(p.handleNodeAPIRoutes))
	apiMux.HandleFunc("/api/accounts/", p.requireSession(p.handleGetAccountMetrics))
	apiMux.HandleFunc("/api/metrics/aggregate", p.requireSession(p.handleAggregateMetrics))
	apiMux.HandleFunc("/api/metrics/cleanup", p.requireSession(p.handleCleanupMetrics))
	apiMux.HandleFunc("/api/monitor/dashboard", p.requireSession(p.handleMonitorDashboard))
	apiMux.HandleFunc("/api/monitor/shares", p.requireSession(p.handleMonitorShares))
	apiMux.HandleFunc("/api/monitor/shares/", p.requireSession(p.handleRevokeMonitorShare))
	apiMux.HandleFunc("/api/monitor/share/", p.handleAccessMonitorShare)
	settingsHandler := &SettingsHandler{store: p.store, cache: p.settingsCache}
	apiMux.HandleFunc("/api/settings/version", p.requireSession(settingsHandler.GetVersion))
	apiMux.HandleFunc("/api/settings", p.requireSession(settingsHandler.ListSettings))
	apiMux.HandleFunc("/api/settings/batch", p.requireSession(settingsHandler.BatchUpdate))
	apiMux.HandleFunc("/api/settings/", p.requireSession(settingsHandler.HandleSetting))
	apiMux.HandleFunc("/api/claude-config/template", p.requireSession(p.handleClaudeConfigTemplate))
	apiMux.HandleFunc("/api/claude-config/download/", p.handleClaudeConfigDownload)
	// 定价和使用统计 API
	apiMux.HandleFunc("/api/pricing", p.requireSession(p.handlePricing))
	apiMux.HandleFunc("/api/usage/logs", p.requireSession(p.handleUsageLogs))
	apiMux.HandleFunc("/api/usage/summary", p.requireSession(p.handleUsageSummary))
	apiMux.HandleFunc("/api/usage/cleanup", p.requireSession(p.handleUsageCleanup))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/version" {
			p.handleVersion(w, r)
			return
		}

		if path == "/api/monitor/ws" {
			p.handleMonitorWebSocket(w, r)
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

		// Allow shared health history access without session when share_token is present.
		if strings.HasPrefix(path, "/api/nodes/") && strings.HasSuffix(path, "/health-history") {
			if r.URL.Query().Get("share_token") != "" {
				p.handleNodeAPIRoutes(w, r)
				return
			}
			apiMux.ServeHTTP(w, r)
			return
		}

		if (strings.HasPrefix(path, "/api/nodes/") && strings.HasSuffix(path, "/metrics")) ||
			(strings.HasPrefix(path, "/api/accounts/") && strings.HasSuffix(path, "/metrics")) ||
			path == "/api/metrics/aggregate" || path == "/api/metrics/cleanup" {
			apiMux.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/monitor/") {
			apiMux.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/settings") {
			apiMux.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/pricing") || strings.HasPrefix(path, "/api/usage/") {
			apiMux.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/claude-config/") {
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
			strings.HasPrefix(path, "/monitor/") || strings.HasPrefix(path, "/settings") ||
			path == "/vite.svg" || path == "/favicon.ico" ||
			strings.HasPrefix(path, "/qcc-icon-") {
			spa(w, r)
			return
		}

		// 只代理 /v1/messages 接口，其他请求透传到上游
		if path == "/v1/messages" {
			// Proxy endpoints for /v1/messages
			proxyKey := extractAPIKey(r)
			account := p.getAccountByProxyKey(proxyKey)
			if account == nil {
				account = p.defaultAccount
			}
			if account == nil {
				http.Error(w, "account not found", http.StatusUnauthorized)
				return
			}

			skipNodes := make(map[string]bool)
			firstAttemptFailed := false
			baseCtx := context.WithValue(r.Context(), accountContextKey{}, account)
			baseCtx = context.WithValue(baseCtx, nodeContextKey{}, nil)
			overallDeadline := time.Time{}
			if p.retryConfig.TotalTimeout > 0 {
				overallDeadline = time.Now().Add(p.retryConfig.TotalTimeout)
			}
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body.Close()
			}

			// attempt 只计算真正发送请求的次数，maxLoops 防止无限循环
			// maxLoops = 节点数量 * 2，确保即使有熔断器也能尝试所有节点
			attempt := 0
			maxLoops := len(account.Nodes) * 2
			if maxLoops < 20 {
				maxLoops = 20 // 至少尝试 20 次循环
			}
			for loops := 0; loops < maxLoops; loops++ {
				reqForAttempt := r.Clone(baseCtx)
				if len(bodyBytes) > 0 {
					reqForAttempt.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					reqForAttempt.ContentLength = int64(len(bodyBytes))
				}
				node := p.selectHealthyNodeExcluding(account, skipNodes)
				if node == nil {
					break
				}

				// 检查熔断器
				var cb *CircuitBreaker
				if p.cbConfig.Enabled {
					cb = p.getOrCreateCircuitBreaker(node.ID)
					if !cb.AllowRequest() {
						p.logger.Printf("node %s circuit breaker is open, skipping (loop %d/%d, tried %d nodes)", node.Name, loops+1, maxLoops, attempt)
						skipNodes[node.ID] = true
						continue // 跳过此节点，不计入 attempt
					}
				}

				usage := &usage{}
				proxy, streamState := p.newReverseProxy(node, usage)
				p.logger.Printf("%s %s via %s (account=%s, node %d/%d)", r.Method, r.URL.String(), node.Name, account.ID, attempt+1, len(account.Nodes))

				start := time.Now()
				mw := &metricsWriter{ResponseWriter: w, status: http.StatusOK}

				// 计算本次尝试的超时时间：按配置的 per-attempt 优先，其次单次超时，再受总超时约束
				timeout := p.retryConfig.PerRequestTimeout
				if len(p.retryConfig.PerAttemptTimeouts) > attempt {
					timeout = p.retryConfig.PerAttemptTimeouts[attempt]
				}
				if !overallDeadline.IsZero() {
					remaining := time.Until(overallDeadline)
					if remaining <= 0 {
						http.Error(w, `{"error":{"type":"proxy_timeout","message":"request timeout after all retries"}}`, http.StatusServiceUnavailable)
						return
					}
					if remaining < timeout {
						timeout = remaining
					}
				}

				attemptCtx := context.WithValue(baseCtx, nodeContextKey{}, node)
				attemptCtx, cancel := context.WithTimeout(attemptCtx, timeout)
				reqForAttempt = reqForAttempt.WithContext(attemptCtx)
				proxy.ServeHTTP(wrapFirstByteFlush(mw, streamState), reqForAttempt)
				cancel()

				// 真正发送了请求，计数器+1
				attempt++

				upstreamStatus := extractUpstreamStatus(mw)
				statusForRetry := upstreamStatus
				if statusForRetry == 0 {
					statusForRetry = mw.status
				}

				// 判断是否是 context 错误（499=客户端关闭，504=网关超时）
				// context 错误不应该触发熔断器和节点失败标记
				isContextError := mw.status == 499 || mw.status == http.StatusGatewayTimeout

				failed := mw.status != http.StatusOK || statusForRetry >= http.StatusInternalServerError

				if attempt == 1 && failed {
					firstAttemptFailed = true
				}

				// context 错误不记录到熔断器
				if cb != nil && !isContextError {
					cb.RecordResult(!failed)
				}

				shouldRetry := failed && statusForRetry >= http.StatusInternalServerError && shouldRetryStatus(statusForRetry, p.retryConfig)
				// context 错误不应该重试（客户端已断开或超时）
				if isContextError {
					shouldRetry = false
				}
				isLastAttempt := attempt >= p.retryConfig.MaxAttempts
				finalAttempt := !failed || !shouldRetry || isLastAttempt

				var retryAttemptsTotal int64
				if finalAttempt {
					retryAttemptsTotal = int64(attempt)
				}
				var retrySuccess int64
				if finalAttempt && !failed && firstAttemptFailed {
					retrySuccess = 1
				}

				p.recordMetrics(r.Context(), node.ID, start, mw, usage, retryAttemptsTotal, retrySuccess, finalAttempt)

				if !failed {
					return
				}

				// context 错误不记录健康事件和节点失败
				if isContextError {
					p.logger.Printf("[context] request canceled/timeout for node %s, not marking as failure", node.Name)
					return
				}

				errMsg := extractErrorMessage(mw, statusForRetry)
				if account != nil {
					p.recordHealthEvent(account.ID, node.ID, HealthCheckMethodProxy, CheckSourceProxyFail, false, time.Since(start), errMsg, time.Now().UTC())
				}
				if p.shouldFail(node.ID, errMsg) {
					// 仅在最后一次尝试失败时才把节点标记为全局失败，避免单请求重试耗尽所有节点
					if isLastAttempt {
						p.handleFailure(node.ID, errMsg)
					} else {
						p.logger.Printf("[retry] node %s failed in attempt %d, will try other nodes", node.Name, attempt+1)
					}
				}
				skipNodes[node.ID] = true

				if !shouldRetry {
					return
				}

				// 如果还有可尝试的节点，记录日志并继续
				if shouldRetry {
					p.logger.Printf("retrying with next node (tried %d/%d nodes), %s failed: %s", attempt, len(account.Nodes), node.Name, errMsg)
					backoff := calculateBackoff(attempt-1, p.retryConfig)
					time.Sleep(backoff)
				}
			}

			// 检查响应是否已写入（避免重复调用 WriteHeader）
			if _, ok := w.(interface{ Header() http.Header }); ok {
				if w.Header().Get("Content-Type") == "" {
					// 响应头未写入，返回 503 表示服务暂时不可用（所有节点不可用）
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"error":{"type":"service_unavailable","message":"all nodes unavailable"}}`, http.StatusServiceUnavailable)
				}
			}
			return
		}

		// 其他请求透传到上游（不做任何处理）
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
		// 透传代理：不记录指标，不处理失败
		proxy := p.newPassthroughProxy(node)
		proxy.ServeHTTP(w, r)
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
		isAPIRequest := strings.HasPrefix(r.URL.Path, "/admin/api/") ||
			strings.HasPrefix(r.URL.Path, "/api/notification/") ||
			strings.HasPrefix(r.URL.Path, "/api/nodes/") ||
			strings.HasPrefix(r.URL.Path, "/api/accounts/") ||
			strings.HasPrefix(r.URL.Path, "/api/metrics/") ||
			strings.HasPrefix(r.URL.Path, "/api/monitor/") ||
			strings.HasPrefix(r.URL.Path, "/api/settings") ||
			strings.HasPrefix(r.URL.Path, "/api/claude-config/") ||
			strings.HasPrefix(r.URL.Path, "/api/pricing") ||
			strings.HasPrefix(r.URL.Path, "/api/usage/")

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

func extractUpstreamStatus(mw *metricsWriter) int {
	if mw == nil {
		return 0
	}
	if us := mw.Header().Get("X-Upstream-Status"); us != "" {
		if val, err := strconv.Atoi(us); err == nil {
			return val
		}
	}
	return 0
}

func extractErrorMessage(mw *metricsWriter, upstreamStatus int) string {
	if mw == nil {
		return "unknown error"
	}
	if msg := mw.Header().Get("X-Retry-Error"); msg != "" {
		return msg
	}
	if upstreamStatus >= http.StatusInternalServerError {
		return fmt.Sprintf("upstream status %d", upstreamStatus)
	}
	if mw.status != 0 {
		return fmt.Sprintf("status %d", mw.status)
	}
	return "unknown error"
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

	paths := []string{"CHANGELOG.md", "/app/CHANGELOG.md"}
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
