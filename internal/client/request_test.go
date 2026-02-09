package client

import (
	"fmt"
	"regexp"
	"testing"
)

func TestCacheCtrl(t *testing.T) {
	t.Run("returns cache control with ephemeral type", func(t *testing.T) {
		cc := cacheCtrl()
		if cc == nil {
			t.Fatal("cacheCtrl() returned nil")
		}
		if cc.Type != "ephemeral" {
			t.Errorf("Type = %s, want ephemeral", cc.Type)
		}
	})
}

func TestBuildSystem(t *testing.T) {
	t.Run("builds system blocks with minimal config", func(t *testing.T) {
		cfg := Config{Minimal: true}
		system1 := "test system1"

		blocks := buildSystem(cfg, system1)

		if len(blocks) != 2 {
			t.Fatalf("Expected 2 blocks, got %d", len(blocks))
		}

		// Check first block
		if blocks[0].Type != "text" {
			t.Errorf("Block 0 Type = %s, want text", blocks[0].Type)
		}
		if blocks[0].CacheControl == nil {
			t.Error("Block 0 CacheControl is nil")
		}

		// Check second block
		if blocks[1].Type != "text" {
			t.Errorf("Block 1 Type = %s, want text", blocks[1].Type)
		}
		if blocks[1].Text != system1 {
			t.Errorf("Block 1 Text = %s, want %s", blocks[1].Text, system1)
		}
		if blocks[1].CacheControl == nil {
			t.Error("Block 1 CacheControl is nil")
		}
	})

	t.Run("builds system blocks with non-minimal config", func(t *testing.T) {
		cfg := Config{Minimal: false}
		system1 := "test system1"

		blocks := buildSystem(cfg, system1)

		if len(blocks) != 2 {
			t.Fatalf("Expected 2 blocks, got %d", len(blocks))
		}
	})
}

func TestLoadTools(t *testing.T) {
	t.Run("loads tools from JSON", func(t *testing.T) {
		tools := loadTools()
		if tools == nil {
			t.Fatal("loadTools() returned nil")
		}

		// Tools should be a valid JSON structure
		// We can't check the exact structure, but we can verify it's not empty
		toolsSlice, ok := tools.([]interface{})
		if !ok {
			t.Fatal("loadTools() did not return a slice")
		}

		if len(toolsSlice) == 0 {
			t.Error("loadTools() returned empty slice")
		}
	})
}

func TestMessageBody(t *testing.T) {
	t.Run("creates message body with all fields", func(t *testing.T) {
		cfg := Config{
			Message: "test message",
			Minimal: false,
		}
		model := "claude-sonnet-4-5-20250929"
		tools := loadTools()
		system1 := "test system1"

		body := messageBody(cfg, model, tools, system1)

		if body.Model != model {
			t.Errorf("Model = %s, want %s", body.Model, model)
		}

		if len(body.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(body.Messages))
		}

		msg := body.Messages[0]
		if msg.Role != "user" {
			t.Errorf("Role = %s, want user", msg.Role)
		}

		if len(msg.Content) != 1 {
			t.Fatalf("Expected 1 content item, got %d", len(msg.Content))
		}

		content := msg.Content[0]
		if content.Type != "text" {
			t.Errorf("Content Type = %s, want text", content.Type)
		}
		if content.Text != "test message" {
			t.Errorf("Content Text = %s, want test message", content.Text)
		}

		if len(body.System) != 2 {
			t.Errorf("Expected 2 system blocks, got %d", len(body.System))
		}

		if body.Tools == nil {
			t.Error("Tools is nil")
		}

		if body.MaxTokens != 32000 {
			t.Errorf("MaxTokens = %d, want 32000", body.MaxTokens)
		}

		if !body.Stream {
			t.Error("Stream should be true")
		}

		if body.Metadata == nil {
			t.Fatal("Metadata is nil")
		}

		userID, ok := body.Metadata["user_id"].(string)
		if !ok {
			t.Fatal("user_id not found in metadata")
		}
		if userID == "" {
			t.Error("user_id is empty")
		}
	})
}

