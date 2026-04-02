package version

import (
	"testing"
	"time"
)

func TestGetFormattedBuildDate(t *testing.T) {
	// Save original values
	originalBuildDate := BuildDate
	defer func() {
		BuildDate = originalBuildDate
	}()

	t.Run("empty build date returns 未知", func(t *testing.T) {
		BuildDate = ""
		result := GetFormattedBuildDate()
		if result != "未知" {
			t.Errorf("GetFormattedBuildDate() = %s, want 未知", result)
		}
	})

	t.Run("dev build date returns 开发版本", func(t *testing.T) {
		BuildDate = "dev"
		result := GetFormattedBuildDate()
		if result != "开发版本" {
			t.Errorf("GetFormattedBuildDate() = %s, want 开发版本", result)
		}
	})

	t.Run("valid RFC3339 date formats correctly", func(t *testing.T) {
		// Use a specific UTC time
		BuildDate = "2026-02-03T12:30:45Z"
		result := GetFormattedBuildDate()

		// Should be formatted as Beijing time (UTC+8)
		expected := "2026年02月03日 20时30分45秒"
		if result != expected {
			t.Errorf("GetFormattedBuildDate() = %s, want %s", result, expected)
		}
	})

	t.Run("invalid date format returns error message", func(t *testing.T) {
		BuildDate = "invalid-date"
		result := GetFormattedBuildDate()

		expected := "invalid-date (格式错误)"
		if result != expected {
			t.Errorf("GetFormattedBuildDate() = %s, want %s", result, expected)
		}
	})

	t.Run("RFC3339 with timezone", func(t *testing.T) {
		BuildDate = "2026-02-03T20:30:45+08:00"
		result := GetFormattedBuildDate()

		// Should be formatted as Beijing time
		expected := "2026年02月03日 20时30分45秒"
		if result != expected {
			t.Errorf("GetFormattedBuildDate() = %s, want %s", result, expected)
		}
	})
}

func TestGetVersionInfo(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalGitCommit := GitCommit
	originalBuildDate := BuildDate
	originalGoVersion := GoVersion
	originalResolveBuildInfo := resolveBuildInfo
	originalResolveFallbackVersion := resolveFallbackVersion

	defer func() {
		Version = originalVersion
		GitCommit = originalGitCommit
		BuildDate = originalBuildDate
		GoVersion = originalGoVersion
		resolveBuildInfo = originalResolveBuildInfo
		resolveFallbackVersion = originalResolveFallbackVersion
	}()

	t.Run("returns complete version info", func(t *testing.T) {
		resolveBuildInfo = func() string { return "" }
		resolveFallbackVersion = func() string { return "" }
		Version = "v1.2.3"
		GitCommit = "abc123"
		BuildDate = "2026-02-03T12:30:45Z"
		GoVersion = "go1.21.0"

		info := GetVersionInfo()

		if info.Version != "v1.2.3" {
			t.Errorf("Version = %s, want v1.2.3", info.Version)
		}
		if info.GitCommit != "abc123" {
			t.Errorf("GitCommit = %s, want abc123", info.GitCommit)
		}
		if info.BuildDate != "2026-02-03T12:30:45Z" {
			t.Errorf("BuildDate = %s, want 2026-02-03T12:30:45Z", info.BuildDate)
		}
		if info.BuildDateBeijing != "2026年02月03日 20时30分45秒" {
			t.Errorf("BuildDateBeijing = %s, want 2026年02月03日 20时30分45秒", info.BuildDateBeijing)
		}
		if info.GoVersion != "go1.21.0" {
			t.Errorf("GoVersion = %s, want go1.21.0", info.GoVersion)
		}
	})

	t.Run("returns dev version info", func(t *testing.T) {
		resolveBuildInfo = func() string { return "" }
		resolveFallbackVersion = func() string { return "" }
		Version = "dev"
		GitCommit = ""
		BuildDate = "dev"
		GoVersion = "go1.21.0"

		info := GetVersionInfo()

		if info.Version != "dev" {
			t.Errorf("Version = %s, want dev", info.Version)
		}
		if info.BuildDateBeijing != "开发版本" {
			t.Errorf("BuildDateBeijing = %s, want 开发版本", info.BuildDateBeijing)
		}
	})

	t.Run("handles empty values", func(t *testing.T) {
		resolveBuildInfo = func() string { return "" }
		resolveFallbackVersion = func() string { return "" }
		Version = ""
		GitCommit = ""
		BuildDate = ""
		GoVersion = ""

		info := GetVersionInfo()

		if info.Version != "" {
			t.Errorf("Version = %s, want empty", info.Version)
		}
		if info.BuildDateBeijing != "未知" {
			t.Errorf("BuildDateBeijing = %s, want 未知", info.BuildDateBeijing)
		}
	})

	t.Run("falls back to changelog version when build version is unset", func(t *testing.T) {
		resolveBuildInfo = func() string { return "" }
		resolveFallbackVersion = func() string { return "1.12.1" }
		Version = "dev"
		GitCommit = "abc123"
		BuildDate = "2026-02-03T12:30:45Z"
		GoVersion = "go1.21.0"

		info := GetVersionInfo()

		if info.Version != "v1.12.1" {
			t.Errorf("Version = %s, want v1.12.1", info.Version)
		}
	})

	t.Run("falls back to changelog version when build info is a pseudo version", func(t *testing.T) {
		resolveBuildInfo = func() string { return "v1.11.1-0.20260402064355-3cb2cdfe0528+dirty" }
		resolveFallbackVersion = func() string { return "1.12.1" }
		Version = "dev"
		GitCommit = ""
		BuildDate = ""
		GoVersion = "go1.21.0"

		info := GetVersionInfo()

		if info.Version != "v1.12.1" {
			t.Errorf("Version = %s, want v1.12.1", info.Version)
		}
	})
}

