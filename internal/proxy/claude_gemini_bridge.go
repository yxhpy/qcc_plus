package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

type protocolBridgeContextKey struct{}

const (
	SourceProtocolClaude = "claude"
	SourceProtocolGemini = "gemini"

	geminiModelsPrefix         = "/v1beta/models/"
	geminiGenerateSuffix       = ":generateContent"
	geminiStreamGenerateSuffix = ":streamGenerateContent"
)

type protocolBridge struct {
	requestPath string
	model       string
	streaming   bool
}

func newProtocolBridge(requestPath, ingressProtocol, targetProtocol, model string, streaming bool) *protocolBridge {
	if requestPath != "/v1/messages" {
		return nil
	}
	if ingressProtocol != SourceProtocolClaude || targetProtocol != SourceProtocolGemini {
		return nil
	}
	if strings.TrimSpace(model) == "" {
		return nil
	}
	return &protocolBridge{
		requestPath: requestPath,
		model:       strings.TrimSpace(model),
		streaming:   streaming,
	}
}

func protocolBridgeFromRequest(req *http.Request) *protocolBridge {
	if req == nil {
		return nil
	}
	if v := req.Context().Value(protocolBridgeContextKey{}); v != nil {
		if bridge, ok := v.(*protocolBridge); ok {
			return bridge
		}
	}
	return nil
}

func (b *protocolBridge) enabled() bool {
	return b != nil && strings.TrimSpace(b.model) != ""
}

func (b *protocolBridge) upstreamPath() string {
	if !b.enabled() {
		return ""
	}
	if b.streaming {
		return geminiModelsPrefix + b.model + geminiStreamGenerateSuffix
	}
	return geminiModelsPrefix + b.model + geminiGenerateSuffix
}

func (b *protocolBridge) upstreamRawQuery() string {
	if !b.enabled() || !b.streaming {
		return ""
	}
	return "alt=sse"
}

func extractGeminiTargetModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if idx := strings.LastIndex(model, "/"); idx >= 0 && idx < len(model)-1 {
		return strings.TrimSpace(model[idx+1:])
	}
	return model
}

func buildGeminiRequestFromClaude(body []byte, targetModel string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid Claude request JSON: %w", err)
	}

	if strings.TrimSpace(targetModel) == "" {
		return nil, fmt.Errorf("Gemini target model is required")
	}

	toolNamesByID, err := collectClaudeToolUseNames(payload["messages"])
	if err != nil {
		return nil, err
	}

	contents, err := convertClaudeMessagesToGemini(payload["messages"], toolNamesByID)
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("messages must contain at least one supported text/tool block")
	}

	out := map[string]any{
		"contents": contents,
	}

	systemInstruction, err := convertClaudeSystemToGemini(payload["system"])
	if err != nil {
		return nil, err
	}
	if systemInstruction != nil {
		out["systemInstruction"] = systemInstruction
	}

	tools, toolConfig, err := convertClaudeToolsToGemini(payload["tools"], payload["tool_choice"])
	if err != nil {
		return nil, err
	}
	if len(tools) > 0 {
		out["tools"] = tools
	}
	if toolConfig != nil {
		out["toolConfig"] = toolConfig
	}

	generationConfig := map[string]any{}
	if maxTokens := jsonNumberToInt64(payload["max_tokens"]); maxTokens > 0 {
		generationConfig["maxOutputTokens"] = maxTokens
	}
	if temperature, ok := jsonNumberToFloat64(payload["temperature"]); ok {
		generationConfig["temperature"] = temperature
	}
	if topP, ok := jsonNumberToFloat64(payload["top_p"]); ok {
		generationConfig["topP"] = topP
	}
	if stopSequences := jsonValueToStringSlice(payload["stop_sequences"]); len(stopSequences) > 0 {
		generationConfig["stopSequences"] = stopSequences
	}
	if len(generationConfig) > 0 {
		out["generationConfig"] = generationConfig
	}

	return json.Marshal(out)
}

func convertClaudeSystemToGemini(raw any) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	parts := make([]any, 0, 2)
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			parts = append(parts, map[string]any{"text": v})
		}
	case []any:
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("system blocks must be JSON objects")
			}
			if jsonString(obj["type"]) != "text" {
				return nil, fmt.Errorf("unsupported system block type %q for Gemini bridge", jsonString(obj["type"]))
			}
			text := jsonString(obj["text"])
			if strings.TrimSpace(text) != "" {
				parts = append(parts, map[string]any{"text": text})
			}
		}
	default:
		return nil, fmt.Errorf("unsupported system payload for Gemini bridge")
	}

	if len(parts) == 0 {
		return nil, nil
	}
	return map[string]any{"parts": parts}, nil
}

