package proxy

import (
	"context"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestNewBuilder(t *testing.T) {
	b := NewBuilder()
	if b == nil {
		t.Fatal("NewBuilder should return non-nil builder")
	}
	if b.listenAddr != ":8000" {
		t.Errorf("default listenAddr = %s, want :8000", b.listenAddr)
	}
	if b.logger == nil {
		t.Error("default logger should not be nil")
	}
	if b.upstreamName != "default" {
		t.Errorf("default upstreamName = %s, want default", b.upstreamName)
	}
	if b.retries != 3 {
		t.Errorf("default retries = %d, want 3", b.retries)
	}
	if b.failLimit != 3 {
		t.Errorf("default failLimit = %d, want 3", b.failLimit)
	}
	if b.healthEvery != 30*time.Second {
		t.Errorf("default healthEvery = %v, want 30s", b.healthEvery)
	}
}

func TestBuilder_WithUpstream(t *testing.T) {
	b := NewBuilder().WithUpstream("https://api.example.com")
	if b.upstreamRaw != "https://api.example.com" {
		t.Errorf("upstreamRaw = %s, want https://api.example.com", b.upstreamRaw)
	}
}

func TestBuilder_WithAPIKey(t *testing.T) {
	b := NewBuilder().WithAPIKey("test-key")
	if b.upstreamKey != "test-key" {
		t.Errorf("upstreamKey = %s, want test-key", b.upstreamKey)
	}
}

func TestBuilder_WithNodeName(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		want     string
	}{
		{"empty name", "", "default"},
		{"custom name", "custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithNodeName(tt.nodeName)
			if b.upstreamName != tt.want {
				t.Errorf("upstreamName = %s, want %s", b.upstreamName, tt.want)
			}
		})
	}
}

func TestBuilder_WithListenAddr(t *testing.T) {
	b := NewBuilder().WithListenAddr(":9000")
	if b.listenAddr != ":9000" {
		t.Errorf("listenAddr = %s, want :9000", b.listenAddr)
	}
}

func TestBuilder_WithTransport(t *testing.T) {
	transport := &http.Transport{}
	b := NewBuilder().WithTransport(transport)
	if b.transport != transport {
		t.Error("transport not set correctly")
	}
}

func TestBuilder_WithEnv(t *testing.T) {
	// Save original env
	orig := os.Getenv("PROXY_HEALTH_CHECK_MODE")
	defer os.Setenv("PROXY_HEALTH_CHECK_MODE", orig)

	os.Setenv("PROXY_HEALTH_CHECK_MODE", "api")
	b := NewBuilder().WithEnv()
	// Just verify it doesn't panic
	if b == nil {
		t.Error("WithEnv should return builder")
	}
}

func TestBuilder_WithCLIRunner(t *testing.T) {
	runner := func(ctx context.Context, image string, env map[string]string, prompt string, model string) (string, error) {
		return "OK", nil
	}
	b := NewBuilder().WithCLIRunner(runner)
	// Just verify it doesn't panic
	if b == nil {
		t.Error("WithCLIRunner should return builder")
	}
}

func TestBuilder_WithFailLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"positive limit", 5, 5},
		{"zero limit", 0, 3}, // Should keep default
		{"negative limit", -1, 3}, // Should keep default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithFailLimit(tt.limit)
			if b.failLimit != tt.want {
				t.Errorf("failLimit = %d, want %d", b.failLimit, tt.want)
			}
		})
	}
}

func TestBuilder_WithHealthEvery(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     time.Duration
	}{
		{"positive duration", 60 * time.Second, 60 * time.Second},
		{"zero duration", 0, 30 * time.Second}, // Should keep default
		{"negative duration", -1 * time.Second, 30 * time.Second}, // Should keep default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithHealthEvery(tt.duration)
			if b.healthEvery != tt.want {
				t.Errorf("healthEvery = %v, want %v", b.healthEvery, tt.want)
			}
		})
	}
}

func TestBuilder_WithHealthAllInterval(t *testing.T) {
	b := NewBuilder().WithHealthAllInterval(15 * time.Minute)
	if b.healthAllInterval != 15*time.Minute {
		t.Errorf("healthAllInterval = %v, want 15m", b.healthAllInterval)
	}
}

func TestBuilder_WithAdminKey(t *testing.T) {
	b := NewBuilder().WithAdminKey("custom-admin-key")
	if b.adminKey != "custom-admin-key" {
		t.Errorf("adminKey = %s, want custom-admin-key", b.adminKey)
	}
}

func TestBuilder_WithDefaultAccountName(t *testing.T) {
	tests := []struct {
		name        string
		accountName string
		want        string
	}{
		{"empty name", "", ""},
		{"custom name", "custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithDefaultAccountName(tt.accountName)
			if b.defaultAccountName != tt.want {
				t.Errorf("defaultAccountName = %s, want %s", b.defaultAccountName, tt.want)
			}
		})
	}
}

func TestBuilder_WithDefaultAccount(t *testing.T) {
	b := NewBuilder().WithDefaultAccount("test-account", "test-key")
	if b.defaultAccountName != "test-account" {
		t.Errorf("defaultAccountName = %s, want test-account", b.defaultAccountName)
	}
	if b.defaultProxyKey != "test-key" {
		t.Errorf("defaultProxyKey = %s, want test-key", b.defaultProxyKey)
	}
}