func TestWarmupBody(t *testing.T) {
	t.Run("creates warmup body", func(t *testing.T) {
		cfg := Config{
			Message: "ignored",
			Minimal: false,
		}
		model := "claude-sonnet-4-5-20250929"

		body := warmupBody(cfg, model)

		if body.Model != model {
			t.Errorf("Model = %s, want %s", body.Model, model)
		}

		if len(body.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(body.Messages))
		}

		msg := body.Messages[0]
		if msg.Role != "user" {
			t.Errorf("Role = %s, want user", msg.Role)
		}

		if len(msg.Content) != 1 {
			t.Fatalf("Expected 1 content item, got %d", len(msg.Content))
		}

		content := msg.Content[0]
		if content.Text != "Warmup" {
			t.Errorf("Content Text = %s, want Warmup", content.Text)
		}

		if len(body.System) != 2 {
			t.Errorf("Expected 2 system blocks, got %d", len(body.System))
		}

		if body.MaxTokens != 32000 {
			t.Errorf("MaxTokens = %d, want 32000", body.MaxTokens)
		}

		if !body.Stream {
			t.Error("Stream should be true")
		}

		if body.Metadata == nil {
			t.Fatal("Metadata is nil")
		}
	})
}

func TestUUID(t *testing.T) {
	t.Run("generates valid UUID", func(t *testing.T) {
		id := uuid()

		// UUID format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		// where y is 8, 9, a, or b
		pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
		matched, err := regexp.MatchString(pattern, id)
		if err != nil {
			t.Fatalf("Regex error: %v", err)
		}

		if !matched {
			t.Errorf("UUID %s does not match expected format", id)
		}
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		id1 := uuid()
		id2 := uuid()

		if id1 == id2 {
			t.Error("Generated UUIDs should be unique")
		}
	})

	t.Run("UUID has correct version and variant", func(t *testing.T) {
		id := uuid()

		// Check version (4th group should start with 4)
		if id[14] != '4' {
			t.Errorf("UUID version should be 4, got %c", id[14])
		}

		// Check variant (5th group should start with 8, 9, a, or b)
		variant := id[19]
		if variant != '8' && variant != '9' && variant != 'a' && variant != 'b' {
			t.Errorf("UUID variant should be 8/9/a/b, got %c", variant)
		}
	})
}

func TestFirstEnv(t *testing.T) {
	t.Run("returns first non-empty env var", func(t *testing.T) {
		// Set test environment variables
		t.Setenv("TEST_VAR1", "")
		t.Setenv("TEST_VAR2", "value2")
		t.Setenv("TEST_VAR3", "value3")

		result := firstEnv("TEST_VAR1", "TEST_VAR2", "TEST_VAR3")
		if result != "value2" {
			t.Errorf("firstEnv() = %s, want value2", result)
		}
	})

	t.Run("returns empty string if all vars are empty", func(t *testing.T) {
		result := firstEnv("NONEXISTENT_VAR1", "NONEXISTENT_VAR2")
		if result != "" {
			t.Errorf("firstEnv() = %s, want empty string", result)
		}
	})

	t.Run("returns first var if it's set", func(t *testing.T) {
		t.Setenv("TEST_FIRST", "first_value")
		t.Setenv("TEST_SECOND", "second_value")

		result := firstEnv("TEST_FIRST", "TEST_SECOND")
		if result != "first_value" {
			t.Errorf("firstEnv() = %s, want first_value", result)
		}
	})
}

func TestMust(t *testing.T) {
	t.Run("does not panic on nil error", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("must() panicked on nil error: %v", r)
			}
		}()

		must(nil)
	})

	t.Run("panics on non-nil error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("must() did not panic on error")
			}
		}()

		must(fmt.Errorf("test error"))
	})
}