func collectClaudeToolUseNames(rawMessages any) (map[string]string, error) {
	result := make(map[string]string)
	messages, ok := rawMessages.([]any)
	if !ok {
		return result, nil
	}
	for _, item := range messages {
		msg, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("messages must be JSON objects")
		}
		blocks, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, rawBlock := range blocks {
			block, ok := rawBlock.(map[string]any)
			if !ok {
				continue
			}
			if jsonString(block["type"]) != "tool_use" {
				continue
			}
			id := strings.TrimSpace(jsonString(block["id"]))
			name := strings.TrimSpace(jsonString(block["name"]))
			if id != "" && name != "" {
				result[id] = name
			}
		}
	}
	return result, nil
}

func convertClaudeMessagesToGemini(rawMessages any, toolNamesByID map[string]string) ([]any, error) {
	messages, ok := rawMessages.([]any)
	if !ok {
		return nil, fmt.Errorf("messages must be an array")
	}

	contents := make([]any, 0, len(messages))
	for _, rawMessage := range messages {
		msg, ok := rawMessage.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("messages must be JSON objects")
		}
		role := strings.TrimSpace(jsonString(msg["role"]))
		if role == "" {
			return nil, fmt.Errorf("message role is required")
		}
		parts, err := convertClaudeContentToGeminiParts(msg["content"], toolNamesByID)
		if err != nil {
			return nil, err
		}
		if len(parts) == 0 {
			continue
		}
		geminiRole := "user"
		if role == "assistant" {
			geminiRole = "model"
		}
		contents = append(contents, map[string]any{
			"role":  geminiRole,
			"parts": parts,
		})
	}
	return contents, nil
}

func convertClaudeContentToGeminiParts(rawContent any, toolNamesByID map[string]string) ([]any, error) {
	switch v := rawContent.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []any{map[string]any{"text": v}}, nil
	case []any:
		parts := make([]any, 0, len(v))
		for _, rawBlock := range v {
			block, ok := rawBlock.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("message content blocks must be JSON objects")
			}
			switch jsonString(block["type"]) {
			case "text":
				text := jsonString(block["text"])
				if strings.TrimSpace(text) != "" {
					parts = append(parts, map[string]any{"text": text})
				}
			case "tool_use":
				name := strings.TrimSpace(jsonString(block["name"]))
				if name == "" {
					return nil, fmt.Errorf("tool_use name is required")
				}
				args := jsonObject(block["input"])
				id := strings.TrimSpace(jsonString(block["id"]))
				if id != "" {
					toolNamesByID[id] = name
				}
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": name,
						"args": args,
					},
				})
			case "tool_result":
				toolUseID := strings.TrimSpace(jsonString(block["tool_use_id"]))
				if toolUseID == "" {
					return nil, fmt.Errorf("tool_result tool_use_id is required")
				}
				name := strings.TrimSpace(toolNamesByID[toolUseID])
				if name == "" {
					return nil, fmt.Errorf("tool_result references unknown tool_use_id %q", toolUseID)
				}
				response := map[string]any{
					"content": extractClaudeToolResultContent(block["content"]),
				}
				if isError, ok := block["is_error"].(bool); ok && isError {
					response["is_error"] = true
				}
				parts = append(parts, map[string]any{
					"functionResponse": map[string]any{
						"name":     name,
						"response": response,
					},
				})
			default:
				return nil, fmt.Errorf("unsupported content block type %q for Gemini bridge", jsonString(block["type"]))
			}
		}
		return parts, nil
	default:
		return nil, fmt.Errorf("unsupported content payload for Gemini bridge")
	}
}

func extractClaudeToolResultContent(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if ok && jsonString(obj["type"]) == "text" {
				parts = append(parts, jsonString(obj["text"]))
				continue
			}
			buf, err := json.Marshal(item)
			if err == nil {
				parts = append(parts, string(buf))
			}
		}
		return strings.Join(parts, "\n")
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(buf)
	}
}

