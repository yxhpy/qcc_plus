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
	"time"
)

type retryTransport struct {
	base     http.RoundTripper
	attempts int
	logger   *log.Logger
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
		cloned := req.Clone(req.Context())
		if bodyCopy != nil {
			cloned.Body = io.NopCloser(bytes.NewReader(bodyCopy))
		}
		resp, err := t.base.RoundTrip(cloned)
		if err != nil {
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
	if lastResp != nil {
		// 返回 502，包含最后一次状态信息。
		msg := fmt.Sprintf("proxy retries exhausted: %v", lastErr)
		errPayload := map[string]any{
			"error": map[string]any{
				"type":            "proxy_error",
				"message":         lastErr.Error(),
				"upstream_status": lastResp.StatusCode,
				"retries":         attempts,
			},
		}
		if len(lastRespBody) > 0 {
			errPayload["error"].(map[string]any)["upstream_body"] = string(lastRespBody)
		}
		bodyBytes, marshalErr := json.Marshal(errPayload)
		if marshalErr != nil {
			bodyBytes = []byte(msg)
		}
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(bytes.NewBuffer(bodyBytes)),
			Header: http.Header{
				"Content-Type":      []string{"application/json"},
				"X-Retry-Error":     []string{msg},
				"X-Upstream-Status": []string{fmt.Sprintf("%d", lastResp.StatusCode)},
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

// 构建指向指定节点的反向代理。
func (p *Server) newReverseProxy(node *Node, u *usage) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(node.URL)
	proxy.Transport = p.transport
	proxy.FlushInterval = -1

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

				if cleaned, ok := cleanTools(bodyBytes); ok {
					bodyBytes = cleaned
				}

				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
				req.Header.Set("Content-Length", strconv.FormatInt(int64(len(bodyBytes)), 10))
				req.Header.Del("Transfer-Encoding")
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.logger.Printf("proxy error %s %s: %v", r.Method, r.URL.String(), err)
		http.Error(w, "upstream error", http.StatusBadGateway)
	}

	return proxy
}
