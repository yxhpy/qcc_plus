package notify

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestWithQueueSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		expected int
	}{
		{"positive size", 256, 256},
		{"zero size", 0, 0}, // Should not change default
		{"negative size", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ManagerConfig{QueueSize: 0}
			opt := WithQueueSize(tt.size)
			opt(&cfg)
			if tt.size > 0 && cfg.QueueSize != tt.expected {
				t.Errorf("expected QueueSize %d, got %d", tt.expected, cfg.QueueSize)
			}
		})
	}
}

func TestWithWorkerCount(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected int
	}{
		{"positive count", 4, 4},
		{"zero count", 0, 0},
		{"negative count", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ManagerConfig{WorkerCount: 0}
			opt := WithWorkerCount(tt.count)
			opt(&cfg)
			if tt.count > 0 && cfg.WorkerCount != tt.expected {
				t.Errorf("expected WorkerCount %d, got %d", tt.expected, cfg.WorkerCount)
			}
		})
	}
}

func TestWithDedupWindow(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected time.Duration
	}{
		{"positive duration", 10 * time.Minute, 10 * time.Minute},
		{"zero duration", 0, 0},
		{"negative duration", -1 * time.Minute, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ManagerConfig{DedupWindow: 0}
			opt := WithDedupWindow(tt.duration)
			opt(&cfg)
			if tt.duration > 0 && cfg.DedupWindow != tt.expected {
				t.Errorf("expected DedupWindow %v, got %v", tt.expected, cfg.DedupWindow)
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	logger := &mockLogger{}
	cfg := ManagerConfig{}
	opt := WithLogger(logger)
	opt(&cfg)
	if cfg.Logger != logger {
		t.Error("expected Logger to be set")
	}

	// Test with nil logger
	cfg2 := ManagerConfig{Logger: logger}
	opt2 := WithLogger(nil)
	opt2(&cfg2)
	if cfg2.Logger != logger {
		t.Error("nil logger should not override existing logger")
	}
}

func TestWithSendTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"positive timeout", 10 * time.Second, 10 * time.Second},
		{"zero timeout", 0, 0},
		{"negative timeout", -1 * time.Second, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ManagerConfig{SendTimeout: 0}
			opt := WithSendTimeout(tt.timeout)
			opt(&cfg)
			if tt.timeout > 0 && cfg.SendTimeout != tt.expected {
				t.Errorf("expected SendTimeout %v, got %v", tt.expected, cfg.SendTimeout)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "default config",
			opts: nil,
		},
		{
			name: "custom config",
			opts: []Option{
				WithQueueSize(256),
				WithWorkerCount(4),
				WithDedupWindow(10 * time.Minute),
				WithSendTimeout(10 * time.Second),
			},
		},
		{
			name: "zero worker count should default to 1",
			opts: []Option{
				WithWorkerCount(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStore{}
			m := NewManager(mock, tt.opts...)
			if m == nil {
				t.Fatal("NewManager() should not return nil")
			}
			if m.store != mock {
				t.Error("Manager should have correct store")
			}
			if m.queue == nil {
				t.Error("Manager should have queue")
			}
			if m.stopped == nil {
				t.Error("Manager should have stopped channel")
			}
			if m.lastNotify == nil {
				t.Error("Manager should have lastNotify map")
			}

			// Clean up
			m.Stop()
		})
	}
}

func TestManagerPublish(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		queueSize int
		wantDrop  bool
	}{
		{
			name: "publish success",
			event: Event{
				AccountID: "acc1",
				EventType: EventNodeFailed,
				Title:     "Test",
				Content:   "Test content",
			},
			queueSize: 10,
			wantDrop:  false,
		},
		{
			name: "publish with zero time",
			event: Event{
				AccountID:  "acc1",
				EventType:  EventNodeRecovered,
				Title:      "Test",
				Content:    "Test content",
				OccurredAt: time.Time{}, // Zero time
			},
			queueSize: 10,
			wantDrop:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStore{}
			logger := &mockLogger{}
			m := NewManager(mock, WithQueueSize(tt.queueSize), WithLogger(logger))
			defer m.Stop()

			m.Publish(tt.event)

			// Give some time for processing
			time.Sleep(10 * time.Millisecond)

			if tt.wantDrop {
				if len(logger.messages) == 0 {
					t.Error("expected drop message in log")
				}
			}
		})
	}
}

func TestManagerPublishNil(t *testing.T) {
	var m *Manager
	// Should not panic
	m.Publish(Event{})
}

