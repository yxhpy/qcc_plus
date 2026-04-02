package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildHealthProbeURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{
			name:    "base URL already has v1",
			baseURL: "https://example.com/v1",
			path:    "/v1/chat/completions",
			want:    "https://example.com/v1/chat/completions",
		},
		{
			name:    "base URL without v1",
			baseURL: "https://example.com",
			path:    "/v1/chat/completions",
			want:    "https://example.com/v1/chat/completions",
		},
		{
			name:    "base URL has trailing slash after v1",
			baseURL: "https://example.com/v1/",
			path:    "/v1/chat/completions",
			want:    "https://example.com/v1/chat/completions",
		},
		{
			name:    "base URL already has v1beta",
			baseURL: "https://example.com/v1beta",
			path:    "/v1beta/models/gemini-2.5-flash:generateContent",
			want:    "https://example.com/v1beta/models/gemini-2.5-flash:generateContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHealthProbeURL(tt.baseURL, tt.path)
			if got != tt.want {
				t.Fatalf("buildHealthProbeURL(%q, %q) = %q, want %q", tt.baseURL, tt.path, got, tt.want)
			}
		})
	}
}

// TestHealthCheck tests node health checking functionality
func TestHealthCheck(t *testing.T) {
	// Create a test server that can be controlled
	healthy := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":[{"text":"ok"}]}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer upstream.Close()

	b := NewBuilder().
		WithUpstream(upstream.URL).
		WithFailLimit(2).
		WithHealthEvery(100 * time.Millisecond)
	srv := buildServerNoWarmup(t, b)

	// Create test account and node
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node, err := srv.addNodeToAccount(acc, "test-node", upstream.URL, "", 1)
	if err != nil {
		t.Fatalf("add node: %v", err)
	}

	t.Run("Check healthy node", func(t *testing.T) {
		healthy = true
		srv.checkNodeHealth(acc, node.ID, "test")

		// Give it time to process
		time.Sleep(50 * time.Millisecond)

		// Verify node is not marked as failed
		srv.mu.RLock()
		failed := srv.nodeIndex[node.ID].Failed
		srv.mu.RUnlock()
		if failed {
			t.Error("expected node to not be marked as failed")
		}
	})

	t.Run("Check unhealthy node", func(t *testing.T) {
		healthy = false
		srv.checkNodeHealth(acc, node.ID, "test")

		// Give it time to process
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("Check non-existent node", func(t *testing.T) {
		srv.checkNodeHealth(acc, "non-existent-id", "test")
		// Should not panic
	})
}

// TestHandleFailure tests failure handling
func TestHandleFailure(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com").
		WithFailLimit(2)
	srv := buildServerNoWarmup(t, b)

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	node1, err := srv.addNodeToAccount(acc, "node1", "http://node1.com", "", 1)
	if err != nil {
		t.Fatalf("add node1: %v", err)
	}
	node2, err := srv.addNodeToAccount(acc, "node2", "http://node2.com", "", 2)
	if err != nil {
		t.Fatalf("add node2: %v", err)
	}

	t.Run("Handle first failure", func(t *testing.T) {
		srv.handleFailure(node1.ID, "test error")

		// Node should not be marked as failed yet (fail limit is 2)
		srv.mu.RLock()
		failStreak := srv.nodeIndex[node1.ID].Metrics.FailStreak
		srv.mu.RUnlock()
		if failStreak != 1 {
			t.Errorf("expected fail streak 1, got %d", failStreak)
		}
	})

	t.Run("Handle failure exceeding limit", func(t *testing.T) {
		// Second failure - handleFailure marks node as failed immediately
		// and doesn't increment fail streak (it only sets to 1 if 0)
		srv.handleFailure(node1.ID, "test error 2")

		srv.mu.RLock()
		failed := srv.nodeIndex[node1.ID].Failed
		failStreak := srv.nodeIndex[node1.ID].Metrics.FailStreak
		srv.mu.RUnlock()

		if !failed {
			t.Error("expected node to be marked as failed")
		}
		// handleFailure doesn't increment fail streak, it stays at 1
		if failStreak != 1 {
			t.Errorf("expected fail streak 1, got %d", failStreak)
		}
	})

	t.Run("Handle failure triggers node switch", func(t *testing.T) {
		// After node1 fails, active should switch to node2
		time.Sleep(50 * time.Millisecond) // Give it time to process

		srv.mu.RLock()
		activeID := srv.accountByID[acc.ID].ActiveID
		srv.mu.RUnlock()

		if activeID != node2.ID {
			t.Errorf("expected active node to switch to node2, got %s", activeID)
		}
	})
}

// TestCircuitBreaker tests circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	t.Run("Circuit breaker allows request when closed", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Enabled:          true,
			ConsecutiveFails: 3,
			CooldownSeconds:  10,
		})
		if !cb.AllowRequest() {
			t.Error("expected circuit breaker to allow request when closed")
		}
	})

	t.Run("Circuit breaker opens after failures", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Enabled:          true,
			ConsecutiveFails: 2,
			CooldownSeconds:  10,
		})

		// Record failures
		cb.RecordResult(false)
		cb.RecordResult(false)

		// Circuit should be open now
		if cb.AllowRequest() {
			t.Error("expected circuit breaker to be open after failures")
		}
	})

	t.Run("Circuit breaker records success", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Enabled:          true,
			ConsecutiveFails: 3,
			CooldownSeconds:  10,
			FailureRate:      0.5, // 50% failure rate threshold
		})

		cb.RecordResult(false) // 1 failure: rate = 100%
		cb.RecordResult(true)  // 1 failure, 1 success: rate = 50%

		// With 50% failure rate (exactly at threshold), circuit breaker opens
		// To keep it closed, we need more successes
		if cb.AllowRequest() {
			t.Error("expected circuit breaker to be open with 50% failure rate")
		}
	})

	t.Run("Circuit breaker disabled allows all requests", func(t *testing.T) {
		cb := NewCircuitBreaker(CircuitBreakerConfig{
			Enabled:          false,
			ConsecutiveFails: 1,
		})

		// Even after failures, should allow requests when disabled
		cb.RecordResult(false)
		cb.RecordResult(false)

		if !cb.AllowRequest() {
			t.Error("expected disabled circuit breaker to allow all requests")
		}
	})
}
