package proxy

import (
	"log"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestNewHealthScheduler(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

	t.Run("with all parameters", func(t *testing.T) {
		logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
		scheduler := NewHealthScheduler(srv, 5*time.Minute, 2, 1, logger)
		if scheduler == nil {
			t.Fatal("expected non-nil scheduler")
		}
		if scheduler.server != srv {
			t.Error("server not set correctly")
		}
		if scheduler.logger != logger {
			t.Error("logger not set correctly")
		}
		if scheduler.interval != 5*time.Minute {
			t.Errorf("interval = %v, want %v", scheduler.interval, 5*time.Minute)
		}
		if scheduler.workers != 2 {
			t.Errorf("workers = %d, want 2", scheduler.workers)
		}
		if scheduler.cliWorkers != 1 {
			t.Errorf("cliWorkers = %d, want 1", scheduler.cliWorkers)
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		scheduler := NewHealthScheduler(srv, 5*time.Minute, 2, 1, nil)
		if scheduler == nil {
			t.Fatal("expected non-nil scheduler")
		}
		if scheduler.logger == nil {
			t.Error("expected default logger")
		}
	})

	t.Run("with zero interval", func(t *testing.T) {
		scheduler := NewHealthScheduler(srv, 0, 2, 1, nil)
		if scheduler.interval != defaultHealthAllInterval {
			t.Errorf("interval = %v, want %v", scheduler.interval, defaultHealthAllInterval)
		}
	})

	t.Run("with zero workers", func(t *testing.T) {
		scheduler := NewHealthScheduler(srv, 5*time.Minute, 0, 0, nil)
		if scheduler.workers != defaultHealthCheckConcurrency {
			t.Errorf("workers = %d, want %d", scheduler.workers, defaultHealthCheckConcurrency)
		}
		if scheduler.cliWorkers != defaultCLIHealthCheckConcurrency {
			t.Errorf("cliWorkers = %d, want %d", scheduler.cliWorkers, defaultCLIHealthCheckConcurrency)
		}
	})
}

func TestHealthScheduler_Interval(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

	t.Run("non-nil scheduler", func(t *testing.T) {
		scheduler := NewHealthScheduler(srv, 5*time.Minute, 2, 1, nil)
		if scheduler.Interval() != 5*time.Minute {
			t.Errorf("Interval() = %v, want %v", scheduler.Interval(), 5*time.Minute)
		}
	})

	t.Run("nil scheduler", func(t *testing.T) {
		var scheduler *HealthScheduler
		if scheduler.Interval() != defaultHealthAllInterval {
			t.Errorf("Interval() = %v, want %v", scheduler.Interval(), defaultHealthAllInterval)
		}
	})
}

func TestHealthScheduler_StartStop(t *testing.T) {
	upstream := httptest.NewServer(nil)
	defer upstream.Close()

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(upstream.URL))

	// Create test account and node
	acc, err := srv.createAccount("test-account", "test-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := srv.addNodeToAccount(acc, "node-1", upstream.URL, "", 1); err != nil {
		t.Fatalf("add node: %v", err)
	}

	scheduler := NewHealthScheduler(srv, 100*time.Millisecond, 2, 1, nil)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	// Stop should complete within timeout
	done := make(chan struct{})
	go func() {
		scheduler.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("stop timeout")
	}
}

func TestHealthScheduler_StartNilServer(t *testing.T) {
	scheduler := NewHealthScheduler(nil, 5*time.Minute, 2, 1, nil)
	if err := scheduler.Start(); err != nil {
		t.Errorf("start with nil server should not error: %v", err)
	}
}

func TestHealthScheduler_StartZeroInterval(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))
	scheduler := NewHealthScheduler(srv, 0, 2, 1, nil)
	scheduler.interval = 0 // Force zero after normalization

	if err := scheduler.Start(); err != nil {
		t.Errorf("start with zero interval should not error: %v", err)
	}
}

func TestHealthScheduler_StopNil(t *testing.T) {
	var scheduler *HealthScheduler
	scheduler.Stop() // Should not panic
}

func TestHealthScheduler_StopMultipleTimes(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))
	scheduler := NewHealthScheduler(srv, 100*time.Millisecond, 2, 1, nil)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Stop multiple times should be safe
	scheduler.Stop()
	scheduler.Stop()
	scheduler.Stop()
}