func convertClaudeToolsToGemini(rawTools any, rawToolChoice any) ([]any, map[string]any, error) {
	if rawTools == nil {
		return nil, nil, nil
	}
	items, ok := rawTools.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("tools must be an array")
	}
	functionDeclarations := make([]any, 0, len(items))
	for _, rawTool := range items {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("tools must be JSON objects")
		}
		name := strings.TrimSpace(jsonString(tool["name"]))
		if name == "" {
			return nil, nil, fmt.Errorf("tool name is required")
		}
		declaration := map[string]any{
			"name": name,
		}
		if description := strings.TrimSpace(jsonString(tool["description"])); description != "" {
			declaration["description"] = description
		}
		if schema := jsonObjectOrNil(tool["input_schema"]); schema != nil {
			declaration["parameters"] = schema
		}
		functionDeclarations = append(functionDeclarations, declaration)
	}
	if len(functionDeclarations) == 0 {
		return nil, nil, nil
	}

	tools := []any{map[string]any{"functionDeclarations": functionDeclarations}}
	toolConfig, err := convertClaudeToolChoiceToGemini(rawToolChoice)
	if err != nil {
		return nil, nil, err
	}
	return tools, toolConfig, nil
}

func convertClaudeToolChoiceToGemini(rawToolChoice any) (map[string]any, error) {
	if rawToolChoice == nil {
		return nil, nil
	}
	obj, ok := rawToolChoice.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tool_choice must be a JSON object")
	}
	switch strings.TrimSpace(jsonString(obj["type"])) {
	case "", "auto":
		return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}, nil
	case "any":
		return map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}, nil
	case "tool":
		name := strings.TrimSpace(jsonString(obj["name"]))
		if name == "" {
			return nil, fmt.Errorf("tool_choice.name is required when tool_choice.type=tool")
		}
		return map[string]any{
			"functionCallingConfig": map[string]any{
				"mode":                 "ANY",
				"allowedFunctionNames": []string{name},
			},
		}, nil
	case "none":
		return map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}, nil
	default:
		return nil, fmt.Errorf("unsupported tool_choice type %q for Gemini bridge", jsonString(obj["type"]))
	}
}

func translateGeminiResponseToClaude(body []byte, model string) ([]byte, error) {
	payload, err := parseGeminiPayload(body)
	if err != nil {
		return nil, err
	}

	candidate := firstGeminiCandidate(payload)
	content, sawToolUse, err := geminiCandidateToClaudeContent(candidate)
	if err != nil {
		return nil, err
	}
	inputTokens, outputTokens := geminiUsageFromPayload(payload)
	resp := map[string]any{
		"id":            nextBridgeMessageID(),
		"type":          "message",
		"role":          "assistant",
		"model":         chooseNonEmpty(jsonString(candidate["modelVersion"]), model),
		"content":       content,
		"stop_reason":   mapGeminiFinishReason(jsonString(candidate["finishReason"]), sawToolUse),
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}
	return json.Marshal(resp)
}

func translateGeminiResponseToClaudeSSE(body []byte, model string) ([]byte, error) {
	payload, err := parseGeminiPayload(body)
	if err != nil {
		return nil, err
	}

	candidate := firstGeminiCandidate(payload)
	content, sawToolUse, err := geminiCandidateToClaudeContent(candidate)
	if err != nil {
		return nil, err
	}
	inputTokens, outputTokens := geminiUsageFromPayload(payload)
	stopReason := mapGeminiFinishReason(jsonString(candidate["finishReason"]), sawToolUse)

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	messageID := nextBridgeMessageID()
	modelName := chooseNonEmpty(jsonString(candidate["modelVersion"]), model)
	if err := writeClaudeSSEEvent(writer, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      messageID,
			"type":    "message",
			"role":    "assistant",
			"model":   modelName,
			"content": []any{},
		},
	}); err != nil {
		return nil, err
	}

	for idx, rawBlock := range content {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			continue
		}
		if err := writeClaudeSSEEvent(writer, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         idx,
			"content_block": block,
		}); err != nil {
			return nil, err
		}
		if jsonString(block["type"]) == "text" {
			if err := writeClaudeSSEEvent(writer, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": idx,
				"delta": map[string]any{
					"type": "text_delta",
					"text": jsonString(block["text"]),
				},
			}); err != nil {
				return nil, err
			}
		}
		if err := writeClaudeSSEEvent(writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": idx,
		}); err != nil {
			return nil, err
		}
	}

	if err := writeClaudeSSEEvent(writer, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}); err != nil {
		return nil, err
	}
	if err := writeClaudeSSEEvent(writer, "message_stop", map[string]any{
		"type": "message_stop",
	}); err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseGeminiPayload(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid Gemini response JSON: %w", err)
	}
	return payload, nil
}

func firstGeminiCandidate(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return map[string]any{}
	}
	candidate, _ := candidates[0].(map[string]any)
	if candidate == nil {
		return map[string]any{}
	}
	return candidate
}

