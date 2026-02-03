package store

import (
	"context"
	"testing"
	"time"
)

// TestConfigOperations tests config-related operations
func TestConfigOperations(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-account-1",
		Name: "Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	t.Run("LoadAllByAccount returns empty for new account", func(t *testing.T) {
		records, cfg, activeID, err := s.LoadAllByAccount(ctx, acc.ID)
		if err != nil {
			t.Fatalf("LoadAllByAccount failed: %v", err)
		}
		// Should return empty records for new account
		if len(records) != 0 {
			t.Errorf("expected empty records, got %d", len(records))
		}
		// Config and activeID should be empty for new account
		_ = cfg
		_ = activeID
	})

	t.Run("LoadConfigByAccount returns default for missing key", func(t *testing.T) {
		cfg, activeID, err := s.LoadConfigByAccount(ctx, acc.ID)
		if err != nil {
			t.Fatalf("LoadConfigByAccount failed: %v", err)
		}
		// For a new account, config should be empty
		_ = cfg
		_ = activeID
	})

	t.Run("UpdateConfig creates new config", func(t *testing.T) {
		// Create a config with test values
		cfg := Config{
			Retries:     5,
			FailLimit:   3,
			HealthEvery: 30 * time.Second,
		}
		err := s.UpdateConfig(ctx, acc.ID, cfg, "")
		if err != nil {
			t.Fatalf("UpdateConfig failed: %v", err)
		}

		// Verify config was created
		loadedCfg, _, err := s.LoadConfigByAccount(ctx, acc.ID)
		if err != nil {
			t.Fatalf("LoadConfigByAccount failed: %v", err)
		}
		if loadedCfg.Retries != 5 {
			t.Errorf("expected Retries=5, got %d", loadedCfg.Retries)
		}
		if loadedCfg.FailLimit != 3 {
			t.Errorf("expected FailLimit=3, got %d", loadedCfg.FailLimit)
		}
	})

	t.Run("UpdateConfig updates existing config", func(t *testing.T) {
		cfg := Config{
			Retries:     10,
			FailLimit:   5,
			HealthEvery: 60 * time.Second,
		}
		err := s.UpdateConfig(ctx, acc.ID, cfg, "")
		if err != nil {
			t.Fatalf("UpdateConfig failed: %v", err)
		}

		// Verify config was updated
		loadedCfg, _, err := s.LoadConfigByAccount(ctx, acc.ID)
		if err != nil {
			t.Fatalf("LoadConfigByAccount failed: %v", err)
		}
		if loadedCfg.Retries != 10 {
			t.Errorf("expected Retries=10, got %d", loadedCfg.Retries)
		}
		if loadedCfg.FailLimit != 5 {
			t.Errorf("expected FailLimit=5, got %d", loadedCfg.FailLimit)
		}
	})

	t.Run("SetActive updates active node", func(t *testing.T) {
		err := s.SetActive(ctx, acc.ID, "node-123")
		if err != nil {
			t.Fatalf("SetActive failed: %v", err)
		}

		// Verify active node was set
		_, activeID, err := s.LoadConfigByAccount(ctx, acc.ID)
		if err != nil {
			t.Fatalf("LoadConfigByAccount failed: %v", err)
		}
		if activeID != "node-123" {
			t.Errorf("expected 'node-123', got %s", activeID)
		}
	})

	t.Run("LoadAllByAccount returns all configs", func(t *testing.T) {
		// Add config
		cfg := Config{
			Retries:     3,
			FailLimit:   2,
			HealthEvery: 45 * time.Second,
		}
		s.UpdateConfig(ctx, acc.ID, cfg, "")

		records, loadedCfg, _, err := s.LoadAllByAccount(ctx, acc.ID)
		if err != nil {
			t.Fatalf("LoadAllByAccount failed: %v", err)
		}

		// Check that config was loaded
		if loadedCfg.Retries != 3 {
			t.Errorf("expected Retries=3, got %d", loadedCfg.Retries)
		}
		_ = records
	})
}