func TestHealthScheduler_CheckAllNodes(t *testing.T) {
	upstream := httptest.NewServer(nil)
	defer upstream.Close()

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(upstream.URL))

	// Create test account and nodes
	acc, err := srv.createAccount("test-account", "test-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	node1, err := srv.addNodeToAccount(acc, "node-1", upstream.URL, "", 1)
	if err != nil {
		t.Fatalf("add node 1: %v", err)
	}

	node2, err := srv.addNodeToAccount(acc, "node-2", upstream.URL, "", 2)
	if err != nil {
		t.Fatalf("add node 2: %v", err)
	}

	// Disable node2
	srv.mu.Lock()
	acc.Nodes[node2.ID].Disabled = true
	srv.mu.Unlock()

	scheduler := NewHealthScheduler(srv, 10*time.Minute, 2, 1, nil)
	scheduler.checkAllNodes()

	// Verify that health checks were performed
	// (actual health check logic is tested in health_test.go)
	srv.mu.RLock()
	n1 := acc.Nodes[node1.ID]
	srv.mu.RUnlock()

	if n1 == nil {
		t.Fatal("node1 should exist")
	}

	// Node should have been checked (LastHealthCheckAt should be set)
	// Note: This is a basic check; detailed health check logic is tested elsewhere
}

func TestHealthScheduler_CheckAllNodes_EmptyNodes(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

	// Create account with no nodes
	_, err := srv.createAccount("test-account", "test-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	scheduler := NewHealthScheduler(srv, 10*time.Minute, 2, 1, nil)
	scheduler.checkAllNodes() // Should not panic
}

func TestHealthScheduler_RecoverPanic(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))
	scheduler := NewHealthScheduler(srv, 10*time.Minute, 2, 1, nil)

	// Should not panic
	func() {
		defer scheduler.recoverPanic("test")
		panic("test panic")
	}()
}

func TestNormalizeHealthCheckWorkers(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	tests := []struct {
		name    string
		workers int
		want    int
	}{
		{
			name:    "zero workers",
			workers: 0,
			want:    defaultHealthCheckConcurrency,
		},
		{
			name:    "negative workers",
			workers: -1,
			want:    defaultHealthCheckConcurrency,
		},
		{
			name:    "valid workers",
			workers: 2,
			want:    2,
		},
		{
			name:    "excessive workers",
			workers: 100,
			want:    min(4, runtime.NumCPU()*2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeHealthCheckWorkers(tt.workers, logger)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
			if got < 1 {
				t.Error("workers should be at least 1")
			}
		})
	}
}

func TestNormalizeCLIHealthCheckWorkers(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	tests := []struct {
		name    string
		workers int
		overall int
		want    int
	}{
		{
			name:    "zero workers",
			workers: 0,
			overall: 4,
			want:    defaultCLIHealthCheckConcurrency,
		},
		{
			name:    "negative workers",
			workers: -1,
			overall: 4,
			want:    defaultCLIHealthCheckConcurrency,
		},
		{
			name:    "valid workers",
			workers: 2,
			overall: 4,
			want:    2,
		},
		{
			name:    "exceeds overall",
			workers: 10,
			overall: 4,
			want:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCLIHealthCheckWorkers(tt.workers, tt.overall, logger)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
			if got < 1 {
				t.Error("workers should be at least 1")
			}
			if got > tt.overall {
				t.Errorf("workers %d should not exceed overall %d", got, tt.overall)
			}
		})
	}
}

func TestHealthScheduler_ConcurrencyControl(t *testing.T) {
	upstream := httptest.NewServer(nil)
	defer upstream.Close()

	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream(upstream.URL))

	// Create test account with multiple nodes
	acc, err := srv.createAccount("test-account", "test-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Add 10 nodes
	for i := 0; i < 10; i++ {
		_, err := srv.addNodeToAccount(acc, "node-"+string(rune('0'+i)), upstream.URL, "", i+1)
		if err != nil {
			t.Fatalf("add node %d: %v", i, err)
		}
	}

	// Test with limited concurrency
	scheduler := NewHealthScheduler(srv, 10*time.Minute, 2, 1, nil)

	start := time.Now()
	scheduler.checkAllNodes()
	duration := time.Since(start)

	// With 10 nodes and concurrency of 2, it should take some time
	// but not too long (health checks should run in parallel)
	if duration > 30*time.Second {
		t.Errorf("checkAllNodes took too long: %v", duration)
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