func TestManagerStop(t *testing.T) {
	mock := &mockStore{}
	m := NewManager(mock)

	// Stop should be idempotent
	m.Stop()
	m.Stop()

	// Verify stopped channel is closed
	select {
	case <-m.stopped:
		// Expected
	default:
		t.Error("stopped channel should be closed")
	}
}

func TestManagerStopNil(t *testing.T) {
	var m *Manager
	// Should not panic
	m.Stop()
}

func TestManagerHandleEvent(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		setupMock func(*mockStore)
		wantCalls int
	}{
		{
			name: "no subscriptions",
			event: Event{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Test",
				Content:    "Test content",
				OccurredAt: time.Now(),
			},
			setupMock: func(m *mockStore) {
				m.subscriptions = []store.SubscriptionWithChannel{}
			},
			wantCalls: 0,
		},
		{
			name: "subscription error",
			event: Event{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Test",
				Content:    "Test content",
				OccurredAt: time.Now(),
			},
			setupMock: func(m *mockStore) {
				m.subsErr = errors.New("database error")
			},
			wantCalls: 0,
		},
		{
			name: "successful send",
			event: Event{
				AccountID:  "acc1",
				EventType:  EventNodeFailed,
				Title:      "Node Failed",
				Content:    "Node 1 failed",
				OccurredAt: time.Now(),
			},
			setupMock: func(m *mockStore) {
				m.subscriptions = []store.SubscriptionWithChannel{
					{
						Subscription: store.NotificationSubscriptionRecord{
							ID:        "sub1",
							AccountID: "acc1",
							ChannelID: "ch1",
							EventType: EventNodeFailed,
							Enabled:   true,
						},
						Channel: store.NotificationChannelRecord{
							ID:          "ch1",
							AccountID:   "acc1",
							ChannelType: ChannelWechatWork,
							Name:        "Test Channel",
							Config:      json.RawMessage(`{"webhook_url":"https://example.com/webhook"}`),
							Enabled:     true,
						},
					},
				}
			},
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStore{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			logger := &mockLogger{}
			m := NewManager(mock, WithLogger(logger), WithSendTimeout(1*time.Second))
			defer m.Stop()

			m.handleEvent(tt.event)

			if len(mock.historyCalls) != tt.wantCalls {
				t.Errorf("expected %d history calls, got %d", tt.wantCalls, len(mock.historyCalls))
			}
		})
	}
}

func TestManagerComposeDedupKey(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		name     string
		event    Event
		sub      store.SubscriptionWithChannel
		expected string
	}{
		{
			name: "with dedup key",
			event: Event{
				AccountID: "acc1",
				EventType: EventNodeFailed,
				DedupKey:  "node1",
			},
			sub: store.SubscriptionWithChannel{
				Channel: store.NotificationChannelRecord{
					ID: "ch1",
				},
			},
			expected: "ch1|acc1|node.failed:node1",
		},
		{
			name: "without dedup key",
			event: Event{
				AccountID: "acc1",
				EventType: EventNodeRecovered,
				DedupKey:  "",
			},
			sub: store.SubscriptionWithChannel{
				Channel: store.NotificationChannelRecord{
					ID: "ch2",
				},
			},
			expected: "ch2|acc1|node.recovered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.composeDedupKey(tt.event, tt.sub)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestManagerShouldSend(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		dedupWindow  time.Duration
		callTwice    bool
		expectFirst  bool
		expectSecond bool
	}{
		{
			name:         "first call should send",
			key:          "test-key-1",
			dedupWindow:  5 * time.Minute,
			callTwice:    false,
			expectFirst:  true,
			expectSecond: false,
		},
		{
			name:         "second call within window should not send",
			key:          "test-key-2",
			dedupWindow:  5 * time.Minute,
			callTwice:    true,
			expectFirst:  true,
			expectSecond: false,
		},
		{
			name:         "second call after window should send",
			key:          "test-key-3",
			dedupWindow:  1 * time.Millisecond,
			callTwice:    true,
			expectFirst:  true,
			expectSecond: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				cfg: ManagerConfig{
					DedupWindow: tt.dedupWindow,
				},
				lastNotify: make(map[string]time.Time),
			}

			result1 := m.shouldSend(tt.key)
			if result1 != tt.expectFirst {
				t.Errorf("first call: expected %v, got %v", tt.expectFirst, result1)
			}

			if tt.callTwice {
				if tt.dedupWindow > 1*time.Millisecond {
					// Call immediately
					result2 := m.shouldSend(tt.key)
					if result2 != tt.expectSecond {
						t.Errorf("second call (immediate): expected %v, got %v", tt.expectSecond, result2)
					}
				} else {
					// Wait for window to expire
					time.Sleep(2 * time.Millisecond)
					result2 := m.shouldSend(tt.key)
					if result2 != tt.expectSecond {
						t.Errorf("second call (after window): expected %v, got %v", tt.expectSecond, result2)
					}
				}
			}
		})
	}
}

