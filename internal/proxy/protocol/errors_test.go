package protocol

import "testing"

func TestNormalizeUpstreamError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		upCode    string
		msg       string
		wantCode  NormalizedErrorCode
		wantRetry bool
	}{
		{"auth", 401, "authentication_error", "bad key", ErrAuthInvalid, false},
		{"rate", 429, "rate_limit_error", "rate limit hit", ErrRateLimited, true},
		{"model", 404, "model_not_found", "model not found", ErrModelNotFound, false},
		{"ctx", 400, "invalid_request", "context length exceeded", ErrContextExceeded, false},
		{"upstream", 503, "overloaded", "service unavailable", ErrUpstreamUnavailable, true},
		{"unsupported", 400, "invalid_request", "feature not supported", ErrUnsupportedFeature, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeUpstreamError(tt.status, tt.upCode, tt.msg)
			if got.Code != tt.wantCode {
				t.Fatalf("code=%s want=%s", got.Code, tt.wantCode)
			}
			if got.Retryable != tt.wantRetry {
				t.Fatalf("retryable=%v want=%v", got.Retryable, tt.wantRetry)
			}
		})
	}
}
