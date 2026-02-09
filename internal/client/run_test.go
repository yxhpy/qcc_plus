package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRun_WithWarmup(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: content_block_delta\n"))
		w.Write([]byte("data: {\"delta\":{\"text\":\"OK\"}}\n"))
		w.Write([]byte("\n"))
		w.Write([]byte("event: message_delta\n"))
		w.Write([]byte("data: {\"usage\":{}}\n"))
		w.Write([]byte("\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:       "test-token",
		BaseURL:     server.URL,
		Model:       "claude-sonnet-4-5-20250929",
		WarmupModel: "claude-haiku-4-5-20251001",
		NoWarmup:    false,
		Message:     "test message",
		Minimal:     true,
	}

	err := Run(cfg)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}

	// Should make 3 requests: 2 warmups + 1 main
	if requestCount != 3 {
		t.Errorf("Expected 3 requests (2 warmups + 1 main), got %d", requestCount)
	}
}

func TestRun_NoWarmup(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: content_block_delta\n"))
		w.Write([]byte("data: {\"delta\":{\"text\":\"OK\"}}\n"))
		w.Write([]byte("\n"))
		w.Write([]byte("event: message_delta\n"))
		w.Write([]byte("data: {\"usage\":{}}\n"))
		w.Write([]byte("\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:    "test-token",
		BaseURL:  server.URL,
		Model:    "claude-sonnet-4-5-20250929",
		NoWarmup: true,
		Message:  "test message",
		Minimal:  true,
	}

	err := Run(cfg)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}

	// Should make only 1 request (main)
	if requestCount != 1 {
		t.Errorf("Expected 1 request (main only), got %d", requestCount)
	}
}

func TestRun_WarmupError(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// First warmup fails
		if requestCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server error"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: message_delta\ndata: {}\n\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:       "test-token",
		BaseURL:     server.URL,
		Model:       "claude-sonnet-4-5-20250929",
		WarmupModel: "claude-haiku-4-5-20251001",
		NoWarmup:    false,
		Message:     "test message",
		Minimal:     true,
	}

	err := Run(cfg)
	if err == nil {
		t.Fatal("Expected error from warmup failure")
	}

	// Should stop after first warmup failure
	if requestCount != 1 {
		t.Errorf("Expected 1 request (failed warmup), got %d", requestCount)
	}
}

func TestRun_MainRequestError(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Warmups succeed, main request fails
		if requestCount <= 2 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("event: message_delta\ndata: {}\n\n"))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	cfg := Config{
		Token:       "test-token",
		BaseURL:     server.URL,
		Model:       "claude-sonnet-4-5-20250929",
		WarmupModel: "claude-haiku-4-5-20251001",
		NoWarmup:    false,
		Message:     "test message",
		Minimal:     true,
	}

	err := Run(cfg)
	if err == nil {
		t.Fatal("Expected error from main request failure")
	}

	// Should make all 3 requests
	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}
}

func TestRun_FullFlow(t *testing.T) {
	var receivedModels []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the request body to check which model was used
		var body Body
		if err := decodeJSON(r.Body, &body); err == nil {
			receivedModels = append(receivedModels, body.Model)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: content_block_delta\n"))
		w.Write([]byte("data: {\"delta\":{\"text\":\"Response\"}}\n"))
		w.Write([]byte("\n"))
		w.Write([]byte("event: message_delta\n"))
		w.Write([]byte("data: {\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n"))
		w.Write([]byte("\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:       "test-token",
		BaseURL:     server.URL,
		Model:       "claude-sonnet-4-5-20250929",
		WarmupModel: "claude-haiku-4-5-20251001",
		NoWarmup:    false,
		Message:     "test message",
		Minimal:     false,
	}

	err := Run(cfg)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}

	// Verify the models used
	if len(receivedModels) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(receivedModels))
	}

	// First warmup should use warmup model
	if receivedModels[0] != "claude-haiku-4-5-20251001" {
		t.Errorf("First warmup model = %s, want claude-haiku-4-5-20251001", receivedModels[0])
	}

	// Second warmup should use main model
	if receivedModels[1] != "claude-sonnet-4-5-20250929" {
		t.Errorf("Second warmup model = %s, want claude-sonnet-4-5-20250929", receivedModels[1])
	}

	// Main request should use main model
	if receivedModels[2] != "claude-sonnet-4-5-20250929" {
		t.Errorf("Main request model = %s, want claude-sonnet-4-5-20250929", receivedModels[2])
	}
}

func TestRun_ContextPropagation(t *testing.T) {
	// Test that context is properly used in requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context is attached to request
		if r.Context() == nil {
			t.Error("Request context is nil")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: message_delta\ndata: {}\n\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:    "test-token",
		BaseURL:  server.URL,
		Model:    "claude-sonnet-4-5-20250929",
		NoWarmup: true,
		Message:  "test",
		Minimal:  true,
	}

	err := Run(cfg)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRun_SecondWarmupError(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// First warmup succeeds, second fails
		if requestCount == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server error"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: message_delta\ndata: {}\n\n"))
	}))
	defer server.Close()

	cfg := Config{
		Token:       "test-token",
		BaseURL:     server.URL,
		Model:       "claude-sonnet-4-5-20250929",
		WarmupModel: "claude-haiku-4-5-20251001",
		NoWarmup:    false,
		Message:     "test message",
		Minimal:     true,
	}

	err := Run(cfg)
	if err == nil {
		t.Fatal("Expected error from second warmup failure")
	}

	// Should stop after second warmup failure
	if requestCount != 2 {
		t.Errorf("Expected 2 requests (first warmup + failed second warmup), got %d", requestCount)
	}
}

// Helper function to decode JSON
func decodeJSON(r interface{}, v interface{}) error {
	// Use json.NewDecoder for actual decoding
	if reader, ok := r.(interface{ Read([]byte) (int, error) }); ok {
		return json.NewDecoder(reader.(interface {
			Read([]byte) (int, error)
		})).Decode(v)
	}
	return nil
}