func geminiCandidateToClaudeContent(candidate map[string]any) ([]any, bool, error) {
	content := make([]any, 0, 2)
	sawToolUse := false
	contentObj := jsonObjectOrNil(candidate["content"])
	parts, _ := contentObj["parts"].([]any)
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		if text := jsonString(part["text"]); text != "" {
			content = append(content, map[string]any{
				"type": "text",
				"text": text,
			})
		}
		if functionCall := jsonObjectOrNil(part["functionCall"]); functionCall != nil {
			sawToolUse = true
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    nextBridgeToolUseID(),
				"name":  jsonString(functionCall["name"]),
				"input": jsonObject(functionCall["args"]),
			})
		}
	}
	return content, sawToolUse, nil
}

func geminiUsageFromPayload(payload map[string]any) (int64, int64) {
	usage := jsonObjectOrNil(payload["usageMetadata"])
	if usage == nil {
		return 0, 0
	}
	return jsonNumberToInt64(usage["promptTokenCount"]), jsonNumberToInt64(usage["candidatesTokenCount"])
}

func mapGeminiFinishReason(reason string, sawToolUse bool) string {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "MAX_TOKENS":
		return "max_tokens"
	case "STOP_SEQUENCE":
		return "stop_sequence"
	case "MALFORMED_FUNCTION_CALL":
		return "tool_use"
	case "", "STOP", "FINISH_REASON_UNSPECIFIED":
		if sawToolUse {
			return "tool_use"
		}
		return "end_turn"
	default:
		if sawToolUse {
			return "tool_use"
		}
		return "end_turn"
	}
}

func translateGeminiErrorToClaude(body []byte, status int) []byte {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(status)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if errObj := jsonObjectOrNil(payload["error"]); errObj != nil {
			if code := jsonNumberToInt64(errObj["code"]); code > 0 {
				status = int(code)
			}
			if msg := strings.TrimSpace(jsonString(errObj["message"])); msg != "" {
				message = msg
			}
		}
	}

	errorType := "api_error"
	switch status {
	case http.StatusBadRequest:
		errorType = "invalid_request_error"
	case http.StatusUnauthorized, http.StatusForbidden:
		errorType = "authentication_error"
	case http.StatusNotFound:
		errorType = "not_found_error"
	case http.StatusTooManyRequests:
		errorType = "rate_limit_error"
	}
	if status >= 500 {
		errorType = "api_error"
	}

	resp := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errorType,
			"message": message,
		},
	}
	translated, err := json.Marshal(resp)
	if err != nil {
		return []byte(`{"type":"error","error":{"type":"api_error","message":"gemini bridge error"}}`)
	}
	return translated
}

func writeClaudeAPIError(w http.ResponseWriter, status int, errorType, message string) {
	writeJSON(w, status, map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errorType,
			"message": message,
		},
	})
}

var bridgeMessageSeq uint64
var bridgeToolUseSeq uint64

func nextBridgeMessageID() string {
	return fmt.Sprintf("msg_gbridge_%d", atomic.AddUint64(&bridgeMessageSeq, 1))
}

func nextBridgeToolUseID() string {
	return fmt.Sprintf("toolu_gbridge_%d", atomic.AddUint64(&bridgeToolUseSeq, 1))
}

type geminiClaudeStreamReader struct {
	inner io.ReadCloser
	pipe  *io.PipeReader
}

