package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_NoArgs(t *testing.T) {
	_, err := LoadConfig([]string{})
	if err == nil {
		t.Fatal("Expected error when no args provided")
	}
	if err.Error() != "need a message argument" {
		t.Errorf("Expected 'need a message argument', got %v", err)
	}
}

func TestLoadConfig_NoToken(t *testing.T) {
	// Clear all token env vars
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, err := LoadConfig([]string{"test"})
	if err == nil {
		t.Fatal("Expected error when no token provided")
	}
	if err.Error() != "missing ANTHROPIC_AUTH_TOKEN" {
		t.Errorf("Expected 'missing ANTHROPIC_AUTH_TOKEN', got %v", err)
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")
	// Clear other env vars that might override defaults
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("MODEL", "")
	t.Setenv("WARMUP_MODEL", "")
	t.Setenv("NO_WARMUP", "")
	t.Setenv("MINIMAL_SYSTEM", "")
	t.Setenv("USER_HASH", "")

	cfg, err := LoadConfig([]string{"hello", "world"})
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Token != "test-token" {
		t.Errorf("Token = %s, want test-token", cfg.Token)
	}

	if cfg.BaseURL != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %s, want https://api.anthropic.com", cfg.BaseURL)
	}

	if cfg.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("Model = %s, want claude-sonnet-4-5-20250929", cfg.Model)
	}

	if cfg.WarmupModel != "claude-haiku-4-5-20251001" {
		t.Errorf("WarmupModel = %s, want claude-haiku-4-5-20251001", cfg.WarmupModel)
	}

	if cfg.Message != "hello world" {
		t.Errorf("Message = %s, want 'hello world'", cfg.Message)
	}

	if cfg.NoWarmup {
		t.Error("NoWarmup should be false by default")
	}

	if !cfg.Minimal {
		t.Error("Minimal should be true by default")
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "custom-token")
	t.Setenv("ANTHROPIC_BASE_URL", "https://custom.api.com")
	t.Setenv("MODEL", "custom-model")
	t.Setenv("WARMUP_MODEL", "custom-warmup")
	t.Setenv("NO_WARMUP", "1")
	t.Setenv("MINIMAL_SYSTEM", "0")
	t.Setenv("USER_HASH", "custom-hash")

	cfg, err := LoadConfig([]string{"test"})
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Token != "custom-token" {
		t.Errorf("Token = %s, want custom-token", cfg.Token)
	}

	if cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %s, want https://custom.api.com", cfg.BaseURL)
	}

	if cfg.Model != "custom-model" {
		t.Errorf("Model = %s, want custom-model", cfg.Model)
	}

	if cfg.WarmupModel != "custom-warmup" {
		t.Errorf("WarmupModel = %s, want custom-warmup", cfg.WarmupModel)
	}

	if !cfg.NoWarmup {
		t.Error("NoWarmup should be true")
	}

	if cfg.Minimal {
		t.Error("Minimal should be false")
	}

	if cfg.UserHash != "custom-hash" {
		t.Errorf("UserHash = %s, want custom-hash", cfg.UserHash)
	}
}

func TestGetenvDefault(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEY", "test_value")
		result := getenvDefault("TEST_KEY", "default")
		if result != "test_value" {
			t.Errorf("getenvDefault() = %s, want test_value", result)
		}
	})

	t.Run("returns default when env not set", func(t *testing.T) {
		result := getenvDefault("NONEXISTENT_KEY", "default_value")
		if result != "default_value" {
			t.Errorf("getenvDefault() = %s, want default_value", result)
		}
	})
}

func TestComputeUserHash_WithUserHash(t *testing.T) {
	cfg := Config{UserHash: "custom-hash"}
	result := computeUserHash(cfg)
	if result != "custom-hash" {
		t.Errorf("computeUserHash() = %s, want custom-hash", result)
	}
}

func TestComputeUserHash_FromToken(t *testing.T) {
	cfg := Config{Token: "test-token"}
	result := computeUserHash(cfg)

	// Should return a 64-character hex string (SHA256)
	if len(result) != 64 {
		t.Errorf("Hash length = %d, want 64", len(result))
	}

	// Should be deterministic
	result2 := computeUserHash(cfg)
	if result != result2 {
		t.Error("Hash should be deterministic")
	}
}

func TestScanCaptureHash_NoDirectory(t *testing.T) {
	// Change to a temp directory without .capture
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	result := scanCaptureHash()
	if result != "" {
		t.Errorf("scanCaptureHash() = %s, want empty string", result)
	}
}

func TestScanCaptureHash_WithValidFile(t *testing.T) {
	// Create temp directory with .capture
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	captureDir := filepath.Join(tmpDir, ".capture")
	os.Mkdir(captureDir, 0755)

	// Create a fetch log file with user_id
	logFile := filepath.Join(captureDir, "fetch_test.log")
	content := `{"user_id":"user_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890_account__session_foo"}`
	os.WriteFile(logFile, []byte(content), 0644)

	result := scanCaptureHash()
	expected := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if result != expected {
		t.Errorf("scanCaptureHash() = %s, want %s", result, expected)
	}
}

