package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"qcc_plus/internal/notify"
)

type retryTransport struct {
	base      http.RoundTripper
	attempts  int
	logger    *log.Logger
	notifyMgr *notify.Manager
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		return nil, errors.New("nil base transport")
	}
	attempts := t.attempts
	if attempts < 1 {
		attempts = 1
	}
	var bodyCopy []byte
	if req.Body != nil {
		bodyCopy, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyCopy))
	}

	var lastResp *http.Response
	var lastErr error
	var lastRespBody []byte
	for i := 0; i < attempts; i++ {
		// 在重试前检查 context 是否已取消，避免无效请求
		if err := req.Context().Err(); err != nil {
			return nil, err // 直接返回 context 错误，不继续重试
		}

		cloned := req.Clone(req.Context())
		if bodyCopy != nil {
			cloned.Body = io.NopCloser(bytes.NewReader(bodyCopy))
		}
		resp, err := t.base.RoundTrip(cloned)
		if err != nil {
			// 如果是 context 取消/超时，立即返回，不继续重试
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			lastErr = err
		} else {
			if resp.StatusCode == http.StatusOK {
				return resp, nil
			}
			// 非 200 视为失败，保存响应以便最终返回
			lastResp = resp
			bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if readErr == nil && len(bodyBytes) > 0 {
				lastRespBody = bodyBytes
				bodyPreview := string(bodyBytes)
				if len(bodyBytes) >= 4096 {
					bodyPreview = bodyPreview + "...(truncated)"
				}
				lastErr = fmt.Errorf("upstream status %d: %s", resp.StatusCode, bodyPreview)
			} else if readErr != nil {
				lastErr = fmt.Errorf("upstream status %d (read error: %v)", resp.StatusCode, readErr)
			} else {
				lastErr = fmt.Errorf("upstream status %d", resp.StatusCode)
			}
		}
		if i < attempts-1 {
			t.logger.Printf("retry %d/%d for %s: %v", i+1, attempts, req.URL.String(), lastErr)
			time.Sleep(time.Duration(150*(i+1)) * time.Millisecond)
		}
	}

	if t.notifyMgr != nil && lastErr != nil {
		if acc := accountFromCtx(req); acc != nil {
			nodeName := ""
			if n := nodeFromCtx(req); n != nil {
				nodeName = n.Name
			}
			errText := lastErr.Error()
			t.notifyMgr.Publish(notify.Event{
				AccountID:  acc.ID,
				EventType:  notify.EventRequestFailed,
				Title:      "请求失败告警",
				Content:    fmt.Sprintf("**请求**: %s %s\n**节点**: %s\n**重试次数**: %d\n**错误信息**: %s", req.Method, req.URL.String(), chooseNonEmpty(nodeName, "-"), attempts, errText),
				OccurredAt: time.Now(),
			})
		}
	}
	if lastResp != nil {
		// 透传上游原始状态码，避免 502 触发客户端重试
		// 保留原始响应体，让客户端看到真实的错误信息
		var bodyBytes []byte
		if len(lastRespBody) > 0 {
			bodyBytes = lastRespBody
		} else {
			bodyBytes = []byte(fmt.Sprintf(`{"error":{"type":"upstream_error","message":"%s"}}`, lastErr.Error()))
		}
		return &http.Response{
			StatusCode: lastResp.StatusCode,
			Body:       io.NopCloser(bytes.NewBuffer(bodyBytes)),
			Header: http.Header{
				"Content-Type":  []string{"application/json"},
				"X-Proxy-Error": []string{fmt.Sprintf("retries exhausted after %d attempts", attempts)},
			},
			Request: req,
		}, nil
	}
	return nil, lastErr
}

// usageReader 在转发时截取部分响应体，用于提取 usage。
type usageReader struct {
	io.ReadCloser
	buf     *bytes.Buffer
	tracker *usage
}

const usageBufLimit = 256 * 1024 // 256KB 足够找到 usage 字段

func (u *usageReader) Read(p []byte) (int, error) {
	n, err := u.ReadCloser.Read(p)
	if n > 0 && u.buf != nil {
		if u.buf.Len() < usageBufLimit {
			// 仅保存前 256KB，避免占用过多内存。
			remain := usageBufLimit - u.buf.Len()
			slice := p[:n]
			if len(slice) > remain {
				slice = slice[:remain]
			}
			u.buf.Write(slice)
		}
	}
	return n, err
}