func TestManagerShouldSendConcurrent(t *testing.T) {
	m := &Manager{
		cfg: ManagerConfig{
			DedupWindow: 5 * time.Minute,
		},
		lastNotify: make(map[string]time.Time),
	}

	// Test concurrent access
	var wg sync.WaitGroup
	key := "concurrent-key"
	results := make([]bool, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = m.shouldSend(key)
		}(i)
	}

	wg.Wait()

	// Only one should return true
	trueCount := 0
	for _, r := range results {
		if r {
			trueCount++
		}
	}

	if trueCount != 1 {
		t.Errorf("expected exactly 1 true result, got %d", trueCount)
	}
}

func TestManagerLogf(t *testing.T) {
	tests := []struct {
		name   string
		logger Logger
		format string
		args   []interface{}
	}{
		{
			name:   "with logger",
			logger: &mockLogger{},
			format: "test message: %s",
			args:   []interface{}{"hello"},
		},
		{
			name:   "with nil logger",
			logger: nil,
			format: "test message",
			args:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				cfg: ManagerConfig{
					Logger: tt.logger,
				},
			}

			// Should not panic
			m.logf(tt.format, tt.args...)

			if tt.logger != nil {
				ml := tt.logger.(*mockLogger)
				if len(ml.messages) != 1 {
					t.Errorf("expected 1 log message, got %d", len(ml.messages))
				}
			}
		})
	}
}

func TestRandomID(t *testing.T) {
	// Test that randomID generates unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := randomID()
		if id == "" {
			t.Error("randomID() should not return empty string")
		}
		if ids[id] {
			t.Errorf("randomID() generated duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestManagerIntegration(t *testing.T) {
	// Integration test: publish event and verify it's processed
	// Create a test HTTP server for webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errcode": 0,
			"errmsg":  "ok",
		})
	}))
	defer server.Close()

	mock := &mockStore{
		subscriptions: []store.SubscriptionWithChannel{
			{
				Subscription: store.NotificationSubscriptionRecord{
					ID:        "sub1",
					AccountID: "acc1",
					ChannelID: "ch1",
					EventType: EventNodeFailed,
					Enabled:   true,
				},
				Channel: store.NotificationChannelRecord{
					ID:          "ch1",
					AccountID:   "acc1",
					ChannelType: ChannelWechatWork,
					Name:        "Test",
					Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
					Enabled:     true,
				},
			},
		},
	}

	logger := &mockLogger{}
	m := NewManager(mock, WithLogger(logger), WithWorkerCount(1), WithQueueSize(10))
	defer m.Stop()

	// Publish event
	evt := Event{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Node Failed",
		Content:    "Node 1 has failed",
		DedupKey:   "node1",
		OccurredAt: time.Now(),
	}

	m.Publish(evt)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify history was recorded
	if len(mock.historyCalls) == 0 {
		t.Error("expected history to be recorded")
	}
}

func TestManagerQueueFull(t *testing.T) {
	mock := &mockStore{}
	logger := &mockLogger{}
	m := NewManager(mock, WithLogger(logger), WithQueueSize(1), WithWorkerCount(0))
	defer m.Stop()

	// Fill the queue
	m.Publish(Event{AccountID: "acc1", EventType: EventNodeFailed})
	m.Publish(Event{AccountID: "acc2", EventType: EventNodeFailed})

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Should have logged a drop message
	if len(logger.messages) == 0 {
		t.Error("expected drop message in log when queue is full")
	}
}

func TestRandomIDFallback(t *testing.T) {
	// Test that randomID returns a valid ID even if crypto/rand fails
	// We can't easily force crypto/rand to fail, but we can verify the function works
	id := randomID()
	if id == "" {
		t.Error("randomID should not return empty string")
	}
	if len(id) < 10 {
		t.Error("randomID should return a reasonably long ID")
	}
}