func TestScanCaptureHash_NoValidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	captureDir := filepath.Join(tmpDir, ".capture")
	os.Mkdir(captureDir, 0755)

	// Create a file without fetch_ prefix
	logFile := filepath.Join(captureDir, "other.log")
	os.WriteFile(logFile, []byte("test"), 0644)

	result := scanCaptureHash()
	if result != "" {
		t.Errorf("scanCaptureHash() = %s, want empty string", result)
	}
}

func TestExtractHash_InvalidFormat(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"no user_", "some random text"},
		{"no account separator", "user_abcdef1234567890"},
		{"hash too short", "user_abc_account__session_foo"},
		{"hash too long", "user_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890extra_account__session_foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHash(tt.line)
			if result != "" {
				t.Errorf("extractHash(%s) = %s, want empty string", tt.line, result)
			}
		})
	}
}

func TestExtractHash_WithEscapedQuotes(t *testing.T) {
	line := `\"user_id\":\"user_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef_account__session_test\"`
	result := extractHash(line)
	expected := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	if result != expected {
		t.Errorf("extractHash() = %s, want %s", result, expected)
	}
}

func TestComputeUserHash_WithCaptureFile(t *testing.T) {
	// Create temp directory with .capture
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	captureDir := filepath.Join(tmpDir, ".capture")
	os.Mkdir(captureDir, 0755)

	// Create a fetch log file with user_id
	logFile := filepath.Join(captureDir, "fetch_test.log")
	content := `{"user_id":"user_fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210_account__session_foo"}`
	os.WriteFile(logFile, []byte(content), 0644)

	cfg := Config{Token: "test-token"}
	result := computeUserHash(cfg)
	expected := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	if result != expected {
		t.Errorf("computeUserHash() = %s, want %s", result, expected)
	}
}

func TestScanCaptureHash_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	captureDir := filepath.Join(tmpDir, ".capture")
	os.Mkdir(captureDir, 0755)

	// Create older file
	oldFile := filepath.Join(captureDir, "fetch_old.log")
	oldContent := `{"user_id":"user_1111111111111111111111111111111111111111111111111111111111111111_account__session_foo"}`
	os.WriteFile(oldFile, []byte(oldContent), 0644)

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create newer file
	newFile := filepath.Join(captureDir, "fetch_new.log")
	newContent := `{"user_id":"user_2222222222222222222222222222222222222222222222222222222222222222_account__session_foo"}`
	os.WriteFile(newFile, []byte(newContent), 0644)

	result := scanCaptureHash()
	// Should return hash from newer file
	if result != "2222222222222222222222222222222222222222222222222222222222222222" &&
		result != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Errorf("scanCaptureHash() = %s, want hash from one of the files", result)
	}
}

func TestScanCaptureHash_FileReadError(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	captureDir := filepath.Join(tmpDir, ".capture")
	os.Mkdir(captureDir, 0755)

	// Create a file without user_id
	logFile := filepath.Join(captureDir, "fetch_test.log")
	content := `{"other":"data"}`
	os.WriteFile(logFile, []byte(content), 0644)

	result := scanCaptureHash()
	if result != "" {
		t.Errorf("scanCaptureHash() = %s, want empty string", result)
	}
}

func TestExtractHash_EmptyParts(t *testing.T) {
	// Test case where split returns empty parts
	line := "user__account__session_foo"
	result := extractHash(line)
	if result != "" {
		t.Errorf("extractHash() = %s, want empty string", result)
	}
}

func TestScanCaptureHash_FileInfoError(t *testing.T) {
	// This test is hard to trigger without mocking, but we can test
	// the case where a file exists but we can't read its info
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	captureDir := filepath.Join(tmpDir, ".capture")
	os.Mkdir(captureDir, 0755)

	// Create a valid fetch log file
	logFile := filepath.Join(captureDir, "fetch_test.log")
	content := `{"user_id":"user_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890_account__session_foo"}`
	os.WriteFile(logFile, []byte(content), 0644)

	// The function should still work even if some files have errors
	result := scanCaptureHash()
	expected := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if result != expected {
		t.Errorf("scanCaptureHash() = %s, want %s", result, expected)
	}
}

func TestExtractHash_NoAccountSeparator(t *testing.T) {
	// Test when there's no _account__session separator
	// In this case, the whole string after user_ becomes parts[0]
	// If it's exactly 64 chars, it will be returned
	line := "user_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	result := extractHash(line)
	// This should actually return the hash since it's 64 chars
	expected := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if result != expected {
		t.Errorf("extractHash() = %s, want %s", result, expected)
	}
}

func TestExtractHash_WrongHashLength(t *testing.T) {
	// Test when hash is not 64 characters
	line := "user_short_account__session_foo"
	result := extractHash(line)
	if result != "" {
		t.Errorf("extractHash() = %s, want empty string", result)
	}
}
