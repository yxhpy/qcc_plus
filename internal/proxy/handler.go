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
	"regexp"
	"strconv"
	"strings"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
	"qcc_plus/internal/version"
	"qcc_plus/web"
)

var strictRouteModelPattern = regexp.MustCompile(`^([^\s/]+)/([^\s]+)$`)

type strictRouteMode struct {
	enabled        bool
	targetNodeName string
	upstreamModel  string
}

func parseStrictRouteModel(model string) strictRouteMode {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return strictRouteMode{}
	}
	matches := strictRouteModelPattern.FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return strictRouteMode{}
	}
	nodeName := strings.TrimSpace(matches[1])
	upstream := strings.TrimSpace(matches[2])
	if nodeName == "" || upstream == "" {
		return strictRouteMode{}
	}
	return strictRouteMode{
		enabled:        true,
		targetNodeName: nodeName,
		upstreamModel:  upstream,
	}
}

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
	apiMux.HandleFunc("/api/session", p.handleSession)
	apiMux.HandleFunc("/admin/api/accounts", p.requireSession(p.handleAccounts))
	apiMux.HandleFunc("/admin/api/nodes", p.requireSession(p.handleNodes))
	apiMux.HandleFunc("/admin/api/nodes/copy", p.requireSession(p.handleCopyNode))
	apiMux.HandleFunc("/admin/api/config", p.requireSession(p.handleConfig))
	apiMux.HandleFunc("/admin/api/nodes/activate", p.requireSession(p.handleActivate))
	apiMux.HandleFunc("/admin/api/nodes/disable", p.requireSession(p.handleDisable))
	apiMux.HandleFunc("/admin/api/nodes/enable", p.requireSession(p.handleEnable))
	apiMux.HandleFunc("/admin/api/cc-switch/import", p.requireSession(p.handleCCSwitchImport))
	apiMux.HandleFunc("/admin/api/cc-switch/export", p.requireSession(p.handleCCSwitchExport))
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
	apiMux.HandleFunc("/api/settings/runtime-definitions", p.requireSession(settingsHandler.GetRuntimeDefinitions))
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
	apiMux.HandleFunc("/api/model-recovery/non-recoverable", p.requireSession(p.handleModelRecoverySetNonRecoverable))
	apiMux.HandleFunc("/api/error-policies", p.requireSession(p.handleErrorPolicies))
	apiMux.HandleFunc("/api/error-policies/toggle", p.requireSession(p.handleErrorPolicyToggle))
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

		if path == "/api/session" {
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

		if strings.HasPrefix(path, "/api/error-policies") {
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

		if normalizedOpenAIPath, ok := normalizeOpenAIIngressPath(path); ok && normalizedOpenAIPath == openAIModelsPath {
			p.handleOpenAIModels(w, r)
			return
		}

		ingressProtocol := detectIngressProtocol(path)
		if path == "/v1/messages" || ingressProtocol != SourceProtocolClaude {
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
			var requestIsStreaming bool
			if len(bodyBytes) > 0 {
				var payload map[string]any
				if err := json.Unmarshal(bodyBytes, &payload); err == nil {
					if mid, ok := payload["model"].(string); ok {
						requestModelID = mid
					}
					if streamFlagEnabled(payload["stream"]) {
						requestIsStreaming = true
					}
				}
			}
			if requestModelID == "" && ingressProtocol == SourceProtocolGemini {
				requestModelID = extractModelFromGeminiPath(path)
			}
			strictMode := parseStrictRouteModel(requestModelID)
			if strictMode.enabled {
				requestModelID = strictMode.upstreamModel
				if len(bodyBytes) > 0 {
					var strictPayload map[string]any
					if err := json.Unmarshal(bodyBytes, &strictPayload); err == nil {
						strictPayload["model"] = strictMode.upstreamModel
						if rewritten, err := json.Marshal(strictPayload); err == nil {
							bodyBytes = rewritten
						}
					}
				}
			}

			attempt := 0
			maxLoops := len(account.Nodes)
			if maxLoops < 1 {
				maxLoops = 1
			}
			if strictMode.enabled {
				maxLoops = 1
			}
			var requestAttempts []store.UsageLogAttempt
			requestStart := time.Now()
			retryBuf := newRetryBufferWriter(w)
			// 检测流式请求：从 header/query 或 body 中的 stream 字段判断
			if requestIsStreaming || isStreamRequest(r) {
				requestIsStreaming = true
				// 流式模式：让 retryBuf 在收到成功响应数据后立即透传给客户端，
				// 避免 SSE 数据被无限缓冲导致客户端"卡住"。
				retryBuf.SetStreaming(true)
			}
			requiredProtocol := ""
			detectedProtocol := ingressProtocol
			if path == "/v1/messages" {
				requiredProtocol = SourceProtocolClaude
			} else if detectedProtocol != SourceProtocolClaude {
				requiredProtocol = detectedProtocol
			}
			if strictMode.enabled {
				requiredProtocol = ""
			}
			for loops := 0; loops < maxLoops; loops++ {
				reqForAttempt := r.Clone(baseCtx)
				node := p.selectHealthyNodeExcluding(account, skipNodes, requestModelID, strictMode.targetNodeName, requiredProtocol, !strictMode.enabled)
				if node == nil {
					break
				}
				targetProtocol := requiredProtocol
				if targetProtocol == "" {
					targetProtocol = NormalizedSourceProtocol(node.SourceProtocol)
				}

				attemptBody := bodyBytes
				if strictMode.enabled {
					if targetProtocol == SourceProtocolGemini {
						targetModel := strictMode.upstreamModel
						if targetModel == "" {
							targetModel = requestModelID
						}
						reqForAttempt.URL.Path = geminiModelsPrefix + targetModel + geminiGenerateSuffix
					} else if targetProtocol == SourceProtocolOpenAI {
						reqForAttempt.URL.Path = rewriteIngressPathForUpstream(path, targetProtocol, requestModelID)
					} else {
						reqForAttempt.URL.Path = r.URL.Path
					}
				} else {
					if targetProtocol == SourceProtocolGemini || targetProtocol == SourceProtocolOpenAI {
						reqForAttempt.URL.Path = rewriteIngressPathForUpstream(path, targetProtocol, requestModelID)
					} else {
						reqForAttempt.URL.Path = r.URL.Path
					}
				}
				if len(attemptBody) > 0 {
					reqForAttempt.Body = io.NopCloser(bytes.NewReader(attemptBody))
					reqForAttempt.ContentLength = int64(len(attemptBody))
				}

				// 检查熔断器
				var cb *CircuitBreaker
				if p.cbConfig.Enabled {
					cb = p.getOrCreateCircuitBreaker(node.ID)
					allowed := cb.AllowRequest()
					if !strictMode.enabled && !allowed {
						p.logger.Printf("node %s circuit breaker is open, skipping (loop %d/%d, tried %d nodes)", node.Name, loops+1, maxLoops, attempt)
						skipNodes[node.ID] = true
						continue // 跳过此节点，不计入 attempt
					}
					if strictMode.enabled && !allowed {
						p.logger.Printf("strict route bypasses open circuit breaker for node %s (loop %d/%d)", node.Name, loops+1, maxLoops)
					}
				}

				usage := &usage{modelID: requestModelID}
				// 判断请求是否为流式（SSE），流式请求使用空闲超时而非总超时
				isStreaming := requestIsStreaming || isStreamRequest(reqForAttempt)
				var idleCfg *streamIdleConfig
				if isStreaming && p.retryConfig.StreamIdleTimeout > 0 {
					idleCfg = &streamIdleConfig{idleTimeout: p.retryConfig.StreamIdleTimeout}
				}
				proxy, streamState := p.newReverseProxy(node, usage, idleCfg, strictMode.enabled)
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
				var cancel context.CancelFunc
				if isStreaming && idleCfg != nil {
					// 流式请求：不设总超时，由 idleTimeoutReader 在无数据时取消
					attemptCtx, cancel = context.WithCancel(attemptCtx)
					idleCfg.cancel = cancel
				} else {
					// 非流式请求：保持原有的总超时
					attemptCtx, cancel = context.WithTimeout(attemptCtx, timeout)
				}
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

				// 判断 context 错误类型：
				// - 499（客户端主动关闭）：不标记失败、不重试
				// - 504（代理超时，上游无响应）：标记失败、允许切换节点重试
				isClientClosed := mw.status == 499
				isProxyTimeout := mw.status == http.StatusGatewayTimeout

				failed := mw.status != http.StatusOK || statusForRetry >= http.StatusInternalServerError

				if attempt == 1 && failed {
					firstAttemptFailed = true
				}

				// 客户端关闭不记录到熔断器，但代理超时应该记录
				if cb != nil && !isClientClosed {
					cb.RecordResult(!failed)
				}

				shouldRetry := !strictMode.enabled && failed && statusForRetry >= http.StatusInternalServerError && shouldRetryStatus(statusForRetry, p.retryConfig)
				isLastAttempt := strictMode.enabled || attempt >= len(account.Nodes)
				// 客户端主动关闭不应该重试
				if isClientClosed {
					shouldRetry = false
				}
				// 代理超时（504）允许切换节点重试
				if !strictMode.enabled && isProxyTimeout && !isLastAttempt {
					shouldRetry = true
				}
				finalAttempt := !failed || !shouldRetry || isLastAttempt

				var retryAttemptsTotal int64
				if finalAttempt {
					retryAttemptsTotal = int64(attempt)
				}
				var retrySuccess int64
				if finalAttempt && !failed && firstAttemptFailed {
					retrySuccess = 1
				}

				p.recordMetrics(r.Context(), node.ID, start, mw, usage, retryAttemptsTotal, retrySuccess, finalAttempt, requiredProtocol)

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
					if !strictMode.enabled && node.APIKeys != nil {
						node.APIKeys.RecordSuccess(node.GetActiveAPIKey())
					}
					// 模型级别恢复：请求成功说明该模型在此节点可用
					if !strictMode.enabled && p.modelRecovery != nil && usage.modelID != "" {
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
							if p.notifyMgr != nil && account != nil {
								p.notifyMgr.Publish(notify.Event{
									AccountID:  account.ID,
									EventType:  notify.EventModelRecovered,
									Title:      "模型已恢复",
									Content:    fmt.Sprintf("**节点名称**: %s\n**模型**: %s\n**恢复方式**: 请求成功\n**时间**: %s", node.Name, usage.modelID, timeutil.FormatBeijingTime(time.Now())),
									DedupKey:   node.ID + ":" + usage.modelID,
									OccurredAt: time.Now(),
								})
							}
						}
					}
					// 写入带 attempts 的 usage log

					p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, true, requestAttempts)
					retryBuf.FlushToReal()
					return
				}

				// 客户端主动关闭：不记录健康事件，直接中止
				if isClientClosed {
					requestAttempts = append(requestAttempts, store.UsageLogAttempt{
						Seq: attempt, NodeID: node.ID, NodeName: node.Name,
						StatusCode: statusForRetry, Success: false,
						DurationMs: time.Since(start).Milliseconds(),
						ErrorMsg:   "client closed connection", Severity: "context_error", Action: "abort",
					})
					p.logger.Printf("[context] client closed connection for node %s, not marking as failure", node.Name)
					p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, false, requestAttempts)
					retryBuf.FlushToReal()
					return
				}

				// 语义化错误分析
				respBody := mw.ErrorBodyPreview()
				classified := ClassifyError(statusForRetry, respBody)
				if p.errorPolicy != nil {
					p.errorPolicy.Record(statusForRetry, classified.Code, classified.Message)
					classified = p.errorPolicy.Apply(statusForRetry, classified)
				}
				errMsg := formatUpstreamErrorDetail(statusForRetry, classified, respBody, extractErrorMessage(mw, statusForRetry))
				// 高可用策略：除客户端主动断开外，任何失败在未到最后尝试前都允许切换下一节点
				if !strictMode.enabled && failed && !isClientClosed && !isLastAttempt {
					shouldRetry = true
				}
				p.logger.Printf("[error] node %s: severity=%s, code=%s, msg=%s, key_related=%v",
					node.Name, classified.Severity, classified.Code, classified.Message, classified.KeyRelated)

				// 确定本次尝试的 action（后续可能被覆盖为 retry）
				attemptAction := "fail"

				// 多密钥轮换：Key 相关错误时尝试切换 key
				if !strictMode.enabled && node.APIKeys != nil && classified.Severity.ShouldSwitchKey() {
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
						errMsg = "所有 API Key 已失效\n" + errMsg
					}
				}

				if account != nil {
					p.recordHealthEvent(account.ID, node.ID, HealthCheckMethodProxy, CheckSourceProxyFail, false, time.Since(start), errMsg, time.Now().UTC())
				}

				// 模型级别故障跟踪：所有失败都进入恢复列表（是否继续检查由“不可恢复”开关控制）
				if !strictMode.enabled && p.modelRecovery != nil && usage.modelID != "" {
					isNew := p.modelRecovery.MarkFailed(node.ID, usage.modelID, account.ID, errMsg)
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
					// 仅首次进入恢复列表时发送通知（避免重复通知）
					if isNew && p.notifyMgr != nil && account != nil {
						p.notifyMgr.Publish(notify.Event{
							AccountID:  account.ID,
							EventType:  notify.EventModelFailed,
							Title:      "模型进入恢复列表",
							Content:    fmt.Sprintf("**节点名称**: %s\n**模型**: %s\n**错误信息**: %s\n**时间**: %s", node.Name, usage.modelID, errMsg, timeutil.FormatBeijingTime(time.Now())),
							DedupKey:   node.ID + ":" + usage.modelID,
							OccurredAt: time.Now(),
						})
					}
				}

				// 根据错误严重程度决定是否标记节点失败
				if !strictMode.enabled {
					switch classified.Severity {
					case SeverityPermanent:
						// 永久错误：高可用优先，当前节点本轮跳过，继续尝试下一个节点
						skipNodes[node.ID] = true
						attemptAction = "permanent_fail"
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
				}

				if shouldRetry && !isLastAttempt {
					attemptAction = "retry"
				}

				// 记录本次失败尝试
				requestAttempts = append(requestAttempts, store.UsageLogAttempt{
					Seq: attempt, NodeID: node.ID, NodeName: node.Name,
					StatusCode: statusForRetry, Success: false,
					DurationMs: time.Since(start).Milliseconds(),
					ErrorMsg:   errMsg, Severity: classified.Severity.String(), Action: attemptAction,
				})

				if !shouldRetry {
					p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, false, requestAttempts)
					retryBuf.FlushToReal()
					return
				}

				// 如果还有可尝试的节点，记录日志并继续
				if shouldRetry && !strictMode.enabled {
					// 安全检查：如果响应已经开始发送给客户端（retryBuf 已 flush），
					// 则无法重试，直接返回（避免发送重复/损坏的数据）
					if retryBuf.IsFlushed() {
						p.logger.Printf("[retry] cannot retry: response already flushed to client for node %s", node.Name)
						p.writeUsageLogWithAttempts(r.Context(), account, node, usage, start, requestStart, false, requestAttempts)
						return
					}
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

		if path == "/v1/models" {
			p.handleOpenAIModels(w, r)
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

type openAIModelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func (p *Server) handleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxyKey := extractAPIKey(r)
	account := p.getAccountByProxyKey(proxyKey)
	if account == nil {
		account = p.defaultAccount
	}
	if account == nil {
		http.Error(w, "account not found", http.StatusUnauthorized)
		return
	}

	seen := make(map[string]struct{})
	models := make([]openAIModelItem, 0)
	createdAt := time.Now().Unix()

	for _, n := range account.Nodes {
		if n == nil {
			continue
		}
		for source, target := range n.ModelMapping {
			if strings.TrimSpace(source) != "" {
				if _, ok := seen[source]; !ok {
					seen[source] = struct{}{}
					models = append(models, openAIModelItem{ID: source, Object: "model", Created: createdAt, OwnedBy: "qcc_plus"})
				}
			}
			if strings.TrimSpace(target) != "" {
				if _, ok := seen[target]; !ok {
					seen[target] = struct{}{}
					models = append(models, openAIModelItem{ID: target, Object: "model", Created: createdAt, OwnedBy: "qcc_plus"})
				}
			}
		}
		hcModel := strings.TrimSpace(n.HealthCheckModel)
		if hcModel != "" {
			if _, ok := seen[hcModel]; !ok {
				seen[hcModel] = struct{}{}
				models = append(models, openAIModelItem{ID: hcModel, Object: "model", Created: createdAt, OwnedBy: "qcc_plus"})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   models,
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
			strings.HasPrefix(r.URL.Path, "/api/model-recovery") ||
			strings.HasPrefix(r.URL.Path, "/api/error-policies")

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
	if key := strings.TrimSpace(r.Header.Get("x-api-key")); key != "" {
		return key
	}
	if key := strings.TrimSpace(r.Header.Get("x-goog-api-key")); key != "" {
		return key
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.Fields(auth)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	if key := strings.TrimSpace(r.URL.Query().Get("key")); key != "" {
		return key
	}
	if key := strings.TrimSpace(r.URL.Query().Get("api_key")); key != "" {
		return key
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
