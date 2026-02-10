package proxy

import (
	"testing"
	"time"
)

func TestModelRecoveryTracker_MarkFailed(t *testing.T) {
	tracker := NewModelRecoveryTracker()

	// 初始状态：无失败模型
	if tracker.Count() != 0 {
		t.Fatalf("expected 0, got %d", tracker.Count())
	}

	// 标记失败
	tracker.MarkFailed("node1", "claude-sonnet-4-20250514", "acc1", "rate limited")
	if tracker.Count() != 1 {
		t.Fatalf("expected 1, got %d", tracker.Count())
	}

	// 同一个模型再次标记，不应增加计数
	tracker.MarkFailed("node1", "claude-sonnet-4-20250514", "acc1", "still rate limited")
	if tracker.Count() != 1 {
		t.Fatalf("expected 1 after duplicate, got %d", tracker.Count())
	}

	// 不同模型
	tracker.MarkFailed("node1", "claude-opus-4-20250514", "acc1", "overloaded")
	if tracker.Count() != 2 {
		t.Fatalf("expected 2, got %d", tracker.Count())
	}

	// 不同节点同模型
	tracker.MarkFailed("node2", "claude-sonnet-4-20250514", "acc1", "timeout")
	if tracker.Count() != 3 {
		t.Fatalf("expected 3, got %d", tracker.Count())
	}
}

func TestModelRecoveryTracker_MarkRecovered(t *testing.T) {
	tracker := NewModelRecoveryTracker()

	tracker.MarkFailed("node1", "claude-sonnet-4-20250514", "acc1", "error")
	tracker.MarkFailed("node1", "claude-opus-4-20250514", "acc1", "error")
	tracker.MarkFailed("node2", "claude-sonnet-4-20250514", "acc1", "error")

	// 恢复一个
	tracker.MarkRecovered("node1", "claude-sonnet-4-20250514")
	if tracker.Count() != 2 {
		t.Fatalf("expected 2 after recovery, got %d", tracker.Count())
	}

	// 确认正确的被移除
	if tracker.IsModelFailed("node1", "claude-sonnet-4-20250514") {
		t.Fatal("node1/claude-sonnet-4-20250514 should not be failed after recovery")
	}
	if !tracker.IsModelFailed("node1", "claude-opus-4-20250514") {
		t.Fatal("node1/claude-opus-4-20250514 should still be failed")
	}
	if !tracker.IsModelFailed("node2", "claude-sonnet-4-20250514") {
		t.Fatal("node2/claude-sonnet-4-20250514 should still be failed")
	}
}

func TestModelRecoveryTracker_MarkNodeRecovered(t *testing.T) {
	tracker := NewModelRecoveryTracker()

	tracker.MarkFailed("node1", "model-a", "acc1", "error")
	tracker.MarkFailed("node1", "model-b", "acc1", "error")
	tracker.MarkFailed("node2", "model-a", "acc1", "error")

	// 清除整个节点
	tracker.MarkNodeRecovered("node1")
	if tracker.Count() != 1 {
		t.Fatalf("expected 1 after node recovery, got %d", tracker.Count())
	}
	if tracker.IsModelFailed("node1", "model-a") {
		t.Fatal("node1/model-a should not be failed")
	}
	if !tracker.IsModelFailed("node2", "model-a") {
		t.Fatal("node2/model-a should still be failed")
	}
}

func TestModelRecoveryTracker_GetAllFailedByAccount(t *testing.T) {
	tracker := NewModelRecoveryTracker()

	tracker.MarkFailed("node1", "model-a", "acc1", "error")
	tracker.MarkFailed("node2", "model-b", "acc2", "error")
	tracker.MarkFailed("node3", "model-c", "acc1", "error")

	acc1Items := tracker.GetAllFailedByAccount("acc1")
	if len(acc1Items) != 2 {
		t.Fatalf("expected 2 items for acc1, got %d", len(acc1Items))
	}

	acc2Items := tracker.GetAllFailedByAccount("acc2")
	if len(acc2Items) != 1 {
		t.Fatalf("expected 1 item for acc2, got %d", len(acc2Items))
	}
}

func TestModelRecoveryTracker_GetPendingRecoveryChecks(t *testing.T) {
	tracker := NewModelRecoveryTracker()

	tracker.MarkFailed("node1", "model-a", "acc1", "error")
	tracker.MarkFailed("node1", "model-b", "acc1", "error")
	tracker.MarkFailed("node2", "model-a", "acc1", "error")

	pending := tracker.GetPendingRecoveryChecks()
	if len(pending) != 2 {
		t.Fatalf("expected 2 nodes in pending, got %d", len(pending))
	}
	if len(pending["node1"]) != 2 {
		t.Fatalf("expected 2 models for node1, got %d", len(pending["node1"]))
	}
	if len(pending["node2"]) != 1 {
		t.Fatalf("expected 1 model for node2, got %d", len(pending["node2"]))
	}

	// 验证 CheckCount 和 LastCheck 被更新
	items := tracker.GetFailedModels("node1")
	for _, item := range items {
		if item.CheckCount != 1 {
			t.Fatalf("expected CheckCount=1, got %d", item.CheckCount)
		}
		if item.LastCheck.IsZero() {
			t.Fatal("LastCheck should not be zero")
		}
	}
}

func TestModelRecoveryTracker_OfflineDuration(t *testing.T) {
	info := &FailedModelInfo{
		FailedAt: time.Now().Add(-5 * time.Minute),
	}
	dur := info.OfflineDuration()
	if dur < 4*time.Minute || dur > 6*time.Minute {
		t.Fatalf("expected ~5min offline duration, got %v", dur)
	}
}

func TestModelRecoveryTracker_EmptyOperations(t *testing.T) {
	tracker := NewModelRecoveryTracker()

	// 空操作不应 panic
	tracker.MarkFailed("", "model", "acc", "err")
	tracker.MarkFailed("node", "", "acc", "err")
	tracker.MarkRecovered("", "model")
	tracker.MarkRecovered("node", "")
	tracker.MarkNodeRecovered("")

	if tracker.Count() != 0 {
		t.Fatalf("expected 0 after empty operations, got %d", tracker.Count())
	}

	if tracker.IsModelFailed("nonexistent", "model") {
		t.Fatal("nonexistent node should not have failed models")
	}

	items := tracker.GetFailedModels("nonexistent")
	if len(items) != 0 {
		t.Fatalf("expected 0 items for nonexistent node, got %d", len(items))
	}
}