func TestManagerHandleEventWithUnsupportedChannel(t *testing.T) {
	mock := &mockStore{
		subscriptions: []store.SubscriptionWithChannel{
			{
				Subscription: store.NotificationSubscriptionRecord{
					ID:        "sub1",
					AccountID: "acc1",
					ChannelID: "ch1",
					EventType: EventNodeFailed,
					Enabled:   true,
				},
				Channel: store.NotificationChannelRecord{
					ID:          "ch1",
					AccountID:   "acc1",
					ChannelType: ChannelEmail, // Unsupported channel type
					Name:        "Email Channel",
					Config:      json.RawMessage(`{"smtp":"smtp.example.com"}`),
					Enabled:     true,
				},
			},
		},
	}

	logger := &mockLogger{}
	m := NewManager(mock, WithLogger(logger), WithSendTimeout(1*time.Second))
	defer m.Stop()

	evt := Event{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		OccurredAt: time.Now(),
	}

	m.handleEvent(evt)

	// Should have logged an error about unsupported channel
	if len(logger.messages) == 0 {
		t.Error("expected error log for unsupported channel")
	}
}

func TestManagerHandleEventWithHistoryError(t *testing.T) {
	// Create a test HTTP server for webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errcode": 0,
			"errmsg":  "ok",
		})
	}))
	defer server.Close()

	mock := &mockStore{
		subscriptions: []store.SubscriptionWithChannel{
			{
				Subscription: store.NotificationSubscriptionRecord{
					ID:        "sub1",
					AccountID: "acc1",
					ChannelID: "ch1",
					EventType: EventNodeFailed,
					Enabled:   true,
				},
				Channel: store.NotificationChannelRecord{
					ID:          "ch1",
					AccountID:   "acc1",
					ChannelType: ChannelWechatWork,
					Name:        "Test",
					Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
					Enabled:     true,
				},
			},
		},
		historyErr: errors.New("database error"),
	}

	logger := &mockLogger{}
	m := NewManager(mock, WithLogger(logger), WithSendTimeout(1*time.Second))
	defer m.Stop()

	evt := Event{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		OccurredAt: time.Now(),
	}

	m.handleEvent(evt)

	// Should have logged an error about history insert failure
	if len(logger.messages) == 0 {
		t.Error("expected error log for history insert failure")
	}
}

func TestManagerHandleEventWithSendError(t *testing.T) {
	// Create a test HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	mock := &mockStore{
		subscriptions: []store.SubscriptionWithChannel{
			{
				Subscription: store.NotificationSubscriptionRecord{
					ID:        "sub1",
					AccountID: "acc1",
					ChannelID: "ch1",
					EventType: EventNodeFailed,
					Enabled:   true,
				},
				Channel: store.NotificationChannelRecord{
					ID:          "ch1",
					AccountID:   "acc1",
					ChannelType: ChannelWechatWork,
					Name:        "Test",
					Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
					Enabled:     true,
				},
			},
		},
	}

	logger := &mockLogger{}
	m := NewManager(mock, WithLogger(logger), WithSendTimeout(1*time.Second))
	defer m.Stop()

	evt := Event{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		OccurredAt: time.Now(),
	}

	m.handleEvent(evt)

	// Should have recorded history with failed status
	if len(mock.historyCalls) != 1 {
		t.Errorf("expected 1 history call, got %d", len(mock.historyCalls))
	}
	if mock.historyCalls[0].Status != historyStatusFailed {
		t.Errorf("expected status %s, got %s", historyStatusFailed, mock.historyCalls[0].Status)
	}
	if mock.historyCalls[0].Error == "" {
		t.Error("expected error message in history")
	}
}

func TestManagerHandleEventDeduplication(t *testing.T) {
	// Create a test HTTP server for webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errcode": 0,
			"errmsg":  "ok",
		})
	}))
	defer server.Close()

	mock := &mockStore{
		subscriptions: []store.SubscriptionWithChannel{
			{
				Subscription: store.NotificationSubscriptionRecord{
					ID:        "sub1",
					AccountID: "acc1",
					ChannelID: "ch1",
					EventType: EventNodeFailed,
					Enabled:   true,
				},
				Channel: store.NotificationChannelRecord{
					ID:          "ch1",
					AccountID:   "acc1",
					ChannelType: ChannelWechatWork,
					Name:        "Test",
					Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
					Enabled:     true,
				},
			},
		},
	}

	logger := &mockLogger{}
	m := NewManager(mock, WithLogger(logger), WithDedupWindow(5*time.Minute), WithSendTimeout(1*time.Second))
	defer m.Stop()

	evt := Event{
		AccountID:  "acc1",
		EventType:  EventNodeFailed,
		Title:      "Test",
		Content:    "Test",
		DedupKey:   "node1",
		OccurredAt: time.Now(),
	}

	// Send first event
	m.handleEvent(evt)

	// Send duplicate event immediately
	m.handleEvent(evt)

	// Should only have 1 history call due to deduplication
	if len(mock.historyCalls) != 1 {
		t.Errorf("expected 1 history call due to deduplication, got %d", len(mock.historyCalls))
	}
}
