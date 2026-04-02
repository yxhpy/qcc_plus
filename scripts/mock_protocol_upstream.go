package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type nodeMode struct {
	RequestMode string `json:"request_mode"`
	HealthMode  string `json:"health_mode"`

	RequestStatus int `json:"request_status"`
	HealthStatus  int `json:"health_status"`

	RequestText string `json:"request_text"`
	HealthText  string `json:"health_text"`

	RequestHits int `json:"request_hits"`
	HealthHits  int `json:"health_hits"`

	LastPath          string `json:"last_path"`
	LastAuth          string `json:"last_auth"`
	LastAPIKey        string `json:"last_api_key"`
	LastGoogAPIKey    string `json:"last_goog_api_key"`
	LastBody          string `json:"last_body"`
	LastHealthRequest bool   `json:"last_health_request"`
}

type serverState struct {
	mu    sync.Mutex
	nodes map[string]*nodeMode
}

func newServerState() *serverState {
	return &serverState{nodes: make(map[string]*nodeMode)}
}

func (s *serverState) getNode(name string) *nodeMode {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mode, ok := s.nodes[name]; ok {
		return mode
	}
	mode := &nodeMode{
		RequestMode:   "success",
		HealthMode:    "success",
		RequestStatus: http.StatusOK,
		HealthStatus:  http.StatusOK,
		RequestText:   name + "-request-ok",
		HealthText:    name + "-health-ok",
	}
	s.nodes[name] = mode
	return mode
}

func (s *serverState) snapshot() map[string]nodeMode {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]nodeMode, len(s.nodes))
	for name, mode := range s.nodes {
		out[name] = *mode
	}
	return out
}