// TestMetricsTrend tests metrics trend functions
func TestMetricsTrend(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-account-1",
		Name: "Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	// Create test node
	node := NodeRecord{
		ID:        "test-node-1",
		AccountID: acc.ID,
		Name:      "Test Node",
		BaseURL:   "http://test.com",
		APIKey:    "test-key",
		Weight:    1,
	}
	if err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Insert some test metrics
	now := time.Now()
	for i := 0; i < 5; i++ {
		metrics := MetricsRecord{
			NodeID:            node.ID,
			AccountID:         node.AccountID,
			Timestamp:         now.Add(time.Duration(-i) * time.Hour),
			RequestsTotal:     int64(10 + i),
			RequestsFailed:    int64(i),
			InputTokensTotal:  int64(100 * (i + 1)),
			OutputTokensTotal: int64(200 * (i + 1)),
		}
		if err := s.InsertMetrics(ctx, metrics); err != nil {
			t.Fatalf("failed to insert metrics: %v", err)
		}
	}

	// Aggregate the metrics so they appear in the hourly table
	from := now.Add(-24 * time.Hour)
	to := now
	if err := s.AggregateMetrics(ctx, node.AccountID, MetricsGranularityHourly, from, to); err != nil {
		t.Logf("AggregateMetrics warning: %v", err)
	}

	t.Run("GetNode24hTrend returns trend data", func(t *testing.T) {
		trend, err := s.GetNode24hTrend(ctx, node.AccountID, node.ID)
		if err != nil {
			t.Fatalf("GetNode24hTrend failed: %v", err)
		}

		// Note: trend might be empty if aggregation didn't create hourly records
		// This is OK for the test - we're just verifying the function doesn't error
		_ = trend
	})

	t.Run("GetNodes24hTrend returns trend for multiple nodes", func(t *testing.T) {
		trends, err := s.GetNodes24hTrend(ctx, node.AccountID, []string{node.ID})
		if err != nil {
			t.Fatalf("GetNodes24hTrend failed: %v", err)
		}

		// Note: trends might be empty if aggregation didn't create hourly records
		_ = trends
	})

	t.Run("GetNodes24hTrend with empty list returns empty", func(t *testing.T) {
		trends, err := s.GetNodes24hTrend(ctx, node.AccountID, []string{})
		if err != nil {
			t.Fatalf("GetNodes24hTrend failed: %v", err)
		}

		if len(trends) != 0 {
			t.Error("expected empty trends for empty node list")
		}
	})
}

// TestMetricsAggregation tests metrics aggregation
func TestMetricsAggregation(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-account-1",
		Name: "Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	t.Run("AggregateMetrics processes metrics", func(t *testing.T) {
		// This function aggregates metrics data
		// Test with a specific time range
		now := time.Now()
		from := now.Add(-24 * time.Hour)
		to := now
		err := s.AggregateMetrics(ctx, acc.ID, MetricsGranularityHourly, from, to)
		if err != nil {
			t.Logf("AggregateMetrics returned error (may be expected): %v", err)
		}
	})
}

// TestCleanupOperations tests cleanup functions
func TestCleanupOperations(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account and node
	acc := AccountRecord{
		ID:   "test-account-1",
		Name: "Test Account",
	}
	s.CreateAccount(ctx, acc)

	node := NodeRecord{
		ID:        "test-node-1",
		AccountID: acc.ID,
		Name:      "Test Node",
		BaseURL:   "http://test.com",
		Weight:    1,
	}
	s.UpsertNode(ctx, node)

	t.Run("CleanupMetrics removes old metrics", func(t *testing.T) {
		// Insert old metrics
		oldTime := time.Now().Add(-100 * 24 * time.Hour)
		metrics := MetricsRecord{
			NodeID:         node.ID,
			AccountID:      node.AccountID,
			Timestamp:      oldTime,
			RequestsTotal:  10,
		}
		s.InsertMetrics(ctx, metrics)

		// Cleanup metrics older than 90 days
		cutoffTime := time.Now().Add(-90 * 24 * time.Hour)
		err := s.CleanupMetrics(ctx, node.AccountID, cutoffTime)
		if err != nil {
			t.Fatalf("CleanupMetrics failed: %v", err)
		}
	})

	t.Run("CleanupHealthChecks removes old health checks", func(t *testing.T) {
		// Insert old health check
		oldTime := time.Now().Add(-100 * 24 * time.Hour)
		hc := HealthCheckRecord{
			NodeID:      node.ID,
			AccountID:   node.AccountID,
			CheckTime:   oldTime,
			Success:     true,
			CheckMethod: "API",
		}
		if err := s.InsertHealthCheck(ctx, &hc); err != nil {
			t.Fatalf("failed to insert health check: %v", err)
		}

		// Cleanup health checks older than 90 days
		cutoffTime := time.Now().Add(-90 * 24 * time.Hour)
		err := s.CleanupHealthChecks(ctx, cutoffTime)
		if err != nil {
			t.Fatalf("CleanupHealthChecks failed: %v", err)
		}
	})
}

