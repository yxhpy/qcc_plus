package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatUpstreamErrorDetail_IncludesStatusAndRawJSON(t *testing.T) {
	detail := formatUpstreamErrorDetail(http.StatusServiceUnavailable, ClassifiedError{
		Severity: SeverityNodeDown,
		Code:     "new_api_error",
		Message:  "未知错误类型: new_api_error",
	}, []byte(`{"error":{"type":"new_api_error","message":"relay: service unavailable"}}`), "")

	for _, want := range []string{
		"状态码: 503",
		"级别: node_down",
		"错误码: new_api_error",
		"摘要: 未知错误类型: new_api_error",
		"原始响应:",
		`"type": "new_api_error"`,
	} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail missing %q:\n%s", want, detail)
		}
	}
}

func TestMetricsWriter_CapturesErrorBodyPreview(t *testing.T) {
	rec := httptest.NewRecorder()
	mw := &metricsWriter{ResponseWriter: rec}
	mw.WriteHeader(http.StatusBadGateway)
	if _, err := mw.Write([]byte(`{"error":{"type":"gateway_error","message":"upstream bad gateway"}}`)); err != nil {
		t.Fatalf("write error: %v", err)
	}

	body := string(mw.ErrorBodyPreview())
	if !strings.Contains(body, "gateway_error") {
		t.Fatalf("expected error body preview to include raw response, got %q", body)
	}
}

func TestPreferDetailedError_UsesStructuredFallbackWhenStatusMatches(t *testing.T) {
	current := "status 400"
	fallback := "状态码: 400\n摘要: 请求格式错误\n原始响应:\n{\"error\":{\"message\":\"relay: 账户余额不足\"}}"

	got := preferDetailedError(current, fallback)
	if got != fallback {
		t.Fatalf("expected structured fallback, got %q", got)
	}
}

func TestPreferDetailedError_KeepsCurrentWhenStatusDiffers(t *testing.T) {
	current := "status 400"
	fallback := "状态码: 403\n摘要: 访问被拒绝\n原始响应:\n{\"error\":\"forbidden\"}"

	got := preferDetailedError(current, fallback)
	if got != current {
		t.Fatalf("expected current detail to be preserved, got %q", got)
	}
}
