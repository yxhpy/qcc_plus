package proxy

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestNewMetricsScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	t.Run("with logger", func(t *testing.T) {
		logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
		scheduler := NewMetricsScheduler(st, logger)
		if scheduler == nil {
			t.Fatal("expected non-nil scheduler")
		}
		if scheduler.store != st {
			t.Error("store not set correctly")
		}
		if scheduler.logger != logger {
			t.Error("logger not set correctly")
		}
		if scheduler.aggregateInterval != defaultAggregateInterval {
			t.Errorf("aggregateInterval = %v, want %v", scheduler.aggregateInterval, defaultAggregateInterval)
		}
		if scheduler.cleanupInterval != defaultCleanupInterval {
			t.Errorf("cleanupInterval = %v, want %v", scheduler.cleanupInterval, defaultCleanupInterval)
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		scheduler := NewMetricsScheduler(st, nil)
		if scheduler == nil {
			t.Fatal("expected non-nil scheduler")
		}
		if scheduler.logger == nil {
			t.Error("expected default logger")
		}
	})
}

func TestMetricsScheduler_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.aggregateInterval = 100 * time.Millisecond
	scheduler.cleanupInterval = 100 * time.Millisecond

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

func TestMetricsScheduler_StartNilStore(t *testing.T) {
	scheduler := NewMetricsScheduler(nil, nil)
	if err := scheduler.Start(); err != nil {
		t.Errorf("start with nil store should not error: %v", err)
	}
}

func TestMetricsScheduler_StopNil(t *testing.T) {
	var scheduler *MetricsScheduler
	scheduler.Stop() // Should not panic
}

func TestMetricsScheduler_StopMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.aggregateInterval = 100 * time.Millisecond
	scheduler.cleanupInterval = 100 * time.Millisecond

	if err := scheduler.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Stop multiple times should be safe
	scheduler.Stop()
	scheduler.Stop()
	scheduler.Stop()
}

func TestMetricsScheduler_RunAggregation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	// Insert some test data
	ctx := context.Background()
	now := time.Now().UTC()
	err = st.InsertMetrics(ctx, store.MetricsRecord{
		AccountID:         "test-account",
		NodeID:            "test-node",
		Timestamp:         now.Add(-2 * time.Hour),
		RequestsTotal:     10,
		RequestsSuccess:   9,
		RequestsFailed:    1,
		ResponseTimeSumMs: 1000,
		ResponseTimeCount: 10,
	})
	if err != nil {
		t.Fatalf("record metrics: %v", err)
	}

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.runAggregation()

	// Verify aggregation ran (no errors expected)
	// The actual aggregation logic is tested in store tests
}

func TestMetricsScheduler_RunCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.runCleanup()

	// Verify cleanup ran (no errors expected)
	// The actual cleanup logic is tested in store tests
}

func TestMetricsScheduler_NextAggregateDelay(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.aggregateInterval = time.Hour

	now := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	delay := scheduler.nextAggregateDelay(now)

	// Should delay until next hour boundary (13:00)
	expected := 30 * time.Minute
	if delay != expected {
		t.Errorf("delay = %v, want %v", delay, expected)
	}
}

func TestMetricsScheduler_NextAggregateDelay_ZeroInterval(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.aggregateInterval = 0

	now := time.Now().UTC()
	delay := scheduler.nextAggregateDelay(now)

	if delay != defaultAggregateInterval {
		t.Errorf("delay = %v, want %v", delay, defaultAggregateInterval)
	}
}

func TestMetricsScheduler_NextCleanupDelay(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.cleanupInterval = 24 * time.Hour

	tests := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{
			name: "before cleanup hour",
			now:  time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
			want: time.Hour, // Next 02:00
		},
		{
			name: "after cleanup hour",
			now:  time.Date(2024, 1, 1, 3, 0, 0, 0, time.UTC),
			want: 23 * time.Hour, // Next day 02:00
		},
		{
			name: "at cleanup hour",
			now:  time.Date(2024, 1, 1, 2, 0, 0, 0, time.UTC),
			want: 24 * time.Hour, // Next day 02:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scheduler.nextCleanupDelay(tt.now)
			if got != tt.want {
				t.Errorf("delay = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricsScheduler_NextCleanupDelay_ShortInterval(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.cleanupInterval = time.Hour

	now := time.Now().UTC()
	delay := scheduler.nextCleanupDelay(now)

	if delay != time.Hour {
		t.Errorf("delay = %v, want %v", delay, time.Hour)
	}
}

func TestMetricsScheduler_NextCleanupDelay_ZeroInterval(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)
	scheduler.cleanupInterval = 0

	now := time.Now().UTC()
	delay := scheduler.nextCleanupDelay(now)

	if delay != defaultCleanupInterval {
		t.Errorf("delay = %v, want %v", delay, defaultCleanupInterval)
	}
}

func TestMetricsScheduler_TaskContext(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)

	t.Run("with timeout", func(t *testing.T) {
		ctx, cancel := scheduler.taskContext(100 * time.Millisecond)
		defer cancel()

		select {
		case <-ctx.Done():
			// Expected timeout
		case <-time.After(200 * time.Millisecond):
			t.Error("context should have timed out")
		}
	})

	t.Run("with zero timeout", func(t *testing.T) {
		ctx, cancel := scheduler.taskContext(0)
		defer cancel()

		select {
		case <-ctx.Done():
			// Expected timeout (default 30s)
		case <-time.After(100 * time.Millisecond):
			// Context should still be valid
		}
	})

	t.Run("cancelled by stop", func(t *testing.T) {
		ctx, cancel := scheduler.taskContext(time.Hour)
		defer cancel()

		close(scheduler.stopCh)

		select {
		case <-ctx.Done():
			// Expected cancellation
		case <-time.After(100 * time.Millisecond):
			t.Error("context should have been cancelled")
		}
	})
}

func TestMetricsScheduler_RecoverPanic(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	st, err := store.OpenSQLite("file:" + tmpDB)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	scheduler := NewMetricsScheduler(st, nil)

	// Should not panic
	func() {
		defer scheduler.recoverPanic("test")
		panic("test panic")
	}()
}

func TestStartOfDay(t *testing.T) {
	tests := []struct {
		input time.Time
		want  time.Time
	}{
		{
			input: time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
			want:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			input: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			want:  time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			input: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			want:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := startOfDay(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("startOfDay(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStartOfMonth(t *testing.T) {
	tests := []struct {
		input time.Time
		want  time.Time
	}{
		{
			input: time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
			want:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			input: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			want:  time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			input: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			want:  time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := startOfMonth(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("startOfMonth(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
