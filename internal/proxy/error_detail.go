package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const errorBodyPreviewLimit = 4096

var detailStatusPattern = regexp.MustCompile(`(?:状态码:\s*|status\s+|上游错误\s*\()(\d{3})`)

func formatUpstreamErrorDetail(statusCode int, classified ClassifiedError, rawBody []byte, fallbackSummary string) string {
	summary := strings.TrimSpace(classified.Message)
	if summary == "" {
		summary = strings.TrimSpace(fallbackSummary)
	}

	lines := make([]string, 0, 6)
	if statusCode > 0 {
		lines = append(lines, fmt.Sprintf("状态码: %d", statusCode))
	}
	if severity := strings.TrimSpace(classified.Severity.String()); severity != "" && severity != "unknown" {
		lines = append(lines, fmt.Sprintf("级别: %s", severity))
	}
	if code := strings.TrimSpace(classified.Code); code != "" {
		lines = append(lines, fmt.Sprintf("错误码: %s", code))
	}
	if rawType := strings.TrimSpace(classified.RawType); rawType != "" {
		lines = append(lines, fmt.Sprintf("原始类型: %s", rawType))
	}
	if summary != "" {
		lines = append(lines, fmt.Sprintf("摘要: %s", summary))
	}

	rawPreview := formatRawErrorPreview(rawBody)
	if rawPreview != "" {
		lines = append(lines, "原始响应:")
		lines = append(lines, rawPreview)
	}

	if len(lines) == 0 {
		return strings.TrimSpace(fallbackSummary)
	}
	return strings.Join(lines, "\n")
}

func formatRawErrorPreview(rawBody []byte) string {
	trimmed := bytes.TrimSpace(rawBody)
	if len(trimmed) == 0 {
		return ""
	}

	truncated := false
	if len(trimmed) > errorBodyPreviewLimit {
		trimmed = trimmed[:errorBodyPreviewLimit]
		truncated = true
	}

	preview := string(trimmed)
	if json.Valid(trimmed) {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, trimmed, "", "  "); err == nil {
			preview = pretty.String()
		}
	}

	if truncated {
		preview += "\n...(truncated)"
	}
	return preview
}

func hasStructuredErrorDetail(detail string) bool {
	text := strings.TrimSpace(detail)
	if text == "" {
		return false
	}
	return strings.Contains(text, "状态码:") || strings.Contains(text, "原始响应:")
}

func extractStatusCodeFromDetail(detail string) int {
	match := detailStatusPattern.FindStringSubmatch(strings.TrimSpace(detail))
	if len(match) < 2 {
		return 0
	}
	code, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return code
}

func preferDetailedError(current, fallback string) string {
	current = strings.TrimSpace(current)
	fallback = strings.TrimSpace(fallback)

	if current == "" {
		return fallback
	}
	if fallback == "" || hasStructuredErrorDetail(current) || !hasStructuredErrorDetail(fallback) {
		return current
	}

	currentStatus := extractStatusCodeFromDetail(current)
	fallbackStatus := extractStatusCodeFromDetail(fallback)
	if currentStatus != 0 && fallbackStatus != 0 && currentStatus != fallbackStatus {
		return current
	}

	return fallback
}