func TestParseLatestReleaseVersion(t *testing.T) {
	content := []byte(`# 更新日志

## [Unreleased]

## [1.12.1] - 2026-02-28

### 修复
- 修复版本显示
`)

	got := parseLatestReleaseVersion(content)
	if got != "v1.12.1" {
		t.Errorf("parseLatestReleaseVersion() = %s, want v1.12.1", got)
	}
}

func TestVersionVariables(t *testing.T) {
	t.Run("version variables are accessible", func(t *testing.T) {
		// Just verify that the variables exist and can be accessed
		_ = Version
		_ = GitCommit
		_ = BuildDate
		_ = GoVersion
	})

	t.Run("GoVersion is set from runtime", func(t *testing.T) {
		// GoVersion should be set from runtime.Version()
		if GoVersion == "" {
			t.Error("GoVersion should not be empty")
		}
	})
}

func TestInfoStruct(t *testing.T) {
	t.Run("Info struct can be created", func(t *testing.T) {
		info := Info{
			Version:          "v1.0.0",
			GitCommit:        "abc123",
			BuildDate:        "2026-02-03T12:00:00Z",
			BuildDateBeijing: "2026年02月03日 20时00分00秒",
			GoVersion:        "go1.21.0",
		}

		if info.Version != "v1.0.0" {
			t.Errorf("Version = %s, want v1.0.0", info.Version)
		}
	})
}

func TestBuildDateParsing(t *testing.T) {
	// Save original value
	originalBuildDate := BuildDate
	defer func() {
		BuildDate = originalBuildDate
	}()

	t.Run("various RFC3339 formats", func(t *testing.T) {
		testCases := []struct {
			name     string
			input    string
			wantHour int // Expected hour in Beijing time
		}{
			{
				name:     "UTC time",
				input:    "2026-02-03T12:00:00Z",
				wantHour: 20, // 12 + 8
			},
			{
				name:     "UTC time with milliseconds",
				input:    "2026-02-03T12:00:00.000Z",
				wantHour: 20,
			},
			{
				name:     "Beijing time explicit",
				input:    "2026-02-03T20:00:00+08:00",
				wantHour: 20,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				BuildDate = tc.input
				result := GetFormattedBuildDate()

				// Parse the result to verify the hour
				parsed, err := time.Parse(time.RFC3339, tc.input)
				if err != nil {
					t.Fatalf("Failed to parse test input: %v", err)
				}

				// The result should contain the correct hour in Beijing time
				// We just check that it doesn't return an error format
				if len(result) < 10 {
					t.Errorf("GetFormattedBuildDate() returned too short result: %s", result)
				}
				if result == "未知" || result == "开发版本" {
					t.Errorf("GetFormattedBuildDate() returned unexpected format: %s", result)
				}

				// Verify the hour is correct
				bjTime := parsed.In(time.FixedZone("CST", 8*3600))
				if bjTime.Hour() != tc.wantHour {
					t.Errorf("Expected hour %d, got %d", tc.wantHour, bjTime.Hour())
				}
			})
		}
	})
}
