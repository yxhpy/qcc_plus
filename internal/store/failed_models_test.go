package store

import (
	"context"
	"testing"
	"time"
)

func TestUpsertFailedModelSQLite(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	initial := FailedModelRecord{
		NodeID:         "node-1",
		ModelID:        "claude-sonnet-4-7",
		AccountID:      "acc-1",
		Error:          "first failure",
		FailedAt:       now,
		LastCheck:      now,
		CheckCount:     1,
		NonRecoverable: false,
	}
	if err := s.UpsertFailedModel(ctx, initial); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	updated := initial
	updated.Error = "updated failure"
	updated.LastCheck = now.Add(5 * time.Minute)
	updated.CheckCount = 3
	updated.NonRecoverable = true
	if err := s.UpsertFailedModel(ctx, updated); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	records, err := s.ListAllFailedModels(ctx)
	if err != nil {
		t.Fatalf("list failed models: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 failed model record, got %d", len(records))
	}

	record := records[0]
	if record.Error != updated.Error {
		t.Fatalf("expected error %q, got %q", updated.Error, record.Error)
	}
	if record.CheckCount != updated.CheckCount {
		t.Fatalf("expected check_count %d, got %d", updated.CheckCount, record.CheckCount)
	}
	if !record.NonRecoverable {
		t.Fatal("expected non_recoverable to be true")
	}
	if !record.LastCheck.Equal(updated.LastCheck) {
		t.Fatalf("expected last_check %v, got %v", updated.LastCheck, record.LastCheck)
	}
}
