package store

import (
	"context"
	"testing"
	"time"
)

func TestPricingFunctions(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account and node
	acc := AccountRecord{
		ID:   "test-acc-pricing",
		Name: "Pricing Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	node := NodeRecord{
		ID:        "test-node-pricing",
		AccountID: "test-acc-pricing",
		Name:      "Test Node",
		BaseURL:   "https://api.test.com",
		APIKey:    "test-key",
	}
	if err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("seed default pricing", func(t *testing.T) {
		err := s.SeedDefaultPricing(ctx)
		if err != nil {
			t.Fatalf("SeedDefaultPricing failed: %v", err)
		}

		// Verify default pricing was created
		pricing, err := s.ListModelPricing(ctx, true)
		if err != nil {
			t.Fatalf("ListModelPricing failed: %v", err)
		}

		if len(pricing) == 0 {
			t.Error("Expected at least one default pricing model")
		}
	})

	t.Run("get model pricing", func(t *testing.T) {
		// Get one of the seeded models
		pricing, err := s.GetModelPricing(ctx, "claude-sonnet-4-5-20250929")
		if err != nil {
			t.Fatalf("GetModelPricing failed: %v", err)
		}

		if pricing == nil {
			t.Fatal("Expected pricing, got nil")
		}
		if pricing.ModelID != "claude-sonnet-4-5-20250929" {
			t.Errorf("ModelID mismatch: got %s, want %s", pricing.ModelID, "claude-sonnet-4-5-20250929")
		}
	})

	t.Run("get non-existent model pricing returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetModelPricing(ctx, "non-existent-model")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("list model pricing", func(t *testing.T) {
		pricing, err := s.ListModelPricing(ctx, false)
		if err != nil {
			t.Fatalf("ListModelPricing failed: %v", err)
		}

		if len(pricing) == 0 {
			t.Error("Expected at least one pricing model")
		}
	})

	t.Run("list active model pricing only", func(t *testing.T) {
		// Create an inactive pricing
		inactivePricing := ModelPricingRecord{
			ID:              "inactive-model-id",
			ModelID:         "inactive-model",
			ModelName:       "Inactive Model",
			InputPriceMTok:  1.0,
			OutputPriceMTok: 2.0,
			IsActive:        false,
		}
		if err := s.UpsertModelPricing(ctx, inactivePricing); err != nil {
			t.Fatalf("UpsertModelPricing failed: %v", err)
		}

		// List active only
		pricing, err := s.ListModelPricing(ctx, true)
		if err != nil {
			t.Fatalf("ListModelPricing failed: %v", err)
		}

		for _, p := range pricing {
			if !p.IsActive {
				t.Error("Expected only active pricing models")
			}
		}
	})

	t.Run("upsert model pricing", func(t *testing.T) {
		pricing := ModelPricingRecord{
			ID:              "test-pricing-id",
			ModelID:         "test-model",
			ModelName:       "Test Model",
			InputPriceMTok:  3.0,
			OutputPriceMTok: 15.0,
			IsActive:        true,
		}

		err := s.UpsertModelPricing(ctx, pricing)
		if err != nil {
			t.Fatalf("UpsertModelPricing failed: %v", err)
		}

		// Verify creation
		retrieved, err := s.GetModelPricing(ctx, "test-model")
		if err != nil {
			t.Fatalf("GetModelPricing failed: %v", err)
		}

		if retrieved.InputPriceMTok != 3.0 {
			t.Errorf("InputPriceMTok mismatch: got %f, want %f", retrieved.InputPriceMTok, 3.0)
		}
		if retrieved.OutputPriceMTok != 15.0 {
			t.Errorf("OutputPriceMTok mismatch: got %f, want %f", retrieved.OutputPriceMTok, 15.0)
		}
	})

	t.Run("upsert updates existing model pricing", func(t *testing.T) {
		pricing := ModelPricingRecord{
			ID:              "test-pricing-id",
			ModelID:         "test-model",
			ModelName:       "Test Model Updated",
			InputPriceMTok:  4.0,
			OutputPriceMTok: 20.0,
			IsActive:        false,
		}

		err := s.UpsertModelPricing(ctx, pricing)
		if err != nil {
			t.Fatalf("UpsertModelPricing failed: %v", err)
		}

		// Verify update
		retrieved, err := s.GetModelPricing(ctx, "test-model")
		if err != nil {
			t.Fatalf("GetModelPricing failed: %v", err)
		}

		if retrieved.InputPriceMTok != 4.0 {
			t.Errorf("InputPriceMTok not updated: got %f, want %f", retrieved.InputPriceMTok, 4.0)
		}
		if retrieved.IsActive {
			t.Error("IsActive should be false after update")
		}
	})

	t.Run("delete model pricing", func(t *testing.T) {
		// Create a pricing to delete
		pricing := ModelPricingRecord{
			ID:              "delete-pricing-id",
			ModelID:         "delete-model",
			ModelName:       "Delete Model",
			InputPriceMTok:  1.0,
			OutputPriceMTok: 2.0,
			IsActive:        true,
		}
		if err := s.UpsertModelPricing(ctx, pricing); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Delete it
		err := s.DeleteModelPricing(ctx, "delete-model")
		if err != nil {
			t.Fatalf("DeleteModelPricing failed: %v", err)
		}

		// Verify deletion
		_, err = s.GetModelPricing(ctx, "delete-model")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})

	t.Run("calculate cost", func(t *testing.T) {
		// Use the test-model we created earlier
		cost, err := s.CalculateCost(ctx, "test-model", 1000000, 500000)
		if err != nil {
			t.Fatalf("CalculateCost failed: %v", err)
		}

		// Expected: (1000000 / 1000000) * 4.0 + (500000 / 1000000) * 20.0 = 4.0 + 10.0 = 14.0
		expected := 14.0
		if cost != expected {
			t.Errorf("Cost mismatch: got %f, want %f", cost, expected)
		}
	})

	t.Run("calculate cost for non-existent model returns zero", func(t *testing.T) {
		cost, err := s.CalculateCost(ctx, "non-existent-model", 1000, 1000)
		if err != nil {
			t.Fatalf("CalculateCost should not return error for unknown model, got: %v", err)
		}
		if cost != 0 {
			t.Errorf("Expected cost 0 for unknown model, got %f", cost)
		}
	})

	t.Run("insert usage log", func(t *testing.T) {
		log := UsageLogRecord{
			AccountID:    "test-acc-pricing",
			NodeID:       "test-node-pricing",
			ModelID:      "test-model",
			InputTokens:  1000,
			OutputTokens: 500,
			CostUSD:      0.05,
			RequestID:    "req-123",
			Success:      true,
		}

		err := s.InsertUsageLog(ctx, log)
		if err != nil {
			t.Fatalf("InsertUsageLog failed: %v", err)
		}
	})

	t.Run("query usage logs", func(t *testing.T) {
		// Insert more logs
		logs := []UsageLogRecord{
			{
				AccountID:    "test-acc-pricing",
				NodeID:       "test-node-pricing",
				ModelID:      "test-model",
				InputTokens:  2000,
				OutputTokens: 1000,
				CostUSD:      0.10,
				Success:      true,
			},
			{
				AccountID:    "test-acc-pricing",
				NodeID:       "test-node-pricing",
				ModelID:      "test-model",
				InputTokens:  3000,
				OutputTokens: 1500,
				CostUSD:      0.15,
				Success:      false,
			},
		}
		for _, log := range logs {
			if err := s.InsertUsageLog(ctx, log); err != nil {
				t.Fatalf("InsertUsageLog failed: %v", err)
			}
		}

		// Query logs
		params := QueryUsageParams{
			AccountID: "test-acc-pricing",
		}
		retrieved, err := s.QueryUsageLogs(ctx, params)
		if err != nil {
			t.Fatalf("QueryUsageLogs failed: %v", err)
		}

		if len(retrieved) < 3 {
			t.Errorf("Expected at least 3 logs, got %d", len(retrieved))
		}
	})

	t.Run("query usage logs with filters", func(t *testing.T) {
		params := QueryUsageParams{
			AccountID: "test-acc-pricing",
			NodeID:    "test-node-pricing",
			ModelID:   "test-model",
		}
		retrieved, err := s.QueryUsageLogs(ctx, params)
		if err != nil {
			t.Fatalf("QueryUsageLogs failed: %v", err)
		}

		for _, log := range retrieved {
			if log.AccountID != "test-acc-pricing" {
				t.Errorf("Expected account_id test-acc-pricing, got %s", log.AccountID)
			}
			if log.NodeID != "test-node-pricing" {
				t.Errorf("Expected node_id test-node-pricing, got %s", log.NodeID)
			}
		}
	})

	t.Run("query usage logs with time range", func(t *testing.T) {
		now := time.Now()
		params := QueryUsageParams{
			AccountID: "test-acc-pricing",
			From:      now.Add(-1 * time.Hour),
			To:        now.Add(1 * time.Hour),
		}
		retrieved, err := s.QueryUsageLogs(ctx, params)
		if err != nil {
			t.Fatalf("QueryUsageLogs failed: %v", err)
		}

		if len(retrieved) == 0 {
			t.Error("Expected at least one log in time range")
		}
	})

	t.Run("get usage summary", func(t *testing.T) {
		params := QueryUsageParams{
			AccountID: "test-acc-pricing",
		}
		summary, err := s.GetUsageSummary(ctx, params)
		if err != nil {
			t.Fatalf("GetUsageSummary failed: %v", err)
		}

		if summary == nil {
			t.Fatal("Expected summary, got nil")
		}
		if summary.TotalInputTokens == 0 {
			t.Error("Expected non-zero total input tokens")
		}
		if summary.TotalOutputTokens == 0 {
			t.Error("Expected non-zero total output tokens")
		}
		if summary.TotalCostUSD == 0 {
			t.Error("Expected non-zero total cost")
		}
	})

	t.Run("get usage summary by model", func(t *testing.T) {
		params := QueryUsageParams{
			AccountID: "test-acc-pricing",
		}
		summaries, err := s.GetUsageSummaryByModel(ctx, params)
		if err != nil {
			t.Fatalf("GetUsageSummaryByModel failed: %v", err)
		}

		if len(summaries) == 0 {
			t.Error("Expected at least one model summary")
		}

		for _, summary := range summaries {
			if summary.ModelID == "" {
				t.Error("Expected non-empty model_id")
			}
		}
	})

	t.Run("get usage summary by node", func(t *testing.T) {
		params := QueryUsageParams{
			AccountID: "test-acc-pricing",
		}
		summaries, err := s.GetUsageSummaryByNode(ctx, params)
		if err != nil {
			t.Fatalf("GetUsageSummaryByNode failed: %v", err)
		}

		if len(summaries) == 0 {
			t.Error("Expected at least one node summary")
		}

		for _, summary := range summaries {
			if summary.NodeID == "" {
				t.Error("Expected non-empty node_id")
			}
		}
	})

	t.Run("cleanup usage logs", func(t *testing.T) {
		// Insert an old log
		oldLog := UsageLogRecord{
			AccountID:    "test-acc-pricing",
			NodeID:       "test-node-pricing",
			ModelID:      "test-model",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.01,
			Success:      true,
		}
		if err := s.InsertUsageLog(ctx, oldLog); err != nil {
			t.Fatalf("InsertUsageLog failed: %v", err)
		}

		// Cleanup logs older than 0 days (should delete all)
		err := s.CleanupUsageLogs(ctx, 0)
		if err != nil {
			t.Fatalf("CleanupUsageLogs failed: %v", err)
		}

		// Note: We can't easily verify deletion without manipulating timestamps
		// This test just ensures the function doesn't error
	})
}

