package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestBuildGeminiRequestFromClaude(t *testing.T) {
	body := []byte(`{
		"model":"rc-gemini/gemini-2.5-flash",
		"system":[
			{"type":"text","text":"system one"},
			{"type":"text","text":"system two"}
		],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"hello"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"path":"README.md"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file content"}]}
		],
		"tools":[
			{
				"name":"Read",
				"description":"read file",
				"input_schema":{"type":"object","properties":{"path":{"type":"string"}}}
			}
		],
		"tool_choice":{"type":"tool","name":"Read"},
		"max_tokens":128,
		"temperature":0.2,
		"top_p":0.8,
		"stop_sequences":["DONE"]
	}`)

	translated, err := buildGeminiRequestFromClaude(body, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("buildGeminiRequestFromClaude error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(translated, &payload); err != nil {
		t.Fatalf("unmarshal translated payload: %v", err)
	}

	if _, exists := payload["model"]; exists {
		t.Fatalf("translated payload should not keep Claude model field")
	}

	systemInstruction, ok := payload["systemInstruction"].(map[string]any)
	if !ok {
		t.Fatalf("expected systemInstruction in translated payload")
	}
	parts, _ := systemInstruction["parts"].([]any)
	if len(parts) != 2 {
		t.Fatalf("expected 2 system parts, got %d", len(parts))
	}

	contents, ok := payload["contents"].([]any)
	if !ok || len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %#v", payload["contents"])
	}

	first := contents[0].(map[string]any)
	if first["role"] != "user" {
		t.Fatalf("expected first role user, got %v", first["role"])
	}
	firstParts := first["parts"].([]any)
	if got := firstParts[0].(map[string]any)["text"]; got != "hello" {
		t.Fatalf("expected first text hello, got %v", got)
	}

	second := contents[1].(map[string]any)
	if second["role"] != "model" {
		t.Fatalf("expected second role model, got %v", second["role"])
	}
	secondParts := second["parts"].([]any)
	functionCall := secondParts[0].(map[string]any)["functionCall"].(map[string]any)
	if functionCall["name"] != "Read" {
		t.Fatalf("expected functionCall name Read, got %v", functionCall["name"])
	}
	args := functionCall["args"].(map[string]any)
	if args["path"] != "README.md" {
		t.Fatalf("expected functionCall path README.md, got %v", args["path"])
	}

	third := contents[2].(map[string]any)
	functionResponse := third["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
	if functionResponse["name"] != "Read" {
		t.Fatalf("expected functionResponse name Read, got %v", functionResponse["name"])
	}
	response := functionResponse["response"].(map[string]any)
	if response["content"] != "file content" {
		t.Fatalf("expected functionResponse content file content, got %v", response["content"])
	}

	tools := payload["tools"].([]any)
	functionDeclarations := tools[0].(map[string]any)["functionDeclarations"].([]any)
	declaration := functionDeclarations[0].(map[string]any)
	if declaration["name"] != "Read" {
		t.Fatalf("expected function declaration Read, got %v", declaration["name"])
	}

	toolConfig := payload["toolConfig"].(map[string]any)["functionCallingConfig"].(map[string]any)
	if toolConfig["mode"] != "ANY" {
		t.Fatalf("expected tool config ANY, got %v", toolConfig["mode"])
	}
	allowedNames := toolConfig["allowedFunctionNames"].([]any)
	if len(allowedNames) != 1 || allowedNames[0] != "Read" {
		t.Fatalf("expected allowedFunctionNames [Read], got %#v", allowedNames)
	}

	generationConfig := payload["generationConfig"].(map[string]any)
	if int(generationConfig["maxOutputTokens"].(float64)) != 128 {
		t.Fatalf("expected maxOutputTokens 128, got %v", generationConfig["maxOutputTokens"])
	}
	if generationConfig["temperature"].(float64) != 0.2 {
		t.Fatalf("expected temperature 0.2, got %v", generationConfig["temperature"])
	}
	if generationConfig["topP"].(float64) != 0.8 {
		t.Fatalf("expected topP 0.8, got %v", generationConfig["topP"])
	}
}

func TestBuildGeminiRequestFromClaudeRejectsUnsupportedContent(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"user","content":[{"type":"image","source":"x"}]}
		]
	}`)

	_, err := buildGeminiRequestFromClaude(body, "gemini-2.5-flash")
	if err == nil {
		t.Fatal("expected unsupported content error")
	}
	if !strings.Contains(err.Error(), `unsupported content block type "image"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTranslateGeminiResponseToClaude(t *testing.T) {
	body := []byte(`{
		"candidates":[
			{
				"content":{
					"parts":[
						{"text":"ok"},
						{"functionCall":{"name":"Read","args":{"path":"README.md"}}}
					]
				},
				"finishReason":"STOP",
				"modelVersion":"gemini-2.5-flash"
			}
		],
		"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":22}
	}`)

	translated, err := translateGeminiResponseToClaude(body, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("translateGeminiResponseToClaude error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(translated, &payload); err != nil {
		t.Fatalf("unmarshal translated response: %v", err)
	}

	if payload["type"] != "message" {
		t.Fatalf("expected type=message, got %v", payload["type"])
	}
	if payload["stop_reason"] != "tool_use" {
		t.Fatalf("expected stop_reason tool_use, got %v", payload["stop_reason"])
	}

	content := payload["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}
	if content[0].(map[string]any)["text"] != "ok" {
		t.Fatalf("expected first text block ok, got %#v", content[0])
	}
	toolUse := content[1].(map[string]any)
	if toolUse["type"] != "tool_use" || toolUse["name"] != "Read" {
		t.Fatalf("expected tool_use block for Read, got %#v", toolUse)
	}
	input := toolUse["input"].(map[string]any)
	if input["path"] != "README.md" {
		t.Fatalf("expected tool_use input path README.md, got %v", input["path"])
	}

	usage := payload["usage"].(map[string]any)
	if int(usage["input_tokens"].(float64)) != 11 || int(usage["output_tokens"].(float64)) != 22 {
		t.Fatalf("unexpected usage payload %#v", usage)
	}
}

func TestTranslateGeminiResponseToClaudeSSE(t *testing.T) {
	body := []byte(`{
		"candidates":[
			{
				"content":{"parts":[{"text":"ok"}]},
				"finishReason":"STOP"
			}
		],
		"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":5}
	}`)

	translated, err := translateGeminiResponseToClaudeSSE(body, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("translateGeminiResponseToClaudeSSE error: %v", err)
	}

	out := string(translated)
	for _, want := range []string{
		"event: message_start",
		"event: content_block_start",
		`"text":"ok"`,
		"event: content_block_delta",
		`"input_tokens":3`,
		`"output_tokens":5`,
		"event: message_stop",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("translated SSE missing %q in %s", want, out)
		}
	}
}

func TestStreamGeminiToClaude(t *testing.T) {
	input := strings.NewReader(`data: {"candidates":[{"content":{"parts":[{"text":"Hel"}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":"lo"}]}}]}

data: {"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2}}

`)

	var out bytes.Buffer
	if err := streamGeminiToClaude(&out, input, "gemini-2.5-flash"); err != nil {
		t.Fatalf("streamGeminiToClaude error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"event: message_start",
		`"text":"Hel"`,
		`"text":"lo"`,
		`"input_tokens":3`,
		`"output_tokens":2`,
		"event: message_stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream output missing %q in %s", want, got)
		}
	}
}

