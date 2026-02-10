package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
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
	apiMux.HandleFunc("/api/pricing/sync", p.requireSession(p.handlePricingSync))
	apiMux.HandleFunc("/api/usage/logs", p.requireSession(p.handleUsageLogs))
	apiMux.HandleFunc("/api/usage/summary", p.requireSession(p.handleUsageSummary))
	apiMux.HandleFunc("/api/usage/cleanup", p.requireSession(p.handleUsageCleanup))
	// 环境变量 API
	apiMux.HandleFunc("/api/envvars", p.requireSession(p.handleEnvVars))
	apiMux.HandleFunc("/api/envvars/categories", p.requireSession(p.handleEnvVarsCategories))
	// 模型恢复 API
	apiMux.HandleFunc("/api/model-recovery", p.requireSession(p.handleModelRecovery))
	apiMux.HandleFunc("/api/model-recovery/dismiss", p.requireSession(p.handleModelRecoveryDismiss))

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

		if strings.HasPrefix(path, "/api/envvars") {
			apiMux.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(path, "/api/model-recovery") {
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

			// 提前从请求体提取模型 ID，用于节点选择时跳过该模型失败的节点
			var requestModelID string
			if len(bodyBytes) > 0 {
				var payload map[string]any
				if err := json.Unmarshal(bodyBytes, &payload); err == nil {
					if mid, ok := payload["model"].(string); ok {
						requestModelID = mid
					}
				}
			}

			// attempt 只计算真正发送请求的次数，maxLoops 防止无限循环
			// maxLoops = 节点数量 * 2，确保即使有熔断器也能尝试所有节点
			attempt := 0
			maxLoops := len(account.Nodes) * 2
			if maxLoops < 20 {
				maxLoops = 20 // 至少尝试 20 次循环
			}
			var requestAttempts []store.UsageLogAttempt
			requestStart := time.Now()
			retryBuf := newRetryBufferWriter(w)
			for loops := 0; loops < maxLoops; loops++ {
				reqForAttempt := r.Clone(baseCtx)
				if len(bodyBytes) > 0 {
					reqForAttempt.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					reqForAttempt.ContentLength = int64(len(bodyBytes))
				}
				node := p.selectHealthyNodeExcluding(account, skipNodes, requestModelID)
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

				// 追踪活跃连接数
				if p.nodeScorer != nil {
					p.nodeScorer.IncrActiveConn(node.ID)
					p.nodeScorer.IncrActiveModel(node.ID, requestModelID)
				}

				start := time.Now()
				mw := &metricsWriter{ResponseWriter: retryBuf, status: http.StatusOK}

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

				// 释放活跃连接计数
				if p.nodeScorer != nil {
					p.nodeScorer.DecrActiveConn(node.ID)
					p.nodeScorer.DecrActiveModel(node.ID, requestModelID)
				}

				// 真正发送了请求，计数器+1
				attempt++

				// 记录首字节延迟到评分器（用于慢节点检测）
				if p.nodeScorer != nil && mw.firstWrite {
					latencyMs := mw.firstAt.Sub(start).Milliseconds()
					p.nodeScorer.RecordLatency(node.ID, latencyMs)
					// 尝试恢复已降级的节点
					p.nodeScorer.TryRecoverDegraded(node.ID)
				}

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

				// 请求完成汇总日志：模型、节点、状态、耗时、token
				{
					elapsed := time.Since(start)
					model := usage.modelID
					if model == "" {
						model = "unknown"
					}
					status := "SUCCESS"
					if failed {
						status = fmt.Sprintf("FAILED(%d)", statusForRetry)
					}
					p.logger.Printf("[request] model=%s node=%s status=%s duration=%s tokens(in=%d,out=%d) account=%s",
						model, node.Name, status, elapsed.Round(time.Millisecond), usage.input, usage.output, account.ID)
				}

				if !failed {
					// 成功：记录 attempt
					requestAttempts = append(requestAttempts, store.UsageLogAttempt{
						Seq: attempt, NodeID: node.ID, NodeName: node.Name,
						StatusCode: statusForRetry, Success: true,
						DurationMs: time.Since(start).Milliseconds(),
						Action:     "success",
					})
					// 成功：记录 key 成功（多密钥轮换）
					if node.APIKeys != nil {
						node.APIKeys.RecordSuccess(node.GetActiveAPIKey())
					}
					// 模型级别恢复：请求成功说明该模型在此节点可用
					if p.modelRecovery != nil && usage.modelID != "" {
						if p.modelRecovery.IsModelFailed(node.ID, usage.modelID) {
							p.modelRecovery.MarkRecovered(node.ID, usage.modelID)
							p.logger.Printf("[model-recovery] model %s recovered on node %s via successful request", usage.modelID, node.Name)
							if p.wsHub != nil {
								p.wsHub.Broadcast(account.ID, "model_recovery", map[string]interface{}{
									"node_id":   node.ID,
									"node_name": node.Name,
									"model_id":  usage.modelID,
									"status":    "recovered",
									"timestamp": timeutil.FormatBeijingTime(time.Now()),
								})
							}
						}
					}
					// 写入带 attempts 的 usage log
					p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, true, requestAttempts)
					retryBuf.FlushToReal()
					return
				}

				// context 错误不记录健康事件和节点失败
				if isContextError {
					requestAttempts = append(requestAttempts, store.UsageLogAttempt{
						Seq: attempt, NodeID: node.ID, NodeName: node.Name,
						StatusCode: statusForRetry, Success: false,
						DurationMs: time.Since(start).Milliseconds(),
						ErrorMsg:   "context canceled/timeout", Severity: "context_error", Action: "abort",
					})
					p.logger.Printf("[context] request canceled/timeout for node %s, not marking as failure", node.Name)
					p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, false, requestAttempts)
					retryBuf.FlushToReal()
					return
				}

				// 语义化错误分析
				errMsg := extractErrorMessage(mw, statusForRetry)
				var respBody []byte
				if bodyPreview := mw.Header().Get("X-Retry-Error"); bodyPreview != "" {
					respBody = []byte(bodyPreview)
				}
				classified := ClassifyError(statusForRetry, respBody)
				p.logger.Printf("[error] node %s: severity=%s, code=%s, msg=%s, key_related=%v",
					node.Name, classified.Severity, classified.Code, classified.Message, classified.KeyRelated)

				// 确定本次尝试的 action（后续可能被覆盖为 retry）
				attemptAction := "fail"

				// 多密钥轮换：Key 相关错误时尝试切换 key
				if node.APIKeys != nil && classified.Severity.ShouldSwitchKey() {
					shouldSwitch := node.APIKeys.RecordFailure(node.GetActiveAPIKey(), classified)
					if shouldSwitch {
						newKey := node.APIKeys.RotateToNext()
						if newKey != "" {
							p.logger.Printf("[key-rotate] node %s: switched to next API key (remaining active: %d)",
								node.Name, node.APIKeys.ActiveKeyCount())
							requestAttempts = append(requestAttempts, store.UsageLogAttempt{
								Seq: attempt, NodeID: node.ID, NodeName: node.Name,
								StatusCode: statusForRetry, Success: false,
								DurationMs: time.Since(start).Milliseconds(),
								ErrorMsg:   errMsg, Severity: classified.Severity.String(), Action: "key_rotate",
							})
							// Key 切换后不跳过此节点，用新 key 重试
							retryBuf.Reset()
							continue
						}
					}
					// 所有 key 都失效
					if node.APIKeys.AllKeysDisabled() {
						p.logger.Printf("[key-rotate] node %s: all API keys disabled", node.Name)
						errMsg = "所有 API Key 已失效: " + classified.Message
					}
				}

				if account != nil {
					p.recordHealthEvent(account.ID, node.ID, HealthCheckMethodProxy, CheckSourceProxyFail, false, time.Since(start), errMsg, time.Now().UTC())
				}

				// 模型级别故障跟踪：记录失败的模型（非永久错误才跟踪，永久错误如 400 是请求本身的问题）
				if p.modelRecovery != nil && usage.modelID != "" && classified.Severity != SeverityPermanent {
					p.modelRecovery.MarkFailed(node.ID, usage.modelID, account.ID, errMsg)
					p.logger.Printf("[model-recovery] model %s marked failed on node %s: %s", usage.modelID, node.Name, errMsg)
					if p.wsHub != nil && account != nil {
						p.wsHub.Broadcast(account.ID, "model_recovery", map[string]interface{}{
							"node_id":   node.ID,
							"node_name": node.Name,
							"model_id":  usage.modelID,
							"status":    "failed",
							"error":     errMsg,
							"timestamp": timeutil.FormatBeijingTime(time.Now()),
						})
					}
				}

				// 根据错误严重程度决定是否标记节点失败
				switch classified.Severity {
				case SeverityPermanent:
					// 永久错误（如 400）：不标记节点失败，不重试
					p.logger.Printf("[permanent] node %s: %s, not marking as failed", node.Name, errMsg)
					attemptAction = "permanent_fail"
				case SeverityAccountIssue:
					// 账号问题：标记节点失败（需人工介入）
					p.handleFailure(node.ID, errMsg)
					skipNodes[node.ID] = true
				case SeverityKeyInvalid:
					// Key 失效且无可用 key：标记节点失败
					if node.APIKeys == nil || node.APIKeys.AllKeysDisabled() {
						p.handleFailure(node.ID, errMsg)
					}
					skipNodes[node.ID] = true
				case SeverityNodeDown:
					if p.shouldFail(node.ID, errMsg) {
						if isLastAttempt {
							p.handleFailure(node.ID, errMsg)
						} else {
							p.logger.Printf("[retry] node %s failed in attempt %d, will try other nodes", node.Name, attempt+1)
						}
					}
					skipNodes[node.ID] = true
				default:
					// SeverityTransient / SeverityDegraded：可重试
					skipNodes[node.ID] = true
				}

				if shouldRetry && classified.Severity != SeverityPermanent {
					attemptAction = "retry"
				}

				// 记录本次失败尝试
				requestAttempts = append(requestAttempts, store.UsageLogAttempt{
					Seq: attempt, NodeID: node.ID, NodeName: node.Name,
					StatusCode: statusForRetry, Success: false,
					DurationMs: time.Since(start).Milliseconds(),
					ErrorMsg:   errMsg, Severity: classified.Severity.String(), Action: attemptAction,
				})

				if !shouldRetry || classified.Severity == SeverityPermanent {
					p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, false, requestAttempts)
					retryBuf.FlushToReal()
					return
				}

				// 如果还有可尝试的节点，记录日志并继续
				if shouldRetry {
					p.logger.Printf("retrying with next node (tried %d/%d nodes), %s failed: %s", attempt, len(account.Nodes), node.Name, errMsg)
					retryBuf.Reset() // 丢弃失败节点的响应，对客户端无感
					backoff := calculateBackoff(attempt-1, p.retryConfig)
					time.Sleep(backoff)
				}
			}

			// 所有节点都不可用，返回 503
			if !retryBuf.IsFlushed() {
				retryBuf.FlushToReal()
				// 检查响应是否已写入（避免重复调用 WriteHeader）
				if w.Header().Get("Content-Type") == "" {
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
			strings.HasPrefix(r.URL.Path, "/api/usage/") ||
			strings.HasPrefix(r.URL.Path, "/api/envvars") ||
			strings.HasPrefix(r.URL.Path, "/api/model-recovery")

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

// handleEnvVars 返回所有环境变量配置（仅管理员可访问）
func (p *Server) handleEnvVars(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查管理员权限
	isAdmin, _ := r.Context().Value(isAdminContextKey{}).(bool)
	if !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	category := r.URL.Query().Get("category")
	var envvars []EnvVarDefinition

	if category != "" {
		envvars = GetEnvVarsByCategory(EnvVarCategory(category))
	} else {
		envvars = GetAllEnvVarDefinitions()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": envvars,
	})
}

// handleEnvVarsCategories 返回所有环境变量分类（仅管理员可访问）
func (p *Server) handleEnvVarsCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查管理员权限
	isAdmin, _ := r.Context().Value(isAdminContextKey{}).(bool)
	if !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	categories := GetEnvVarCategories()
	writeJSON(w, http.StatusOK, map[string]any{
		"data": categories,
	})
}

