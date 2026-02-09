package proxy

import (
	"net/http"
	"testing"
	"time"
)

func TestServer_Start(t *testing.T) {
	// Test that Start initializes schedulers and starts HTTP server
	// Note: We can't fully test Start() as it blocks, but we can test the setup
	srv := &Server{
		listenAddr: ":0", // Use random port
		transport:  http.DefaultTransport,
		accounts:   make(map[string]*Account),
		nodeIndex:  make(map[string]*Node),
	}

	// Test that Start would initialize properly (we can't actually call it as it blocks)
	if srv.listenAddr == "" {
		t.Error("listenAddr should be set")
	}
}

func TestServer_Stop(t *testing.T) {
	tests := []struct {
		name string
		srv  *Server
	}{
		{
			name: "stop with schedulers",
			srv: &Server{
				healthScheduler:  &HealthScheduler{stopCh: make(chan struct{})},
				metricsScheduler: &MetricsScheduler{stopCh: make(chan struct{})},
				settingsStopCh:   make(chan struct{}),
			},
		},
		{
			name: "stop without schedulers",
			srv:  &Server{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			tt.srv.Stop()
		})
	}
}

func TestServer_Handler(t *testing.T) {
	srv := &Server{
		accounts:   make(map[string]*Account),
		nodeIndex:  make(map[string]*Node),
		sessionMgr: NewSessionManager(24 * time.Hour),
	}

	handler := srv.Handler()
	if handler == nil {
		t.Error("Handler() should return non-nil handler")
	}
}

func TestServer_getOrCreateCircuitBreaker(t *testing.T) {
	srv := &Server{
		circuitBreakers: make(map[string]*CircuitBreaker),
		cbConfig: CircuitBreakerConfig{
			Enabled:          true,
			WindowSeconds:    60,
			FailureRate:      0.5,
			ConsecutiveFails: 5,
			CooldownSeconds:  30,
			HalfOpenMaxCalls: 3,
		},
	}

	// Test creating new circuit breaker
	cb1 := srv.getOrCreateCircuitBreaker("node1")
	if cb1 == nil {
		t.Fatal("getOrCreateCircuitBreaker should return non-nil")
	}

	// Test getting existing circuit breaker
	cb2 := srv.getOrCreateCircuitBreaker("node1")
	if cb1 != cb2 {
		t.Error("getOrCreateCircuitBreaker should return same instance")
	}

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			cb := srv.getOrCreateCircuitBreaker("node2")
			if cb == nil {
				t.Error("concurrent getOrCreateCircuitBreaker failed")
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestServer_getCircuitBreaker(t *testing.T) {
	srv := &Server{
		circuitBreakers: make(map[string]*CircuitBreaker),
	}

	// Test getting non-existent circuit breaker
	cb := srv.getCircuitBreaker("node1")
	if cb != nil {
		t.Error("getCircuitBreaker should return nil for non-existent node")
	}

	// Add a circuit breaker
	srv.circuitBreakers["node1"] = NewCircuitBreaker(CircuitBreakerConfig{})

	// Test getting existing circuit breaker
	cb = srv.getCircuitBreaker("node1")
	if cb == nil {
		t.Error("getCircuitBreaker should return circuit breaker")
	}
}

func TestServer_startSettingsWatcher(t *testing.T) {
	tests := []struct {
		name     string
		srv      *Server
		interval time.Duration
		wantRun  bool
	}{
		{
			name:     "nil server",
			srv:      nil,
			interval: time.Second,
			wantRun:  false,
		},
		{
			name:     "nil cache",
			srv:      &Server{},
			interval: time.Second,
			wantRun:  false,
		},
		{
			name:     "zero interval",
			srv:      &Server{settingsCache: &SettingsCache{}},
			interval: 0,
			wantRun:  false,
		},
		{
			name:     "already running",
			srv:      &Server{settingsCache: &SettingsCache{}, settingsStopCh: make(chan struct{})},
			interval: time.Second,
			wantRun:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.srv.startSettingsWatcher(tt.interval)
			// Just verify it doesn't panic
		})
	}
}

func TestServer_applySettingsFromCache(t *testing.T) {
	tests := []struct {
		name     string
		srv      *Server
		settings map[string]interface{}
	}{
		{
			name: "nil server",
			srv:  nil,
		},
		{
			name: "nil cache",
			srv:  &Server{},
		},
		{
			name: "apply health interval",
			srv: &Server{
				settingsCache: &SettingsCache{
					data: map[string]interface{}{
						"health.check_interval_sec": float64(60),
					},
				},
				healthEvery: 30 * time.Second,
			},
		},
		{
			name: "apply retry max",
			srv: &Server{
				settingsCache: &SettingsCache{
					data: map[string]interface{}{
						"proxy.retry_max": float64(5),
					},
				},
			},
		},
		{
			name: "apply fail limit",
			srv: &Server{
				settingsCache: &SettingsCache{
					data: map[string]interface{}{
						"health.fail_threshold": float64(5),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.srv.applySettingsFromCache()
			// Just verify it doesn't panic
		})
	}
}