func (u *usageReader) Close() error {
	err := u.ReadCloser.Close()
	if u.tracker != nil && u.buf != nil {
		if in, out := parseUsage(u.buf.Bytes()); in > 0 || out > 0 {
			u.tracker.input = in
			u.tracker.output = out
		}
	}
	return err
}

const streamFlushInterval = 50 * time.Millisecond

type streamState struct {
	flag atomic.Bool
}

func (s *streamState) set(enabled bool) {
	if s == nil {
		return
	}
	s.flag.Store(enabled)
}

func (s *streamState) enabled() bool {
	if s == nil {
		return false
	}
	return s.flag.Load()
}

func boolLike(val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func streamFlagEnabled(v any) bool {
	switch tv := v.(type) {
	case bool:
		return tv
	case string:
		return boolLike(tv)
	default:
		return false
	}
}

func isStreamRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	if boolLike(req.URL.Query().Get("stream")) {
		return true
	}
	if boolLike(req.Header.Get("stream")) || boolLike(req.Header.Get("x-stream")) {
		return true
	}
	accept := strings.ToLower(req.Header.Get("Accept"))
	return strings.Contains(accept, "text/event-stream")
}

type firstByteFlusher struct {
	http.ResponseWriter
	flusher http.Flusher
	state   *streamState
	flushed bool
}

func (f *firstByteFlusher) Write(p []byte) (int, error) {
	n, err := f.ResponseWriter.Write(p)
	if err == nil && f.state != nil && f.state.enabled() && !f.flushed && f.flusher != nil {
		f.flusher.Flush()
		f.flushed = true
	}
	return n, err
}

func (f *firstByteFlusher) Flush() {
	if f.flusher != nil {
		f.flusher.Flush()
	}
}

func wrapFirstByteFlush(w http.ResponseWriter, state *streamState) http.ResponseWriter {
	if state == nil {
		return w
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return w
	}
	return &firstByteFlusher{ResponseWriter: w, flusher: flusher, state: state}
}

