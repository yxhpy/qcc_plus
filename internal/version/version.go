package version

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"qcc_plus/internal/timeutil"
)

// Version information injected at build time via -ldflags.
var (
	Version   = "dev"
	GitCommit = ""
	BuildDate = ""
	GoVersion = runtime.Version()
)

var (
	semverPattern          = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
	changelogReleaseRegexp = regexp.MustCompile(`(?m)^## \[(v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\]`)
	goPseudoVersionRegexp  = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?\.\d{14}-[0-9a-f]{12}(?:\+dirty)?$`)
	gitDescribeRegexp      = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?-\d+-g[0-9a-f]{7,}(?:-dirty)?$`)
	resolveBuildInfo       = buildInfoVersion
	resolveFallbackVersion = changelogVersionFallback
	changelogVersionOnce   sync.Once
	changelogVersion       string
)

// Info represents build and runtime version metadata.
type Info struct {
	Version          string `json:"version"`
	GitCommit        string `json:"git_commit"`
	BuildDate        string `json:"build_date"`
	BuildDateBeijing string `json:"build_date_beijing"`
	GoVersion        string `json:"go_version"`
}

// GetFormattedBuildDate returns the build time formatted in Beijing time.
// BuildDate is expected to be an RFC3339 string in UTC set at build time.
func GetFormattedBuildDate() string {
	switch BuildDate {
	case "":
		return "未知"
	case "dev":
		return "开发版本"
	}

	t, err := time.Parse(time.RFC3339, BuildDate)
	if err != nil {
		return BuildDate + " (格式错误)"
	}

	return timeutil.FormatBeijingTime(t)
}

func normalizeVersionString(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "v") {
		return trimmed
	}
	if semverPattern.MatchString(trimmed) {
		return "v" + trimmed
	}
	return trimmed
}

func isUnsetVersion(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "dev", "(devel)", "unknown":
		return true
	default:
		trimmed := strings.TrimSpace(v)
		return goPseudoVersionRegexp.MatchString(trimmed) || gitDescribeRegexp.MatchString(trimmed)
	}
}

func buildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return ""
	}
	return info.Main.Version
}

func changelogVersionFallback() string {
	changelogVersionOnce.Do(func() {
		paths := []string{"CHANGELOG.md", "/app/CHANGELOG.md"}
		if exe, err := os.Executable(); err == nil {
			paths = append(paths, filepath.Join(filepath.Dir(exe), "CHANGELOG.md"))
		}

		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if version := parseLatestReleaseVersion(content); version != "" {
				changelogVersion = version
				return
			}
		}
	})
	return changelogVersion
}

func parseLatestReleaseVersion(content []byte) string {
	match := changelogReleaseRegexp.FindSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	return normalizeVersionString(string(match[1]))
}

func resolvedVersion() string {
	if normalized := normalizeVersionString(Version); !isUnsetVersion(normalized) {
		return normalized
	}
	if buildInfoVersion := normalizeVersionString(resolveBuildInfo()); !isUnsetVersion(buildInfoVersion) {
		return buildInfoVersion
	}
	if fallbackVersion := normalizeVersionString(resolveFallbackVersion()); fallbackVersion != "" {
		return fallbackVersion
	}
	return strings.TrimSpace(Version)
}

// GetVersionInfo returns the current version metadata.
func GetVersionInfo() Info {
	return Info{
		Version:          resolvedVersion(),
		GitCommit:        GitCommit,
		BuildDate:        BuildDate,
		BuildDateBeijing: GetFormattedBuildDate(),
		GoVersion:        GoVersion,
	}
}