func TestCountUsageLogs(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	acc := AccountRecord{ID: "test-acc-count", Name: "Count Test"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Insert some logs
	for i := 0; i < 5; i++ {
		log := UsageLogRecord{
			AccountID:    "test-acc-count",
			NodeID:       "node-count",
			NodeName:     "Count Node",
			ModelID:      "test-model",
			InputTokens:  int64(100 * (i + 1)),
			OutputTokens: int64(50 * (i + 1)),
			CostUSD:      float64(i) * 0.01,
			Success:      i%2 == 0,
		}
		if err := s.InsertUsageLog(ctx, log); err != nil {
			t.Fatalf("InsertUsageLog failed: %v", err)
		}
	}

	t.Run("count all logs for account", func(t *testing.T) {
		total, err := s.CountUsageLogs(ctx, QueryUsageParams{AccountID: "test-acc-count"})
		if err != nil {
			t.Fatalf("CountUsageLogs failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected 5 logs, got %d", total)
		}
	})

	t.Run("count with node filter", func(t *testing.T) {
		total, err := s.CountUsageLogs(ctx, QueryUsageParams{AccountID: "test-acc-count", NodeID: "node-count"})
		if err != nil {
			t.Fatalf("CountUsageLogs failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected 5 logs, got %d", total)
		}
	})

	t.Run("count with model filter", func(t *testing.T) {
		total, err := s.CountUsageLogs(ctx, QueryUsageParams{AccountID: "test-acc-count", ModelID: "test-model"})
		if err != nil {
			t.Fatalf("CountUsageLogs failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected 5 logs, got %d", total)
		}
	})

	t.Run("count with success filter", func(t *testing.T) {
		success := true
		total, err := s.CountUsageLogs(ctx, QueryUsageParams{AccountID: "test-acc-count", Success: &success})
		if err != nil {
			t.Fatalf("CountUsageLogs failed: %v", err)
		}
		if total != 3 {
			t.Errorf("Expected 3 successful logs, got %d", total)
		}
	})

	t.Run("count with time range", func(t *testing.T) {
		now := time.Now()
		total, err := s.CountUsageLogs(ctx, QueryUsageParams{
			AccountID: "test-acc-count",
			From:      now.Add(-1 * time.Hour),
			To:        now.Add(1 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CountUsageLogs failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected 5 logs in time range, got %d", total)
		}
	})

	t.Run("count for non-existent account", func(t *testing.T) {
		total, err := s.CountUsageLogs(ctx, QueryUsageParams{AccountID: "non-existent"})
		if err != nil {
			t.Fatalf("CountUsageLogs failed: %v", err)
		}
		if total != 0 {
			t.Errorf("Expected 0 logs, got %d", total)
		}
	})
}

func TestInsertUsageLogWithAttempts(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	acc := AccountRecord{ID: "test-acc-attempts", Name: "Attempts Test"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("insert log with attempts", func(t *testing.T) {
		log := UsageLogRecord{
			AccountID:     "test-acc-attempts",
			NodeID:        "node-attempts",
			NodeName:      "Attempts Node",
			ModelID:       "test-model",
			InputTokens:   1000,
			OutputTokens:  500,
			CostUSD:       0.05,
			RequestID:     "req-attempts-1",
			Success:       true,
			DurationMs:    1500,
			TotalAttempts: 3,
			Attempts: []UsageLogAttempt{
				{Seq: 1, NodeID: "node-1", NodeName: "Node 1", StatusCode: 500, Success: false, DurationMs: 200, ErrorMsg: "server error", Severity: "node_down", Action: "retry"},
				{Seq: 2, NodeID: "node-2", NodeName: "Node 2", StatusCode: 429, Success: false, DurationMs: 300, ErrorMsg: "rate limited", Severity: "transient", Action: "retry"},
				{Seq: 3, NodeID: "node-attempts", NodeName: "Attempts Node", StatusCode: 200, Success: true, DurationMs: 1000, Action: "success"},
			},
		}

		err := s.InsertUsageLog(ctx, log)
		if err != nil {
			t.Fatalf("InsertUsageLog with attempts failed: %v", err)
		}

		// Verify the log was inserted
		params := QueryUsageParams{AccountID: "test-acc-attempts"}
		logs, err := s.QueryUsageLogs(ctx, params)
		if err != nil {
			t.Fatalf("QueryUsageLogs failed: %v", err)
		}
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}
		if logs[0].TotalAttempts != 3 {
			t.Errorf("Expected 3 total attempts, got %d", logs[0].TotalAttempts)
		}
		if logs[0].RequestID != "req-attempts-1" {
			t.Errorf("Expected request_id req-attempts-1, got %s", logs[0].RequestID)
		}
	})
}

func TestQueryAttemptsByLogIDs(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	acc := AccountRecord{ID: "test-acc-qattempts", Name: "Query Attempts Test"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Insert logs with attempts
	log1 := UsageLogRecord{
		AccountID:     "test-acc-qattempts",
		NodeID:        "node-qa",
		NodeName:      "QA Node",
		ModelID:       "test-model",
		InputTokens:   1000,
		OutputTokens:  500,
		Success:       true,
		TotalAttempts: 2,
		Attempts: []UsageLogAttempt{
			{Seq: 1, NodeID: "node-qa-1", NodeName: "QA Node 1", StatusCode: 500, Success: false, DurationMs: 100, ErrorMsg: "error", Severity: "node_down", Action: "retry"},
			{Seq: 2, NodeID: "node-qa", NodeName: "QA Node", StatusCode: 200, Success: true, DurationMs: 200, Action: "success"},
		},
	}
	if err := s.InsertUsageLog(ctx, log1); err != nil {
		t.Fatalf("InsertUsageLog failed: %v", err)
	}

	log2 := UsageLogRecord{
		AccountID:     "test-acc-qattempts",
		NodeID:        "node-qa",
		NodeName:      "QA Node",
		ModelID:       "test-model",
		InputTokens:   2000,
		OutputTokens:  1000,
		Success:       false,
		TotalAttempts: 1,
		Attempts: []UsageLogAttempt{
			{Seq: 1, NodeID: "node-qa", NodeName: "QA Node", StatusCode: 500, Success: false, DurationMs: 300, ErrorMsg: "server error", Severity: "node_down", Action: "fail"},
		},
	}
	if err := s.InsertUsageLog(ctx, log2); err != nil {
		t.Fatalf("InsertUsageLog failed: %v", err)
	}

	// Get the log IDs
	params := QueryUsageParams{AccountID: "test-acc-qattempts"}
	logs, err := s.QueryUsageLogs(ctx, params)
	if err != nil {
		t.Fatalf("QueryUsageLogs failed: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("Expected at least 2 logs, got %d", len(logs))
	}

	logIDs := make([]int64, len(logs))
	for i, l := range logs {
		logIDs[i] = l.ID
	}

	t.Run("query attempts by log IDs", func(t *testing.T) {
		attempts, err := s.QueryAttemptsByLogIDs(ctx, logIDs)
		if err != nil {
			t.Fatalf("QueryAttemptsByLogIDs failed: %v", err)
		}
		if len(attempts) != 2 {
			t.Errorf("Expected attempts for 2 logs, got %d", len(attempts))
		}
		// Check that we have the right number of attempts per log
		totalAttempts := 0
		for _, a := range attempts {
			totalAttempts += len(a)
		}
		if totalAttempts != 3 {
			t.Errorf("Expected 3 total attempts, got %d", totalAttempts)
		}
	})

	t.Run("query attempts with empty IDs", func(t *testing.T) {
		attempts, err := s.QueryAttemptsByLogIDs(ctx, nil)
		if err != nil {
			t.Fatalf("QueryAttemptsByLogIDs failed: %v", err)
		}
		if attempts != nil {
			t.Errorf("Expected nil for empty IDs, got %v", attempts)
		}
	})

	t.Run("query attempts with non-existent IDs", func(t *testing.T) {
		attempts, err := s.QueryAttemptsByLogIDs(ctx, []int64{99999})
		if err != nil {
			t.Fatalf("QueryAttemptsByLogIDs failed: %v", err)
		}
		if len(attempts) != 0 {
			t.Errorf("Expected 0 attempts for non-existent IDs, got %d", len(attempts))
		}
	})
}
