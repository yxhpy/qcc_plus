package store

import (
	"context"
	"testing"
	"time"
)

func TestInsertHealthCheck(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Setup
	acc := AccountRecord{ID: "test-acc", Name: "Test Account"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	node := NodeRecord{
		ID:        "test-node",
		AccountID: "test-acc",
		Name:      "Test Node",
		BaseURL:   "http://test.com",
		APIKey:    "test-key",
	}
	if err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("insert health check with all fields", func(t *testing.T) {
		now := time.Now().UTC()
		record := &HealthCheckRecord{
			AccountID:      "test-acc",
			NodeID:         "test-node",
			CheckTime:      now,
			Success:        true,
			ResponseTimeMs: 150,
			ErrorMessage:   "",
			CheckMethod:    "api",
			CheckSource:    "scheduled",
			CreatedAt:      now,
		}

		err := s.InsertHealthCheck(ctx, record)
		if err != nil {
			t.Fatalf("InsertHealthCheck failed: %v", err)
		}

		// Query back to verify
		latest, err := s.LatestHealthCheck(ctx, "test-acc", "test-node")
		if err != nil {
			t.Fatalf("LatestHealthCheck failed: %v", err)
		}

		if latest == nil {
			t.Fatal("LatestHealthCheck returned nil")
		}

		if !latest.Success {
			t.Error("Success should be true")
		}
		if latest.ResponseTimeMs != 150 {
			t.Errorf("ResponseTimeMs = %d, want 150", latest.ResponseTimeMs)
		}
		if latest.CheckMethod != "api" {
			t.Errorf("CheckMethod = %s, want api", latest.CheckMethod)
		}
	})

	t.Run("insert health check with default values", func(t *testing.T) {
		record := &HealthCheckRecord{
			AccountID: "test-acc",
			NodeID:    "test-node",
			Success:   false,
			// CheckTime, CheckMethod, CheckSource, CreatedAt not set
		}

		before := time.Now().UTC()
		err := s.InsertHealthCheck(ctx, record)
		after := time.Now().UTC()

		if err != nil {
			t.Fatalf("InsertHealthCheck failed: %v", err)
		}

		// Query back
		latest, err := s.LatestHealthCheck(ctx, "test-acc", "test-node")
		if err != nil {
			t.Fatalf("LatestHealthCheck failed: %v", err)
		}

		// Check default values
		if latest.CheckMethod != "api" {
			t.Errorf("Default CheckMethod should be 'api', got %s", latest.CheckMethod)
		}
		if latest.CheckSource != "scheduled" {
			t.Errorf("Default CheckSource should be 'scheduled', got %s", latest.CheckSource)
		}
		if latest.CheckTime.Before(before) || latest.CheckTime.After(after) {
			t.Errorf("CheckTime not in expected range: %v", latest.CheckTime)
		}
	})

	t.Run("insert health check with failure", func(t *testing.T) {
		record := &HealthCheckRecord{
			AccountID:      "test-acc",
			NodeID:         "test-node",
			CheckTime:      time.Now().UTC(),
			Success:        false,
			ResponseTimeMs: 0,
			ErrorMessage:   "connection timeout",
			CheckMethod:    "head",
			CheckSource:    "manual",
		}

		err := s.InsertHealthCheck(ctx, record)
		if err != nil {
			t.Fatalf("InsertHealthCheck failed: %v", err)
		}

		// Query back
		latest, err := s.LatestHealthCheck(ctx, "test-acc", "test-node")
		if err != nil {
			t.Fatalf("LatestHealthCheck failed: %v", err)
		}

		if latest.Success {
			t.Error("Success should be false")
		}
		if latest.ErrorMessage != "connection timeout" {
			t.Errorf("ErrorMessage = %s, want 'connection timeout'", latest.ErrorMessage)
		}
		if latest.CheckMethod != "head" {
			t.Errorf("CheckMethod = %s, want head", latest.CheckMethod)
		}
	})

	t.Run("insert health check with nil record fails", func(t *testing.T) {
		err := s.InsertHealthCheck(ctx, nil)
		if err == nil {
			t.Fatal("Expected error for nil record, got nil")
		}
	})
}

func TestLatestHealthCheck(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Setup
	acc := AccountRecord{ID: "test-acc", Name: "Test Account"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	node := NodeRecord{
		ID:        "test-node",
		AccountID: "test-acc",
		Name:      "Test Node",
		BaseURL:   "http://test.com",
		APIKey:    "test-key",
	}
	if err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("returns nil when no health checks exist", func(t *testing.T) {
		latest, err := s.LatestHealthCheck(ctx, "test-acc", "nonexistent-node")
		if err != nil {
			t.Fatalf("LatestHealthCheck failed: %v", err)
		}

		if latest != nil {
			t.Error("Expected nil for nonexistent node")
		}
	})

	t.Run("returns latest health check", func(t *testing.T) {
		// Insert multiple health checks
		now := time.Now().UTC()
		records := []*HealthCheckRecord{
			{
				AccountID:      "test-acc",
				NodeID:         "test-node",
				CheckTime:      now.Add(-2 * time.Hour),
				Success:        true,
				ResponseTimeMs: 100,
			},
			{
				AccountID:      "test-acc",
				NodeID:         "test-node",
				CheckTime:      now.Add(-1 * time.Hour),
				Success:        false,
				ResponseTimeMs: 200,
				ErrorMessage:   "timeout",
			},
			{
				AccountID:      "test-acc",
				NodeID:         "test-node",
				CheckTime:      now,
				Success:        true,
				ResponseTimeMs: 150,
			},
		}

		for _, rec := range records {
			if err := s.InsertHealthCheck(ctx, rec); err != nil {
				t.Fatalf("InsertHealthCheck failed: %v", err)
			}
		}

		// Get latest
		latest, err := s.LatestHealthCheck(ctx, "test-acc", "test-node")
		if err != nil {
			t.Fatalf("LatestHealthCheck failed: %v", err)
		}

		if latest == nil {
			t.Fatal("LatestHealthCheck returned nil")
		}

		// Should be the most recent one
		if latest.ResponseTimeMs != 150 {
			t.Errorf("Expected latest ResponseTimeMs 150, got %d", latest.ResponseTimeMs)
		}
		if !latest.Success {
			t.Error("Latest check should be successful")
		}
	})

	t.Run("fails with empty node_id", func(t *testing.T) {
		_, err := s.LatestHealthCheck(ctx, "test-acc", "")
		if err == nil {
			t.Fatal("Expected error for empty node_id, got nil")
		}
	})
}

func TestQueryHealthChecks(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Setup
	acc := AccountRecord{ID: "test-acc", Name: "Test Account"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	node1 := NodeRecord{ID: "node-1", AccountID: "test-acc", Name: "Node 1", BaseURL: "http://1.com", APIKey: "key1"}
	node2 := NodeRecord{ID: "node-2", AccountID: "test-acc", Name: "Node 2", BaseURL: "http://2.com", APIKey: "key2"}
	if err := s.UpsertNode(ctx, node1); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.UpsertNode(ctx, node2); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Insert test data
	now := time.Now().UTC()
	records := []*HealthCheckRecord{
		{
			AccountID:      "test-acc",
			NodeID:         "node-1",
			CheckTime:      now.Add(-2 * time.Hour),
			Success:        true,
			ResponseTimeMs: 100,
			CheckSource:    "scheduled",
		},
		{
			AccountID:      "test-acc",
			NodeID:         "node-1",
			CheckTime:      now.Add(-1 * time.Hour),
			Success:        false,
			ResponseTimeMs: 200,
			ErrorMessage:   "timeout",
			CheckSource:    "manual",
		},
		{
			AccountID:      "test-acc",
			NodeID:         "node-2",
			CheckTime:      now.Add(-1 * time.Hour),
			Success:        true,
			ResponseTimeMs: 150,
			CheckSource:    "scheduled",
		},
	}

	for _, rec := range records {
		if err := s.InsertHealthCheck(ctx, rec); err != nil {
			t.Fatalf("InsertHealthCheck failed: %v", err)
		}
	}

	t.Run("query all health checks for account", func(t *testing.T) {
		params := QueryHealthCheckParams{
			AccountID: "test-acc",
			NodeID:    "node-1", // node_id is required
			From:      now.Add(-3 * time.Hour),
			To:        now,
		}

		results, err := s.QueryHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("QueryHealthChecks failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results for node-1, got %d", len(results))
		}
	})

	t.Run("query health checks for specific node", func(t *testing.T) {
		params := QueryHealthCheckParams{
			AccountID: "test-acc",
			NodeID:    "node-1",
			From:      now.Add(-3 * time.Hour),
			To:        now,
		}

		results, err := s.QueryHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("QueryHealthChecks failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results for node-1, got %d", len(results))
		}

		for _, r := range results {
			if r.NodeID != "node-1" {
				t.Errorf("Expected NodeID node-1, got %s", r.NodeID)
			}
		}
	})

	t.Run("query with time range filter", func(t *testing.T) {
		params := QueryHealthCheckParams{
			AccountID: "test-acc",
			NodeID:    "node-1",
			From:      now.Add(-90 * time.Minute),
			To:        now.Add(-30 * time.Minute),
		}

		results, err := s.QueryHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("QueryHealthChecks failed: %v", err)
		}

		// Should only get records from -1 hour for node-1
		if len(results) != 1 {
			t.Errorf("Expected 1 result in time range, got %d", len(results))
		}
	})

	t.Run("query with check source filter", func(t *testing.T) {
		params := QueryHealthCheckParams{
			AccountID:   "test-acc",
			NodeID:      "node-1",
			From:        now.Add(-3 * time.Hour),
			To:          now,
			CheckSource: "scheduled",
		}

		results, err := s.QueryHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("QueryHealthChecks failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 scheduled check for node-1, got %d", len(results))
		}

		for _, r := range results {
			if r.CheckSource != "scheduled" {
				t.Errorf("Expected CheckSource scheduled, got %s", r.CheckSource)
			}
		}
	})

	t.Run("query with limit", func(t *testing.T) {
		params := QueryHealthCheckParams{
			AccountID: "test-acc",
			NodeID:    "node-1",
			From:      now.Add(-3 * time.Hour),
			To:        now,
			Limit:     1,
		}

		results, err := s.QueryHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("QueryHealthChecks failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result with limit, got %d", len(results))
		}
	})
}

