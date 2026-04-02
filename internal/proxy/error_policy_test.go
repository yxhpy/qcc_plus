package proxy

import (
	"testing"
)

func TestErrorPolicyManager_ObservedToggle(t *testing.T) {
	cache := &SettingsCache{data: map[string]any{}}
	mgr := NewErrorPolicyManager(cache)

	mgr.Record(400, "auth_failed", "token expired")
	snap := mgr.Snapshot()
	if len(snap.Observed) == 0 {
		t.Fatal("expected observed errors")
	}
	id := snap.Observed[0].ID
	if snap.Observed[0].AutoSwitch {
		t.Fatal("expected auto_switch false by default")
	}

	mgr.SetObservedAutoSwitch(id, true)
	snap = mgr.Snapshot()
	if !snap.Observed[0].AutoSwitch {
		t.Fatal("expected auto_switch true after enabling")
	}

	mgr.SetObservedAutoSwitch(id, false)
	snap = mgr.Snapshot()
	if snap.Observed[0].AutoSwitch {
		t.Fatal("expected auto_switch false after disabling")
	}
}

func TestErrorPolicyManager_ApplyBuiltinRule(t *testing.T) {
	cache := &SettingsCache{data: map[string]any{}}
	mgr := NewErrorPolicyManager(cache)

	in := ClassifiedError{Severity: SeverityPermanent, Retryable: false, Code: "packy_api_error", Message: "未知错误类型: packy_api_error"}
	out := mgr.Apply(400, in)
	if out.Severity != SeverityNodeDown {
		t.Fatalf("severity = %v, want %v", out.Severity, SeverityNodeDown)
	}
	if !out.Retryable {
		t.Fatal("retryable = false, want true")
	}
}
