package proxy

import (
	"os"
	"strings"
	"testing"
)

func TestGetEnvVarCategories(t *testing.T) {
	categories := GetEnvVarCategories()
	if len(categories) == 0 {
		t.Error("GetEnvVarCategories should return non-empty slice")
	}

	// Verify all expected categories are present
	expectedCategories := []EnvVarCategory{
		EnvCategoryBasic,
		EnvCategoryMultiTenant,
		EnvCategoryCLI,
		EnvCategoryHealth,
		EnvCategoryWarmup,
		EnvCategoryRetry,
		EnvCategoryTransport,
		EnvCategoryCircuit,
		EnvCategoryMetrics,
		EnvCategoryMySQL,
		EnvCategoryTunnel,
	}

	found := make(map[EnvVarCategory]bool)
	for _, cat := range categories {
		found[cat.Key] = true
		if cat.Label == "" {
			t.Errorf("Category %s has empty label", cat.Key)
		}
		if cat.Description == "" {
			t.Errorf("Category %s has empty description", cat.Key)
		}
	}

	for _, expected := range expectedCategories {
		if !found[expected] {
			t.Errorf("Missing category: %s", expected)
		}
	}
}

func TestGetAllEnvVarDefinitions(t *testing.T) {
	// Save original env
	origKey := os.Getenv("UPSTREAM_API_KEY")
	defer os.Setenv("UPSTREAM_API_KEY", origKey)

	// Set a test env var
	os.Setenv("UPSTREAM_API_KEY", "test-key-12345678")

	definitions := GetAllEnvVarDefinitions()
	if len(definitions) == 0 {
		t.Fatal("GetAllEnvVarDefinitions should return non-empty slice")
	}

	// Verify structure of definitions
	for _, def := range definitions {
		if def.Name == "" {
			t.Error("Definition has empty name")
		}
		if def.Category == "" {
			t.Errorf("Definition %s has empty category", def.Name)
		}
		if def.Description == "" {
			t.Errorf("Definition %s has empty description", def.Name)
		}

		// Verify secret masking
		if def.IsSecret {
			if def.IsSet && !strings.Contains(def.CurrentValue, "****") && def.CurrentValue != "(未设置)" {
				t.Errorf("Secret %s should be masked, got: %s", def.Name, def.CurrentValue)
			}
			if def.DefaultValue != "" && def.DefaultValue != "********" {
				t.Errorf("Secret %s default value should be masked, got: %s", def.Name, def.DefaultValue)
			}
		}
	}

	// Verify specific env vars exist
	expectedVars := []string{
		"LISTEN_ADDR",
		"UPSTREAM_BASE_URL",
		"UPSTREAM_API_KEY",
		"ADMIN_API_KEY",
		"PROXY_RETRY_MAX",
		"PROXY_HEALTH_CHECK_MODE",
	}

	found := make(map[string]bool)
	for _, def := range definitions {
		found[def.Name] = true
	}

	for _, expected := range expectedVars {
		if !found[expected] {
			t.Errorf("Missing env var definition: %s", expected)
		}
	}
}

func TestGetEnvVarsByCategory(t *testing.T) {
	tests := []struct {
		name     string
		category EnvVarCategory
		minCount int
	}{
		{"basic", EnvCategoryBasic, 1},
		{"multi-tenant", EnvCategoryMultiTenant, 1},
		{"cli", EnvCategoryCLI, 1},
		{"health", EnvCategoryHealth, 1},
		{"warmup", EnvCategoryWarmup, 1},
		{"retry", EnvCategoryRetry, 1},
		{"transport", EnvCategoryTransport, 1},
		{"circuit", EnvCategoryCircuit, 1},
		{"metrics", EnvCategoryMetrics, 1},
		{"mysql", EnvCategoryMySQL, 1},
		{"tunnel", EnvCategoryTunnel, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := GetEnvVarsByCategory(tt.category)
			if len(vars) < tt.minCount {
				t.Errorf("GetEnvVarsByCategory(%s) returned %d vars, want at least %d", tt.category, len(vars), tt.minCount)
			}

			// Verify all returned vars have correct category
			for _, v := range vars {
				if v.Category != tt.category {
					t.Errorf("Variable %s has category %s, want %s", v.Name, v.Category, tt.category)
				}
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "short value",
			value: "short",
			want:  "********",
		},
		{
			name:  "8 chars",
			value: "12345678",
			want:  "********",
		},
		{
			name:  "long value",
			value: "sk-ant-api03-1234567890abcdef",
			want:  "sk-a****cdef",
		},
		{
			name:  "empty value",
			value: "",
			want:  "********",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecret(tt.value)
			if got != tt.want {
				t.Errorf("maskSecret(%s) = %s, want %s", tt.value, got, tt.want)
			}
		})
	}
}