func newGeminiToClaudeStreamReader(inner io.ReadCloser, model string) io.ReadCloser {
	pr, pw := io.Pipe()
	reader := &geminiClaudeStreamReader{
		inner: inner,
		pipe:  pr,
	}
	go func() {
		defer inner.Close()
		defer pw.Close()
		if err := streamGeminiToClaude(pw, inner, model); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	return reader
}

func (r *geminiClaudeStreamReader) Read(p []byte) (int, error) {
	return r.pipe.Read(p)
}

func (r *geminiClaudeStreamReader) Close() error {
	_ = r.inner.Close()
	return r.pipe.Close()
}

type geminiStreamEncoder struct {
	writer        *bufio.Writer
	model         string
	messageID     string
	nextIndex     int
	openTextIndex int
	inputTokens   int64
	outputTokens  int64
	stopReason    string
	sawToolUse    bool
	started       bool
}

func newGeminiStreamEncoder(w io.Writer, model string) *geminiStreamEncoder {
	return &geminiStreamEncoder{
		writer:        bufio.NewWriter(w),
		model:         model,
		messageID:     nextBridgeMessageID(),
		openTextIndex: -1,
	}
}

func streamGeminiToClaude(dst io.Writer, src io.Reader, model string) error {
	encoder := newGeminiStreamEncoder(dst, model)
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var dataLines []string

	flushEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if payload == "" || payload == "[DONE]" {
			return nil
		}
		return encoder.consumePayload(payload)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
		if line == "" {
			if err := flushEvent(); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flushEvent(); err != nil {
		return err
	}
	return encoder.finish()
}

func (e *geminiStreamEncoder) consumePayload(raw string) error {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	if err := e.ensureStart(); err != nil {
		return err
	}

	candidate := firstGeminiCandidate(payload)
	if finishReason := strings.TrimSpace(jsonString(candidate["finishReason"])); finishReason != "" {
		e.stopReason = mapGeminiFinishReason(finishReason, e.sawToolUse)
	}
	if in, out := geminiUsageFromPayload(payload); in > 0 || out > 0 {
		e.inputTokens = in
		e.outputTokens = out
	}

	content := jsonObjectOrNil(candidate["content"])
	parts, _ := content["parts"].([]any)
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		if text := jsonString(part["text"]); text != "" {
			if err := e.emitTextDelta(text); err != nil {
				return err
			}
		}
		if functionCall := jsonObjectOrNil(part["functionCall"]); functionCall != nil {
			if err := e.emitToolUse(functionCall); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *geminiStreamEncoder) ensureStart() error {
	if e.started {
		return nil
	}
	e.started = true
	return writeClaudeSSEEvent(e.writer, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      e.messageID,
			"type":    "message",
			"role":    "assistant",
			"model":   e.model,
			"content": []any{},
		},
	})
}

func (e *geminiStreamEncoder) emitTextDelta(text string) error {
	if err := e.ensureStart(); err != nil {
		return err
	}
	if e.openTextIndex < 0 {
		e.openTextIndex = e.nextIndex
		e.nextIndex++
		if err := writeClaudeSSEEvent(e.writer, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": e.openTextIndex,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		}); err != nil {
			return err
		}
	}
	return writeClaudeSSEEvent(e.writer, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": e.openTextIndex,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	})
}

func (e *geminiStreamEncoder) emitToolUse(functionCall map[string]any) error {
	if err := e.ensureStart(); err != nil {
		return err
	}
	if err := e.closeOpenTextBlock(); err != nil {
		return err
	}
	e.sawToolUse = true
	index := e.nextIndex
	e.nextIndex++
	if err := writeClaudeSSEEvent(e.writer, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type":  "tool_use",
			"id":    nextBridgeToolUseID(),
			"name":  jsonString(functionCall["name"]),
			"input": jsonObject(functionCall["args"]),
		},
	}); err != nil {
		return err
	}
	if err := writeClaudeSSEEvent(e.writer, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": index,
	}); err != nil {
		return err
	}
	if e.stopReason == "" {
		e.stopReason = "tool_use"
	}
	return nil
}

func (e *geminiStreamEncoder) closeOpenTextBlock() error {
	if e.openTextIndex < 0 {
		return nil
	}
	index := e.openTextIndex
	e.openTextIndex = -1
	return writeClaudeSSEEvent(e.writer, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": index,
	})
}

func (e *geminiStreamEncoder) finish() error {
	if err := e.ensureStart(); err != nil {
		return err
	}
	if err := e.closeOpenTextBlock(); err != nil {
		return err
	}
	stopReason := e.stopReason
	if stopReason == "" {
		stopReason = mapGeminiFinishReason("", e.sawToolUse)
	}
	if err := writeClaudeSSEEvent(e.writer, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"input_tokens":  e.inputTokens,
			"output_tokens": e.outputTokens,
		},
	}); err != nil {
		return err
	}
	if err := writeClaudeSSEEvent(e.writer, "message_stop", map[string]any{
		"type": "message_stop",
	}); err != nil {
		return err
	}
	return e.writer.Flush()
}

func writeClaudeSSEEvent(writer *bufio.Writer, event string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "data: %s\n\n", body); err != nil {
		return err
	}
	return writer.Flush()
}

func jsonString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func jsonObject(v any) map[string]any {
	if obj, ok := v.(map[string]any); ok && obj != nil {
		return obj
	}
	return map[string]any{}
}

func jsonObjectOrNil(v any) map[string]any {
	if obj, ok := v.(map[string]any); ok && obj != nil {
		return obj
	}
	return nil
}

func jsonNumberToInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func jsonNumberToFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func jsonValueToStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
