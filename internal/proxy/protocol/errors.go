package protocol

import "strings"

// NormalizedErrorCode is stable across ingress/egress protocols.
type NormalizedErrorCode string

const (
	ErrAuthInvalid         NormalizedErrorCode = "auth_invalid"
	ErrRateLimited         NormalizedErrorCode = "rate_limited"
	ErrModelNotFound       NormalizedErrorCode = "model_not_found"
	ErrContextExceeded     NormalizedErrorCode = "context_exceeded"
	ErrUpstreamUnavailable NormalizedErrorCode = "upstream_unavailable"
	ErrUnsupportedFeature  NormalizedErrorCode = "unsupported_feature"
	ErrBadRequest          NormalizedErrorCode = "bad_request"
	ErrUnknown             NormalizedErrorCode = "unknown_error"
)

// NormalizedError is protocol-neutral error envelope.
type NormalizedError struct {
	Code       NormalizedErrorCode `json:"code"`
	Message    string              `json:"message"`
	HTTPStatus int                 `json:"http_status"`
	Retryable  bool                `json:"retryable"`
	Upstream   string              `json:"upstream,omitempty"`
}

// NormalizeUpstreamError maps generic upstream status/code to a stable code.
func NormalizeUpstreamError(httpStatus int, upstreamCode, message string) NormalizedError {
	c := classifyCode(httpStatus, strings.ToLower(strings.TrimSpace(upstreamCode)), strings.ToLower(message))
	return NormalizedError{
		Code:       c,
		Message:    message,
		HTTPStatus: httpStatus,
		Retryable:  isRetryable(c),
		Upstream:   upstreamCode,
	}
}

func classifyCode(status int, upstreamCode, msg string) NormalizedErrorCode {
	switch {
	case status == 401 || status == 403:
		return ErrAuthInvalid
	case status == 429 || strings.Contains(upstreamCode, "rate") || strings.Contains(msg, "rate limit"):
		return ErrRateLimited
	case status == 404 || strings.Contains(upstreamCode, "model") && strings.Contains(upstreamCode, "not"):
		return ErrModelNotFound
	case strings.Contains(msg, "context") && (strings.Contains(msg, "exceed") || strings.Contains(msg, "length")):
		return ErrContextExceeded
	case status >= 500:
		return ErrUpstreamUnavailable
	case status == 400:
		if strings.Contains(msg, "unsupported") || strings.Contains(msg, "not supported") {
			return ErrUnsupportedFeature
		}
		return ErrBadRequest
	default:
		if strings.Contains(msg, "unsupported") {
			return ErrUnsupportedFeature
		}
		return ErrUnknown
	}
}

func isRetryable(code NormalizedErrorCode) bool {
	switch code {
	case ErrRateLimited, ErrUpstreamUnavailable:
		return true
	default:
		return false
	}
}
