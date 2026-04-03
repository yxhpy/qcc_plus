package proxy

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// 协议集成测试：基于 cc-switch 的真实 API 格式
// cc-switch 使用 ProviderAdapter trait 模式处理三种协议
// qcc_plus 使用 SourceProtocol + 透传模式

// TestOpenAIProtocol_E2E 验证 OpenAI/Codex 协议完整流程：
// cc-switch CodexAdapter 使用 Bearer 认证，路径 /v1/chat/completions
func TestOpenAIProtocol_E2E(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
		}
		// cc-switch Codex 格式的响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	}))
	defer up.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(up.URL).
		WithRetry(1).
		WithListenAddr(listener.Addr().String()))

	acc, _ := srv.createAccount("test", "client-key", "pw", false)
	srv.addNodeWithMethod(acc, "openai-node", up.URL, "sk-test-key", 1, HealthCheckMethodHEAD, "", nil, SourceProtocolOpenAI, "", "", "", 0)

	go http.Serve(listener, srv.Handler())

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"max_tokens":16}`
	req, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/openai/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(b))
	}

	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("auth = %q, want Bearer token", gotAuth)
	}
	if gotBody["model"] != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", gotBody["model"])
	}
}

// TestGeminiProtocol_E2E 验证 Gemini 协议完整流程：
// cc-switch GeminiAdapter 使用 x-goog-api-key 认证
func TestGeminiProtocol_E2E(t *testing.T) {
	var gotPath string
	var gotGoogKey string

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotGoogKey = r.Header.Get("x-goog-api-key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":8,"totalTokenCount":13}}`))
	}))
	defer up.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(up.URL).
		WithRetry(1).
		WithListenAddr(listener.Addr().String()))

	acc, _ := srv.createAccount("test", "client-key", "pw", false)
	srv.addNodeWithMethod(acc, "gemini-node", up.URL, "AIza-test-key", 1, HealthCheckMethodHEAD, "", nil, SourceProtocolGemini, "", "", "", 0)

	go http.Serve(listener, srv.Handler())

	req, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/gemini/v1beta/models/gemini-2.5-flash:generateContent", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	req.Header.Set("x-api-key", "client-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(b))
	}

	if !strings.Contains(gotPath, "/v1beta/models/") || !strings.Contains(gotPath, ":generateContent") {
		t.Errorf("path = %q, want /v1beta/models/gemini-2.5-flash:generateContent", gotPath)
	}
	if gotGoogKey != "AIza-test-key" {
		t.Errorf("x-goog-api-key = %q, want AIza-test-key", gotGoogKey)
	}
}

// TestClaudeProtocol_E2E 验证 Claude 协议完整流程（默认协议）
func TestClaudeProtocol_E2E(t *testing.T) {
	var gotPath string
	var gotAPIKey string

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude-sonnet-4-5","stop_reason":"end_turn","usage":{"input_tokens":15,"output_tokens":25}}`))
	}))
	defer up.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(up.URL).
		WithRetry(1).
		WithListenAddr(listener.Addr().String()))

	acc, _ := srv.createAccount("test", "client-key", "pw", false)
	srv.addNodeWithMethod(acc, "claude-node", up.URL, "sk-ant-test", 1, HealthCheckMethodHEAD, "", nil, SourceProtocolClaude, "", "", "", 0)

	go http.Serve(listener, srv.Handler())

	body := `{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/v1/messages", strings.NewReader(body))
	req.Header.Set("x-api-key", "client-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(b))
	}

	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", gotPath)
	}
	if gotAPIKey != "sk-ant-test" {
		t.Errorf("x-api-key = %q, want sk-ant-test", gotAPIKey)
	}
}

// TestParseUsage_OpenAIGeminiFormats 验证 parseUsage 能正确解析三种协议的 usage 格式
// 基于 cc-switch 的真实响应数据结构
func TestParseUsage_OpenAIGeminiFormats(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantIn  int64
		wantOut int64
	}{
		{
			name:   "Claude format (cc-switch ClaudeAdapter)",
			data:   `{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":200}}`,
			wantIn: 100, wantOut: 200,
		},
		{
			name:   "OpenAI format (cc-switch CodexAdapter)",
			data:   `{"id":"chatcmpl-abc","object":"chat.completion","created":1234,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":300,"completion_tokens":400,"total_tokens":700}}`,
			wantIn: 300, wantOut: 400,
		},
		{
			name:   "Gemini format (cc-switch GeminiAdapter)",
			data:   `{"candidates":[],"usageMetadata":{"promptTokenCount":500,"candidatesTokenCount":600,"totalTokenCount":1100}}`,
			wantIn: 500, wantOut: 600,
		},
		{
			name:   "OpenAI SSE chunk",
			data:   `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20}}`,
			wantIn: 10, wantOut: 20,
		},
		{
			name:   "Gemini SSE chunk",
			data:   `data: {"candidates":[],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3,"totalTokenCount":10}}`,
			wantIn: 7, wantOut: 3,
		},
		{
			name:   "Claude SSE with message_stop",
			data:   `event: message_stop\ndata: {"type":"message_stop","message":{"usage":{"input_tokens":50,"output_tokens":75}}}`,
			wantIn: 50, wantOut: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, out := parseUsage([]byte(tt.data))
			if in != tt.wantIn {
				t.Errorf("input = %d, want %d", in, tt.wantIn)
			}
			if out != tt.wantOut {
				t.Errorf("output = %d, want %d", out, tt.wantOut)
			}
		})
	}
}