func TestGetEnvString(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		defaultVal string
		want       string
	}{
		{
			name:       "env set",
			key:        "TEST_STRING",
			value:      "test-value",
			defaultVal: "default",
			want:       "test-value",
		},
		{
			name:       "env not set",
			key:        "TEST_STRING_MISSING",
			value:      "",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "empty env value",
			key:        "TEST_STRING_EMPTY",
			value:      "",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}

			got := GetEnvString(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetEnvString() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		defaultVal int
		want       int
	}{
		{
			name:       "valid int",
			key:        "TEST_INT",
			value:      "42",
			defaultVal: 10,
			want:       42,
		},
		{
			name:       "invalid int",
			key:        "TEST_INT_INVALID",
			value:      "abc",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "env not set",
			key:        "TEST_INT_MISSING",
			value:      "",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "negative int",
			key:        "TEST_INT_NEG",
			value:      "-5",
			defaultVal: 10,
			want:       -5, // GetEnvInt allows negative values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}

			got := GetEnvInt(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetEnvInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		defaultVal bool
		want       bool
	}{
		{
			name:       "true",
			key:        "TEST_BOOL",
			value:      "true",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "1",
			key:        "TEST_BOOL",
			value:      "1",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "yes",
			key:        "TEST_BOOL",
			value:      "yes",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "false",
			key:        "TEST_BOOL",
			value:      "false",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "0",
			key:        "TEST_BOOL",
			value:      "0",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "invalid",
			key:        "TEST_BOOL",
			value:      "invalid",
			defaultVal: true,
			want:       false, // GetEnvBool returns false for invalid values
		},
		{
			name:       "env not set",
			key:        "TEST_BOOL_MISSING",
			value:      "",
			defaultVal: true,
			want:       true,
		},
		{
			name:       "uppercase TRUE",
			key:        "TEST_BOOL",
			value:      "TRUE",
			defaultVal: false,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			got := GetEnvBool(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetEnvBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnvVarDefinition_SecretHandling(t *testing.T) {
	// Save original env
	origKey := os.Getenv("ADMIN_API_KEY")
	defer os.Setenv("ADMIN_API_KEY", origKey)

	// Test with secret env var set
	os.Setenv("ADMIN_API_KEY", "super-secret-key-12345678")

	definitions := GetAllEnvVarDefinitions()

	var adminKeyDef *EnvVarDefinition
	for i := range definitions {
		if definitions[i].Name == "ADMIN_API_KEY" {
			adminKeyDef = &definitions[i]
			break
		}
	}

	if adminKeyDef == nil {
		t.Fatal("ADMIN_API_KEY definition not found")
	}

	if !adminKeyDef.IsSecret {
		t.Error("ADMIN_API_KEY should be marked as secret")
	}

	if !adminKeyDef.IsSet {
		t.Error("ADMIN_API_KEY should be marked as set")
	}

	if !strings.Contains(adminKeyDef.CurrentValue, "****") {
		t.Errorf("ADMIN_API_KEY current value should be masked, got: %s", adminKeyDef.CurrentValue)
	}

	// Test with secret env var not set
	os.Unsetenv("ADMIN_API_KEY")
	definitions = GetAllEnvVarDefinitions()

	for i := range definitions {
		if definitions[i].Name == "ADMIN_API_KEY" {
			adminKeyDef = &definitions[i]
			break
		}
	}

	if adminKeyDef.IsSet {
		t.Error("ADMIN_API_KEY should not be marked as set")
	}

	if adminKeyDef.CurrentValue != "(未设置)" {
		t.Errorf("ADMIN_API_KEY current value should be '(未设置)', got: %s", adminKeyDef.CurrentValue)
	}
}

func TestEnvVarCategories_Completeness(t *testing.T) {
	// Verify all definitions have a valid category
	definitions := GetAllEnvVarDefinitions()
	categories := GetEnvVarCategories()

	validCategories := make(map[EnvVarCategory]bool)
	for _, cat := range categories {
		validCategories[cat.Key] = true
	}

	for _, def := range definitions {
		if !validCategories[def.Category] {
			t.Errorf("Definition %s has invalid category: %s", def.Name, def.Category)
		}
	}
}
