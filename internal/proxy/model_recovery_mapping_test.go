package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckModelRecoveryUsesMappedModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if model, _ := payload["model"].(string); model == "claude-sonnet-4-7" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"content":[{"text":"ok"}]}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"wrong model"}`))
	}))
	defer upstream.Close()

	b := NewBuilder().WithUpstream(upstream.URL)
	srv := buildServerNoWarmup(t, b)

	acc, err := srv.createAccount("test-account", "proxy-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node, err := srv.addNodeWithMethod(acc, "mapped-node", upstream.URL, "k", 1, HealthCheckMethodAPI, "", map[string]string{"claude-sonnet-4-6": "claude-sonnet-4-7"}, "", "", "", "", 0)
	if err != nil {
		t.Fatalf("add node: %v", err)
	}

	srv.modelRecovery.MarkFailed(node.ID, "claude-sonnet-4-6", acc.ID, "status 400")
	if !srv.modelRecovery.IsModelFailed(node.ID, "claude-sonnet-4-6") {
		t.Fatal("expected model to be marked failed before recovery check")
	}

	srv.checkModelRecovery(acc, node, "claude-sonnet-4-6")

	if srv.modelRecovery.IsModelFailed(node.ID, "claude-sonnet-4-6") {
		t.Fatal("expected mapped-model recovery check to clear failed state")
	}
}

func TestCheckModelRecovery_OpenAIProtocolUsesChatCompletionsPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"wrong path"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer upstream.Close()

	b := NewBuilder().WithUpstream(upstream.URL)
	srv := buildServerNoWarmup(t, b)

	acc, err := srv.createAccount("test-account", "proxy-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node, err := srv.addNodeWithMethod(acc, "openai-node", upstream.URL, "k", 1, HealthCheckMethodAPI, "", map[string]string{"claude-haiku-4-5-20251001": "gpt-5.1-codex-mini"}, SourceProtocolOpenAI, "", "", "", 0)
	if err != nil {
		t.Fatalf("add node: %v", err)
	}

	srv.modelRecovery.MarkFailed(node.ID, "claude-haiku-4-5-20251001", acc.ID, "status 404")
	srv.checkModelRecovery(acc, node, "claude-haiku-4-5-20251001")

	if srv.modelRecovery.IsModelFailed(node.ID, "claude-haiku-4-5-20251001") {
		t.Fatal("expected openai protocol recovery to succeed via /v1/chat/completions")
	}
}
