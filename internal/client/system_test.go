package client

import (
	"os"
	"testing"
)

func TestSystem0_Minimal(t *testing.T) {
	result := system0(true)
	expected := "You are Claude Code, Anthropic's official CLI for Claude."
	if result != expected {
		t.Errorf("system0(true) = %s, want %s", result, expected)
	}
}

func TestSystem0_Full(t *testing.T) {
	result := system0(false)
	// Should return the full system prompt from cccli.System0
	if result == "" {
		t.Error("system0(false) returned empty string")
	}
	if result == "You are Claude Code, Anthropic's official CLI for Claude." {
		t.Error("system0(false) should return full prompt, not minimal")
	}
}

func TestRenderSystem1_Minimal(t *testing.T) {
	cfg := Config{Minimal: true}
	result := renderSystem1(cfg, "test-model")
	expected := "You are a file search specialist for Claude Code, Anthropic's official CLI for Claude."
	if result != expected {
		t.Errorf("renderSystem1(minimal) = %s, want %s", result, expected)
	}
}

func TestRenderSystem1_Full(t *testing.T) {
	cfg := Config{Minimal: false}
	model := "claude-sonnet-4-5-20250929"

	// Set OSTYPE for testing
	t.Setenv("OSTYPE", "linux-gnu")

	result := renderSystem1(cfg, model)

	// Should contain environment block
	if !containsSubstring(result, "Working directory:") {
		t.Error("Result should contain 'Working directory:'")
	}

	if !containsSubstring(result, "Is directory a git repo:") {
		t.Error("Result should contain 'Is directory a git repo:'")
	}

	if !containsSubstring(result, "Platform:") {
		t.Error("Result should contain 'Platform:'")
	}

	if !containsSubstring(result, "OS Version:") {
		t.Error("Result should contain 'OS Version:'")
	}

	if !containsSubstring(result, "Today's date:") {
		t.Error("Result should contain 'Today's date:'")
	}

	// Should contain the model ID
	if !containsSubstring(result, model) {
		t.Errorf("Result should contain model ID %s", model)
	}
}

func TestRenderSystem1_ModelReplacement(t *testing.T) {
	cfg := Config{Minimal: false}
	customModel := "claude-opus-4-5-20251101"

	result := renderSystem1(cfg, customModel)

	// Should replace the model ID
	if !containsSubstring(result, customModel) {
		t.Errorf("Result should contain custom model ID %s", customModel)
	}

	// Should not contain the default model IDs
	if containsSubstring(result, "The exact model ID is claude-haiku-4-5-20251001.") {
		t.Error("Result should not contain default haiku model ID")
	}

	if containsSubstring(result, "The exact model ID is claude-sonnet-4-5-20250929.") {
		t.Error("Result should not contain default sonnet model ID")
	}
}

func TestMustPwd(t *testing.T) {
	result := mustPwd()
	if result == "" {
		t.Error("mustPwd() returned empty string")
	}

	// Should be a valid directory
	info, err := os.Stat(result)
	if err != nil {
		t.Errorf("mustPwd() returned invalid path: %v", err)
	}

	if !info.IsDir() {
		t.Error("mustPwd() should return a directory")
	}
}

func TestUname(t *testing.T) {
	result := uname()
	if result == "" {
		t.Error("uname() returned empty string")
	}

	// Should contain a slash
	if !containsSubstring(result, "/") {
		t.Error("uname() should contain '/' separator")
	}

	// Should be in format "os/arch"
	parts := splitString(result, "/")
	if len(parts) != 2 {
		t.Errorf("uname() should return 'os/arch' format, got %s", result)
	}

	// First part should be a valid OS
	validOS := []string{"darwin", "linux", "windows", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "android", "plan9"}
	if !containsString(validOS, parts[0]) {
		t.Errorf("uname() returned unknown OS: %s", parts[0])
	}

	// Second part should be a valid arch
	validArch := []string{"amd64", "386", "arm", "arm64", "ppc64", "ppc64le", "mips", "mipsle", "mips64", "mips64le", "s390x", "riscv64"}
	if !containsString(validArch, parts[1]) {
		t.Errorf("uname() returned unknown arch: %s", parts[1])
	}
}

func TestGitFlag(t *testing.T) {
	result := gitFlag()
	if result != "Unknown" {
		t.Errorf("gitFlag() = %s, want Unknown", result)
	}
}

// Helper functions
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
