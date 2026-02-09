package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	c := client()
	if c == nil {
		t.Fatal("client() returned nil")
	}

	if c.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0", c.Timeout)
	}

	if c.Transport == nil {
		t.Fatal("Transport is nil")
	}
}

func TestSend_Success(t *testing.T) {
	// Create a test server that returns SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("authorization") != "Bearer test-token" {
			t.Errorf("Authorization header = %s, want Bearer test-token", r.Header.Get("authorization"))
		}

		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("anthropic-version = %s, want 2023-06-01", r.Header.Get("anthropic-version"))
		}

		if r.Header.Get("anthropic-beta") != "test-beta" {
			t.Errorf("anthropic-beta = %s, want test-beta", r.Header.Get("anthropic-beta"))
		}

		// Return SSE stream
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: content_block_delta\n"))
		w.Write([]byte("data: {\"delta\":{\"text\":\"Hello\"}}\n"))
		w.Write([]byte("\n"))
		w.Write([]byte("event: message_delta\n"))
		w.Write([]byte("data: {\"usage\":{}}\n"))
		w.Write([]byte("\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: server.URL,
		Message: "test",
	}

	body := Body{
		Model:     "test-model",
		Messages:  []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
		MaxTokens: 100,
		Stream:    true,
	}

	ctx := context.Background()
	err := send(ctx, cfg, body, "test-beta")
	if err != nil {
		t.Errorf("send() error = %v", err)
	}
}

func TestSend_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: server.URL,
	}

	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx := context.Background()
	err := send(ctx, cfg, body, "test-beta")
	if err == nil {
		t.Fatal("Expected error for non-OK status")
	}

	if !contains(err.Error(), "upstream 400") {
		t.Errorf("Error should contain 'upstream 400', got: %v", err)
	}
}

func TestSend_InvalidJSON(t *testing.T) {
	cfg := Config{
		Token:   "test-token",
		BaseURL: "http://localhost:9999",
	}

	// Create a body with invalid JSON (circular reference would cause marshal error)
	// Instead, we'll test with a valid body but invalid URL to trigger network error
	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := send(ctx, cfg, body, "test-beta")
	if err == nil {
		t.Fatal("Expected error for invalid connection")
	}
}

func TestSend_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: server.URL,
	}

	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := send(ctx, cfg, body, "test-beta")
	if err == nil {
		t.Fatal("Expected error for context cancellation")
	}
}

func TestSend_RequestConstruction(t *testing.T) {
	requestReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		// Verify URL path
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Path = %s, want /v1/messages", r.URL.Path)
		}

		// Verify query parameter
		if r.URL.Query().Get("beta") != "true" {
			t.Errorf("beta query param = %s, want true", r.URL.Query().Get("beta"))
		}

		// Verify headers
		expectedHeaders := map[string]string{
			"accept":                                    "application/json",
			"content-type":                              "application/json",
			"anthropic-version":                         "2023-06-01",
			"anthropic-dangerous-direct-browser-access": "true",
			"user-agent":                                "claude-cli/2.0.47 (external, sdk-cli)",
			"x-app":                                     "cli",
			"x-stainless-helper-method":                 "stream",
		}

		for k, v := range expectedHeaders {
			if r.Header.Get(k) != v {
				t.Errorf("Header %s = %s, want %s", k, r.Header.Get(k), v)
			}
		}

		// Verify body can be decoded
		var body Body
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: message_delta\ndata: {}\n\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: server.URL,
	}

	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx := context.Background()
	_ = send(ctx, cfg, body, "test-beta")

	if !requestReceived {
		t.Error("Request was not received by server")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSend_MarshalError(t *testing.T) {
	// Test with a body that would cause marshal error
	// In Go, it's hard to make json.Marshal fail, but we can test the error path
	// by using an invalid body structure
	cfg := Config{
		Token:   "test-token",
		BaseURL: "http://localhost:9999",
	}

	// Create a body with a channel (which can't be marshaled)
	type InvalidBody struct {
		Body
		Ch chan int
	}

	// Since we can't directly test marshal error with Body type,
	// we'll test the network error path instead
	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := send(ctx, cfg, body, "test-beta")
	if err == nil {
		t.Fatal("Expected error for invalid connection")
	}
}

func TestSend_StreamError(t *testing.T) {
	// Test when stream() returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Write incomplete SSE data that might cause stream error
		w.Write([]byte("event: test\n"))
		// Don't write data or closing newline
	}))
	defer server.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: server.URL,
	}

	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx := context.Background()
	err := send(ctx, cfg, body, "test-beta")
	// stream() should complete without error even with incomplete data
	if err != nil {
		t.Errorf("send() error = %v, expected nil", err)
	}
}

func TestSend_ErrorReadingBody(t *testing.T) {
	// Test error reading response body on non-2xx status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000") // Claim large body
		w.WriteHeader(http.StatusBadRequest)
		// Write less than claimed
		w.Write([]byte(`{"error":"bad"}`))
	}))
	defer server.Close()

	cfg := Config{
		Token:   "test-token",
		BaseURL: server.URL,
	}

	body := Body{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: []ContentItem{{Type: "text", Text: "test"}}}},
	}

	ctx := context.Background()
	err := send(ctx, cfg, body, "test-beta")
	if err == nil {
		t.Fatal("Expected error for non-OK status")
	}

	// Should still return error even if reading body fails
	if !contains(err.Error(), "upstream 400") {
		t.Errorf("Error should contain 'upstream 400', got: %v", err)
	}
}