// 构建指向指定节点的反向代理。
func (p *Server) newReverseProxy(node *Node, u *usage) (*httputil.ReverseProxy, *streamState) {
	proxy := httputil.NewSingleHostReverseProxy(node.URL)
	proxy.Transport = p.transport
	proxy.FlushInterval = -1
	streamingState := &streamState{}

	// 清理工具定义，去除 Anthropic 未支持的字段。
	cleanTools := func(body []byte) ([]byte, bool) {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, false
		}
		rawTools, ok := payload["tools"].([]any)
		if !ok || len(rawTools) == 0 {
			return nil, false
		}

		changed := false
		sanitized := make([]any, 0, len(rawTools))
		for _, item := range rawTools {
			obj, ok := item.(map[string]any)
			if !ok {
				sanitized = append(sanitized, item)
				continue
			}
			cleaned := make(map[string]any, 3)
			if v, ok := obj["name"]; ok {
				cleaned["name"] = v
			}
			if v, ok := obj["description"]; ok {
				cleaned["description"] = v
			}
			if v, ok := obj["input_schema"]; ok {
				cleaned["input_schema"] = v
			}
			if len(cleaned) != len(obj) {
				changed = true
			}
			sanitized = append(sanitized, cleaned)
		}
		if !changed {
			return nil, false
		}
		payload["tools"] = sanitized
		buf, err := json.Marshal(payload)
		if err != nil {
			return nil, false
		}
		return buf, true
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		// 将 handler 中的 usage 指针放入响应上下文，便于 body 包装更新。
		if u != nil {
			resp.Request = resp.Request.WithContext(context.WithValue(resp.Request.Context(), usageContextKey{}, u))
		}

		inputTokens := headerInt(resp.Header.Get("x-usage-input-tokens"))
		outputTokens := headerInt(resp.Header.Get("x-usage-output-tokens"))
		if u != nil {
			u.input = inputTokens
			u.output = outputTokens
		}
		if inputTokens > 0 {
			resp.Header.Set("X-Usage-Input-Tokens", fmt.Sprintf("%d", inputTokens))
		}
		if outputTokens > 0 {
			resp.Header.Set("X-Usage-Output-Tokens", fmt.Sprintf("%d", outputTokens))
		}
		resp.Header.Set("X-Proxy-Node", node.Name)

		// 包装 body，捕获 SSE/JSON 中的 usage。
		resp.Body = &usageReader{ReadCloser: resp.Body, tracker: u, buf: &bytes.Buffer{}}
		return nil
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		streaming := isStreamRequest(req)
		originalDirector(req)
		req.Host = node.URL.Host
		if node.APIKey != "" {
			req.Header.Set("x-api-key", node.APIKey)
			req.Header.Set("Authorization", "Bearer "+node.APIKey)
		}

		// 仅处理 JSON 体的写请求，剔除 tools 中的非标准字段（如 custom）。
		if req.Body != nil && (req.Method == http.MethodPost || req.Method == http.MethodPut) {
			ct := strings.ToLower(req.Header.Get("Content-Type"))
			if strings.Contains(ct, "application/json") {
				bodyBytes, err := io.ReadAll(req.Body)
				if err != nil {
					p.logger.Printf("proxy: read request body failed: %v", err)
					req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					return
				}
				_ = req.Body.Close()

				var payload map[string]any
				if err := json.Unmarshal(bodyBytes, &payload); err == nil {
					if streamFlagEnabled(payload["stream"]) {
						streaming = true
					}
				}

				if cleaned, ok := cleanTools(bodyBytes); ok {
					bodyBytes = cleaned
				}

				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
				req.Header.Set("Content-Length", strconv.FormatInt(int64(len(bodyBytes)), 10))
				req.Header.Del("Transfer-Encoding")
			}
		}

		if streaming {
			streamingState.set(true)
			proxy.FlushInterval = streamFlushInterval
		} else {
			streamingState.set(false)
			proxy.FlushInterval = -1
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// 区分 context 取消/超时 和真正的上游错误
		isContextError := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)

		// context 取消通常是客户端断开或请求超时，不需要告警
		if !isContextError && p.notifyMgr != nil {
			if acc := accountFromCtx(r); acc != nil {
				nodeName := ""
				if n := nodeFromCtx(r); n != nil {
					nodeName = n.Name
				}
				p.notifyMgr.Publish(notify.Event{
					AccountID:  acc.ID,
					EventType:  notify.EventRequestProxyError,
					Title:      "代理错误告警",
					Content:    fmt.Sprintf("**请求**: %s %s\n**节点**: %s\n**错误信息**: %v", r.Method, r.URL.String(), chooseNonEmpty(nodeName, "-"), err),
					OccurredAt: time.Now(),
				})
			}
		}

		p.logger.Printf("proxy error %s %s: %v", r.Method, r.URL.String(), err)

		// context 取消返回 499 (Client Closed Request) 或 504 (Gateway Timeout)
		// 而不是 502，避免误判为上游错误
		if isContextError {
			if errors.Is(err, context.DeadlineExceeded) {
				http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
			} else {
				// 499 是非标准状态码，用 499 表示客户端关闭连接
				w.WriteHeader(499)
				w.Write([]byte("client closed request"))
			}
			return
		}
		// 返回 503 而非 502，避免触发客户端重试
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, fmt.Sprintf(`{"error":{"type":"upstream_connection_error","message":"%s"}}`, err.Error()), http.StatusServiceUnavailable)
	}

	return proxy, streamingState
}

// newPassthroughProxy 创建一个简单的透传代理，不做任何额外处理（不记录指标、不处理工具定义）。
func (p *Server) newPassthroughProxy(node *Node) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(node.URL)
	proxy.Transport = p.transport
	proxy.FlushInterval = -1

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = node.URL.Host
		if node.APIKey != "" {
			req.Header.Set("x-api-key", node.APIKey)
			req.Header.Set("Authorization", "Bearer "+node.APIKey)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.logger.Printf("passthrough proxy error %s %s: %v", r.Method, r.URL.String(), err)
		// 返回 503 而非 502，避免触发客户端重试
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, fmt.Sprintf(`{"error":{"type":"upstream_connection_error","message":"%s"}}`, err.Error()), http.StatusServiceUnavailable)
	}

	return proxy
}