// TestProtocolAuthHeaders 验证不同协议使用正确的认证头
// cc-switch CodexAdapter 使用 Bearer，GeminiAdapter 使用 x-goog-api-key
func TestProtocolAuthHeaders(t *testing.T) {
	tests := []struct {
		name       string
		protocol   string
		apiKey     string
		wantHeader string
		wantValue  string
	}{
		{"Claude uses x-api-key", SourceProtocolClaude, "sk-ant-key", "x-api-key", "sk-ant-key"},
		{"OpenAI uses Bearer", SourceProtocolOpenAI, "sk-openai-key", "Authorization", "Bearer sk-openai-key"},
		{"Gemini uses x-goog-api-key", SourceProtocolGemini, "AIza-test-key", "x-goog-api-key", "AIza-test-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotHeader string
			var gotValue string

			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch tt.protocol {
				case SourceProtocolOpenAI:
					gotHeader = "Authorization"
					gotValue = r.Header.Get("Authorization")
				case SourceProtocolGemini:
					gotHeader = "x-goog-api-key"
					gotValue = r.Header.Get("x-goog-api-key")
				default:
					gotHeader = "x-api-key"
					gotValue = r.Header.Get("x-api-key")
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
			}))
			defer up.Close()

			nodeURL, _ := url.Parse(up.URL)
			node := &Node{
				ID:             "test",
				Name:           "test",
				URL:            nodeURL,
				APIKey:         tt.apiKey,
				SourceProtocol: tt.protocol,
			}

			srv := &Server{transport: http.DefaultTransport, logger: log.New(io.Discard, "", 0)}
			proxy, _ := srv.newReverseProxy(node, nil, nil, false)

			req := httptest.NewRequest(http.MethodPost, up.URL+"/v1/messages", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)

			if gotHeader != tt.wantHeader {
				t.Errorf("header = %q, want %q", gotHeader, tt.wantHeader)
			}
			if gotValue != tt.wantValue {
				t.Errorf("value = %q, want %q", gotValue, tt.wantValue)
			}
		})
	}
}

// TestProtocolIsolation 验证 OpenAI 入口不会路由到 Claude 节点
func TestProtocolIsolation(t *testing.T) {
	claudeHits := 0
	openaiHits := 0

	claudeUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claudeHits++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"text":"claude"}]}`))
	}))
	defer claudeUp.Close()

	openaiUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openaiHits++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"openai"}}]}`))
	}))
	defer openaiUp.Close()

	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(claudeUp.URL).
		WithRetry(1).
		WithListenAddr(listener.Addr().String()))

	acc, _ := srv.createAccount("test", "client-key", "pw", false)
	srv.addNodeWithMethod(acc, "claude-node", claudeUp.URL, "key1", 1, HealthCheckMethodHEAD, "", nil, SourceProtocolClaude, "", "", "", 0)
	srv.addNodeWithMethod(acc, "openai-node", openaiUp.URL, "key2", 2, HealthCheckMethodHEAD, "", nil, SourceProtocolOpenAI, "", "", "", 0)

	go http.Serve(listener, srv.Handler())

	req, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{}]}`))
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if claudeHits > 0 {
		t.Errorf("OpenAI request should NOT hit Claude node, but got %d hits", claudeHits)
	}
	if openaiHits != 1 {
		t.Errorf("OpenAI request should hit OpenAI node exactly once, got %d", openaiHits)
	}
}