func TestBuilder_WithStoreDSN(t *testing.T) {
	b := NewBuilder().WithStoreDSN("user:pass@tcp(localhost:3306)/db")
	if b.storeDSN != "user:pass@tcp(localhost:3306)/db" {
		t.Errorf("storeDSN not set correctly")
	}
}

func TestBuilder_WithRetry(t *testing.T) {
	tests := []struct {
		name   string
		retries int
		want   int
	}{
		{"positive retries", 5, 5},
		{"zero retries", 0, 3}, // Should keep default
		{"negative retries", -1, 3}, // Should keep default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder().WithRetry(tt.retries)
			if b.retries != tt.want {
				t.Errorf("retries = %d, want %d", b.retries, tt.want)
			}
		})
	}
}

func TestBuilder_WithLogger(t *testing.T) {
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	b := NewBuilder().WithLogger(logger)
	if b.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestBuilder_Build_MissingUpstream(t *testing.T) {
	b := NewBuilder()
	_, err := b.Build()
	if err != ErrUpstreamMissing {
		t.Errorf("Build() error = %v, want ErrUpstreamMissing", err)
	}
}

func TestBuilder_Build_InvalidUpstream(t *testing.T) {
	b := NewBuilder().WithUpstream("://invalid")
	_, err := b.Build()
	if err == nil {
		t.Error("Build() should return error for invalid upstream")
	}
}

func TestChooseNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first non-empty", []string{"first", "second"}, "first"},
		{"second non-empty", []string{"", "second", "third"}, "second"},
		{"no values", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chooseNonEmpty(tt.vals...)
			if got != tt.want {
				t.Errorf("chooseNonEmpty() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback int
		want     int
	}{
		{"valid int", "TEST_INT", "42", 10, 42},
		{"empty value", "TEST_INT", "", 10, 10},
		{"invalid int", "TEST_INT", "abc", 10, 10},
		{"negative int", "TEST_INT", "-5", 10, 10}, // Negative not allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}
			got := parseEnvInt(tt.key, tt.fallback, nil)
			if got != tt.want {
				t.Errorf("parseEnvInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseEnvDuration(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback time.Duration
		want     time.Duration
	}{
		{"valid duration", "TEST_DUR", "30s", 10 * time.Second, 30 * time.Second},
		{"empty value", "TEST_DUR", "", 10 * time.Second, 10 * time.Second},
		{"invalid duration", "TEST_DUR", "abc", 10 * time.Second, 10 * time.Second},
		{"negative duration", "TEST_DUR", "-5s", 10 * time.Second, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}
			got := parseEnvDuration(tt.key, tt.fallback, nil)
			if got != tt.want {
				t.Errorf("parseEnvDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback bool
		want     bool
	}{
		{"true", "true", false, true},
		{"1", "1", false, true},
		{"yes", "yes", false, true},
		{"on", "on", false, true},
		{"false", "false", true, false},
		{"0", "0", true, false},
		{"no", "no", true, false},
		{"off", "off", true, false},
		{"invalid", "invalid", true, true},
		{"empty", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL"
			if tt.value != "" {
				os.Setenv(key, tt.value)
				defer os.Unsetenv(key)
			}
			got := parseEnvBool(key, tt.fallback, nil)
			if got != tt.want {
				t.Errorf("parseEnvBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDefaultSQLitePath(t *testing.T) {
	// Save original env
	orig := os.Getenv("PROXY_SQLITE_PATH")
	defer os.Setenv("PROXY_SQLITE_PATH", orig)

	tests := []struct {
		name    string
		envPath string
		wantEnv bool
	}{
		{"with env var", "/custom/path/db.sqlite", true},
		{"without env var", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envPath != "" {
				os.Setenv("PROXY_SQLITE_PATH", tt.envPath)
			} else {
				os.Unsetenv("PROXY_SQLITE_PATH")
			}

			got := getDefaultSQLitePath(nil)
			if tt.wantEnv && got != tt.envPath {
				t.Errorf("getDefaultSQLitePath() = %s, want %s", got, tt.envPath)
			}
			if !tt.wantEnv && got == "" {
				t.Error("getDefaultSQLitePath() should return non-empty path")
			}
		})
	}
}

func TestMaskDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "mysql dsn",
			dsn:  "user:password@tcp(localhost:3306)/db",
			want: "***@tcp(localhost:3306)/db",
		},
		{
			name: "no password",
			dsn:  "localhost:3306/db",
			want: "localhost:3306/db",
		},
		{
			name: "empty dsn",
			dsn:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskDSN(tt.dsn)
			if got != tt.want {
				t.Errorf("maskDSN() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestBuildTransportFromEnv(t *testing.T) {
	transport := buildTransportFromEnv(nil)
	if transport == nil {
		t.Fatal("buildTransportFromEnv should return non-nil transport")
	}

	httpTransport, ok := transport.(*http.Transport)
	if !ok {
		t.Fatal("buildTransportFromEnv should return *http.Transport")
	}

	// Verify default values
	if httpTransport.MaxIdleConns != defaultMaxIdleConns {
		t.Errorf("MaxIdleConns = %d, want %d", httpTransport.MaxIdleConns, defaultMaxIdleConns)
	}
	if httpTransport.MaxIdleConnsPerHost != defaultMaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost = %d, want %d", httpTransport.MaxIdleConnsPerHost, defaultMaxIdleConnsPerHost)
	}
}

type mockCLIRunner func(ctx context.Context, image string, env map[string]string, prompt string, model string) (string, error)