func TestStreamGeminiToClaudeIgnoresIncompleteTrailingJSON(t *testing.T) {
	input := strings.NewReader(`data: {"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":"broken"}]}}
`)

	var out bytes.Buffer
	if err := streamGeminiToClaude(&out, input, "gemini-2.5-flash"); err != nil {
		t.Fatalf("streamGeminiToClaude error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `"text":"ok"`) {
		t.Fatalf("expected valid first chunk to survive, got %s", got)
	}
	if strings.Contains(got, `"text":"broken"`) {
		t.Fatalf("expected incomplete trailing chunk to be dropped, got %s", got)
	}
}

func TestClaudeGeminiBridgeProxyStreamEndToEnd(t *testing.T) {
	var (
		mu       sync.Mutex
		gotPath  string
		gotQuery string
		gotBody  map[string]any
	)

	upstreamURL, closeUpstream := startLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hel\"}]}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":2}}\n\n"))
	}))
	defer closeUpstream()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer listener.Close()

	srv := buildServerNoWarmup(t, NewBuilder().
		WithUpstream(upstreamURL).
		WithRetry(1).
		WithListenAddr(listener.Addr().String()))

	acc, err := srv.createAccount("test-account", "client-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := srv.addNodeWithMethod(acc, "rc-gemini", upstreamURL, "AIza-test-key", 1, HealthCheckMethodAPI, "", nil, SourceProtocolGemini, "", ""); err != nil {
		t.Fatalf("add gemini node: %v", err)
	}

	proxyServer := &http.Server{Handler: srv.Handler()}
	go proxyServer.Serve(listener)
	defer proxyServer.Close()

	body := `{
		"model":"rc-gemini/gemini-2.5-flash",
		"system":[{"type":"text","text":"system"}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"tools":[{"name":"Read","description":"read","input_schema":{"type":"object"}}],
		"max_tokens":64,
		"stream":true
	}`
	req, _ := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/v1/messages", strings.NewReader(body))
	req.Header.Set("x-api-key", "client-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %s", resp.Header.Get("Content-Type"))
	}

	respBody, _ := io.ReadAll(resp.Body)
	got := string(respBody)
	for _, want := range []string{
		"event: message_start",
		`"text":"Hel"`,
		`"text":"lo"`,
		`"input_tokens":4`,
		`"output_tokens":2`,
		"event: message_stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("proxy SSE missing %q in %s", want, got)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/v1beta/models/gemini-2.5-flash:streamGenerateContent" {
		t.Fatalf("expected upstream streamGenerateContent path, got %s", gotPath)
	}
	if gotQuery != "alt=sse" {
		t.Fatalf("expected upstream alt=sse query, got %q", gotQuery)
	}
	if len(gotBody) == 0 {
		t.Fatal("expected translated request body")
	}
	if _, exists := gotBody["contents"]; !exists {
		t.Fatalf("expected translated Gemini contents in upstream body, got %#v", gotBody)
	}
}

func startLocalHTTPServer(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen local server: %v", err)
	}
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	return "http://" + listener.Addr().String(), func() {
		_ = server.Close()
		_ = listener.Close()
	}
}