// writeUsageLogWithAttempts 写入带有链路追踪 attempts 的使用日志
func (p *Server) writeUsageLogWithAttempts(ctx context.Context, account *Account, node *Node, u *usage, attemptStart, requestStart time.Time, success bool, attempts []store.UsageLogAttempt) {
	if p.store == nil || u == nil || u.modelID == "" {
		return
	}
	if account == nil || node == nil {
		return
	}

	costUSD, err := p.store.CalculateCost(ctx, u.modelID, u.input, u.output)
	if err != nil && (u.input > 0 || u.output > 0) {
		p.logger.Printf("[usage] failed to calculate cost for model %s: %v", u.modelID, err)
	}

	usageLog := store.UsageLogRecord{
		AccountID:     account.ID,
		NodeID:        node.ID,
		NodeName:      node.Name,
		ModelID:       u.modelID,
		InputTokens:   u.input,
		OutputTokens:  u.output,
		CostUSD:       costUSD,
		RequestID:     u.requestID,
		Success:       success,
		DurationMs:    time.Since(requestStart).Milliseconds(),
		TotalAttempts: len(attempts),
		Attempts:      attempts,
	}

	// 从 attempts 链中提取最后一条错误信息，写入主日志便于直接展示
	if !success {
		for i := len(attempts) - 1; i >= 0; i-- {
			if attempts[i].ErrorMsg != "" {
				usageLog.ErrorMsg = attempts[i].ErrorMsg
				break
			}
		}
	}

	if err := p.store.InsertUsageLog(ctx, usageLog); err != nil {
		p.logger.Printf("[usage] failed to insert usage log for account %s, model %s: %v", account.ID, u.modelID, err)
	}
}