func TestCountHealthChecks(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Setup
	acc := AccountRecord{ID: "test-acc", Name: "Test Account"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	node := NodeRecord{ID: "test-node", AccountID: "test-acc", Name: "Test Node", BaseURL: "http://test.com", APIKey: "key"}
	if err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("count returns 0 for no records", func(t *testing.T) {
		params := QueryHealthCheckParams{
			AccountID: "test-acc",
			NodeID:    "test-node",
		}
		count, err := s.CountHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("CountHealthChecks failed: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("count returns correct number", func(t *testing.T) {
		// Insert test records
		now := time.Now().UTC()
		for i := 0; i < 5; i++ {
			record := &HealthCheckRecord{
				AccountID: "test-acc",
				NodeID:    "test-node",
				CheckTime: now.Add(time.Duration(-i) * time.Hour),
				Success:   true,
			}
			if err := s.InsertHealthCheck(ctx, record); err != nil {
				t.Fatalf("InsertHealthCheck failed: %v", err)
			}
		}

		params := QueryHealthCheckParams{
			AccountID: "test-acc",
			NodeID:    "test-node",
		}
		count, err := s.CountHealthChecks(ctx, params)
		if err != nil {
			t.Fatalf("CountHealthChecks failed: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected count 5, got %d", count)
		}
	})
}