func main() {
	port := os.Getenv("PORT")
	if strings.TrimSpace(port) == "" {
		port = "19081"
	}

	state := newServerState()
	mux := http.NewServeMux()
	mux.HandleFunc("/__health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/__stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"nodes": state.snapshot()})
	})
	mux.HandleFunc("/__control/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/__control/")
		if strings.TrimSpace(name) == "" {
			http.NotFound(w, r)
			return
		}
		mode := state.getNode(name)
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, mode)
		case http.MethodPost:
			var req struct {
				RequestMode   *string `json:"request_mode"`
				HealthMode    *string `json:"health_mode"`
				RequestStatus *int    `json:"request_status"`
				HealthStatus  *int    `json:"health_status"`
				RequestText   *string `json:"request_text"`
				HealthText    *string `json:"health_text"`
				ResetStats    bool    `json:"reset_stats"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
				return
			}
			state.mu.Lock()
			if req.RequestMode != nil {
				mode.RequestMode = normalizeMode(*req.RequestMode)
			}
			if req.HealthMode != nil {
				mode.HealthMode = normalizeMode(*req.HealthMode)
			}
			if req.RequestStatus != nil {
				mode.RequestStatus = *req.RequestStatus
			}
			if req.HealthStatus != nil {
				mode.HealthStatus = *req.HealthStatus
			}
			if req.RequestText != nil {
				mode.RequestText = *req.RequestText
			}
			if req.HealthText != nil {
				mode.HealthText = *req.HealthText
			}
			if req.ResetStats {
				mode.RequestHits = 0
				mode.HealthHits = 0
				mode.LastPath = ""
				mode.LastAuth = ""
				mode.LastAPIKey = ""
				mode.LastGoogAPIKey = ""
				mode.LastBody = ""
				mode.LastHealthRequest = false
			}
			snapshot := *mode
			state.mu.Unlock()
			writeJSON(w, http.StatusOK, snapshot)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/up/", func(w http.ResponseWriter, r *http.Request) {
		handleUpstream(state, w, r)
	})

	addr := ":" + port
	log.Printf("mock protocol upstream listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleUpstream(state *serverState, w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/up/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	nodeName := parts[0]
	upstreamPath := "/" + parts[1]
	mode := state.getNode(nodeName)

	bodyBytes := []byte{}
	if r.Body != nil {
		bodyBytes, _ = ioReadAll(r)
	}
	bodyText := string(bodyBytes)
	isHealth := isHealthRequest(upstreamPath, bodyText)

	state.mu.Lock()
	mode.LastPath = upstreamPath
	mode.LastAuth = r.Header.Get("Authorization")
	mode.LastAPIKey = r.Header.Get("x-api-key")
	mode.LastGoogAPIKey = r.Header.Get("x-goog-api-key")
	mode.LastBody = truncate(bodyText, 512)
	mode.LastHealthRequest = isHealth
	if isHealth {
		mode.HealthHits++
	} else {
		mode.RequestHits++
	}
	snapshot := *mode
	state.mu.Unlock()

	responseText := snapshot.RequestText
	responseStatus := snapshot.RequestStatus
	responseMode := snapshot.RequestMode
	if isHealth {
		responseText = snapshot.HealthText
		responseStatus = snapshot.HealthStatus
		responseMode = snapshot.HealthMode
	}

	if responseStatus == 0 {
		responseStatus = http.StatusOK
	}
	if responseMode == "fail" {
		writeProtocolFailure(w, upstreamPath, responseStatus, nodeName, responseText)
		return
	}
	writeProtocolSuccess(w, upstreamPath, nodeName, responseText)
}

func writeProtocolSuccess(w http.ResponseWriter, upstreamPath, nodeName, text string) {
	if text == "" {
		text = nodeName + "-ok"
	}
	switch {
	case strings.Contains(upstreamPath, "/v1/chat/completions"):
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      "chatcmpl-mock",
			"object":  "chat.completion",
			"created": 1,
			"model":   "mock-openai-model",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 7,
				"total_tokens":      12,
			},
		})
	case strings.Contains(upstreamPath, ":generateContent"):
		writeJSON(w, http.StatusOK, map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"parts": []map[string]string{{"text": text}},
				},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]any{
				"promptTokenCount":     4,
				"candidatesTokenCount": 6,
				"totalTokenCount":      10,
			},
		})
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    "msg_mock",
			"type":  "message",
			"role":  "assistant",
			"model": "mock-claude-model",
			"content": []map[string]string{{
				"type": "text",
				"text": text,
			}},
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  5,
				"output_tokens": 7,
			},
		})
	}
}

func writeProtocolFailure(w http.ResponseWriter, upstreamPath string, status int, nodeName, text string) {
	if text == "" {
		text = nodeName + "-fail"
	}
	if status < 400 {
		status = http.StatusServiceUnavailable
	}
	switch {
	case strings.Contains(upstreamPath, "/v1/chat/completions"):
		writeJSONStatus(w, status, map[string]any{
			"error": map[string]any{
				"message": text,
				"type":    "mock_upstream_error",
			},
		})
	case strings.Contains(upstreamPath, ":generateContent"):
		writeJSONStatus(w, status, map[string]any{
			"error": map[string]any{
				"message": text,
				"status":  strconv.Itoa(status),
			},
		})
	default:
		writeJSONStatus(w, status, map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "mock_upstream_error",
				"message": text,
			},
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	writeJSONStatus(w, status, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func normalizeMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fail":
		return "fail"
	default:
		return "success"
	}
}

func isHealthRequest(path, body string) bool {
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		lower := strings.ToLower(body)
		return strings.Contains(lower, `"max_tokens":1,`) || strings.Contains(lower, `"content":"hi"`)
	}

	if strings.Contains(path, "/v1/chat/completions") {
		if maxTokens, ok := payload["max_tokens"].(float64); ok && int(maxTokens) == 1 {
			if messages, ok := payload["messages"].([]any); ok && len(messages) > 0 {
				if msg, ok := messages[0].(map[string]any); ok {
					if content, ok := msg["content"].(string); ok && strings.TrimSpace(strings.ToLower(content)) == "ping" {
						return true
					}
				}
			}
		}
		return false
	}

	if strings.Contains(path, ":generateContent") {
		if contents, ok := payload["contents"].([]any); ok && len(contents) > 0 {
			if item, ok := contents[0].(map[string]any); ok {
				if parts, ok := item["parts"].([]any); ok && len(parts) > 0 {
					if part, ok := parts[0].(map[string]any); ok {
						if text, ok := part["text"].(string); ok && strings.TrimSpace(strings.ToLower(text)) == "ping" {
							return true
						}
					}
				}
			}
		}
		return false
	}

	if maxTokens, ok := payload["max_tokens"].(float64); ok && int(maxTokens) == 1 {
		if messages, ok := payload["messages"].([]any); ok && len(messages) > 0 {
			if msg, ok := messages[0].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok && strings.TrimSpace(strings.ToLower(content)) == "hi" {
					return true
				}
			}
		}
	}
	return false
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit]
}

func ioReadAll(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
