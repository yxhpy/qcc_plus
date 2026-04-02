package proxy

import (
	"net/http"
	"testing"
)

func TestClassifyError_HTTP400_DefaultToNodeDown(t *testing.T) {
	result := ClassifyError(http.StatusBadRequest, nil)

	if result.Severity != SeverityNodeDown {
		t.Fatalf("severity = %v, want %v", result.Severity, SeverityNodeDown)
	}
	if !result.Retryable {
		t.Fatal("retryable = false, want true")
	}
}

func TestClassifyError_HTTP400_ClientKeywordStaysPermanent(t *testing.T) {
	body := []byte(`{"error":{"type":"invalid_request_error","message":"missing required field: messages"}}`)
	result := ClassifyError(http.StatusBadRequest, body)

	if result.Severity != SeverityPermanent {
		t.Fatalf("severity = %v, want %v", result.Severity, SeverityPermanent)
	}
	if result.Retryable {
		t.Fatal("retryable = true, want false")
	}
}

func TestClassifyError_HTTP400_UnknownProviderErrorToNodeDown(t *testing.T) {
	body := []byte(`{"error":{"type":"packy_api_error","message":"upstream gateway bad request"}}`)
	result := ClassifyError(http.StatusBadRequest, body)

	if result.Severity != SeverityNodeDown {
		t.Fatalf("severity = %v, want %v", result.Severity, SeverityNodeDown)
	}
	if !result.Retryable {
		t.Fatal("retryable = false, want true")
	}
}

func TestValidateHealthCheckResponse_GeminiUsageMetadata(t *testing.T) {
	body := []byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}`)
	result := ValidateHealthCheckResponse(http.StatusOK, body)

	if !result.Success {
		t.Fatalf("success=false, want true, error=%+v", result.Error)
	}
	if !result.HasUsage {
		t.Fatal("hasUsage=false, want true")
	}
	if !result.HasContent {
		t.Fatal("hasContent=false, want true")
	}
	if result.InputTokens != 7 {
		t.Fatalf("inputTokens=%d, want 7", result.InputTokens)
	}
	if result.OutputTokens != 3 {
		t.Fatalf("outputTokens=%d, want 3", result.OutputTokens)
	}
}

func TestValidateHealthCheckResponse_CodexChoicesWithoutUsage(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-123","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	result := ValidateHealthCheckResponse(http.StatusOK, body)

	if !result.Success {
		t.Fatalf("success=false, want true, error=%+v", result.Error)
	}
	if !result.HasContent {
		t.Fatal("hasContent=false, want true")
	}
}

func TestValidateHealthCheckResponse_CodexUsageFields(t *testing.T) {
	body := []byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`)
	result := ValidateHealthCheckResponse(http.StatusOK, body)

	if !result.Success {
		t.Fatalf("success=false, want true, error=%+v", result.Error)
	}
	if !result.HasUsage {
		t.Fatal("hasUsage=false, want true")
	}
	if result.InputTokens != 11 {
		t.Fatalf("inputTokens=%d, want 11", result.InputTokens)
	}
	if result.OutputTokens != 5 {
		t.Fatalf("outputTokens=%d, want 5", result.OutputTokens)
	}
}

func TestValidateHealthCheckResponse_MissingUsageAndContentStillFails(t *testing.T) {
	body := []byte(`{"foo":"bar"}`)
	result := ValidateHealthCheckResponse(http.StatusOK, body)

	if result.Success {
		t.Fatal("success=true, want false")
	}
	if result.Error.Severity != SeverityDegraded {
		t.Fatalf("severity=%v, want %v", result.Error.Severity, SeverityDegraded)
	}
}
