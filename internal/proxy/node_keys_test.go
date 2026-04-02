package proxy

import "testing"

func TestNewKeyRotatorFromNamedKeys(t *testing.T) {
	rotator := NewKeyRotatorFromNamedKeys([]NamedAPIKey{
		{Name: "primary", Key: "sk-primary"},
		{Name: "backup", Key: "sk-backup"},
		{Name: "ignored", Key: "  "},
	}, loadKeyRotatorConfig())

	if rotator == nil {
		t.Fatal("expected rotator")
	}
	if got := rotator.KeyCount(); got != 2 {
		t.Fatalf("expected 2 keys, got %d", got)
	}
	if got := rotator.GetPrimaryKey(); got != "sk-primary" {
		t.Fatalf("expected primary key sk-primary, got %q", got)
	}
}

func TestApplyNodeKeyState(t *testing.T) {
	node := &Node{}
	applyNodeKeyState(node, "", `[{"name":"primary","key":"sk-primary"},{"name":"backup","key":"sk-backup"}]`)

	if node.APIKey != "sk-primary,sk-backup" {
		t.Fatalf("expected joined api key, got %q", node.APIKey)
	}
	if node.APIKeyConfig != `[{"name":"primary","key":"sk-primary"},{"name":"backup","key":"sk-backup"}]` {
		t.Fatalf("expected api key config to be preserved, got %q", node.APIKeyConfig)
	}
	if len(node.APIKeyItems) != 2 {
		t.Fatalf("expected 2 api key items, got %d", len(node.APIKeyItems))
	}
	if node.APIKeys == nil || node.APIKeys.KeyCount() != 2 {
		t.Fatalf("expected rotator with 2 keys")
	}
}
