package client

import (
	"os"
	"strings"
	"testing"
)

func TestExtractHash(t *testing.T) {
	line := "\"user_id\":\"user_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890_account__session_foo\""
	if idx := strings.Index(line, "user_"); idx == -1 {
		t.Fatalf("user_ not found in line: %s", line)
	}
	got := extractHash(line)
	t.Logf("line=%q got=%q", line, got)
	want := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if got != want {
		t.Fatalf("extractHash got %s want %s", got, want)
	}
}

func TestSystem0Minimal(t *testing.T) {
	if got := system0(true); got != "You are Claude Code, Anthropic's official CLI for Claude." {
		t.Fatalf("unexpected system0 minimal: %s", got)
	}
}

func TestLoadConfigUsesEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "token")
	t.Setenv("MODEL", "m1")
	cfg, err := LoadConfig([]string{"hello"})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if cfg.Model != "m1" {
		t.Fatalf("model not picked from env, got %s", cfg.Model)
	}
}

func TestLoadToolsJSONIsValid(t *testing.T) {
	if os.Getenv("SKIP_TOOL_JSON") == "1" { // escape hatch if needed
		t.Skip("SKIP_TOOL_JSON set")
	}
	_ = loadTools() // panics on invalid JSON
}

func TestComputeUserHashPrefersEnv(t *testing.T) {
	cfg := Config{UserHash: "deadbeef"}
	if got := computeUserHash(cfg); got != "deadbeef" {
		t.Fatalf("expected user-supplied hash")
	}
}
