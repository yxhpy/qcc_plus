package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorSeverity 错误严重程度，决定如何处理节点
type ErrorSeverity int

const (
	// SeverityTransient 临时错误：可重试，不影响节点状态（如 429 限流、503 过载）
	SeverityTransient ErrorSeverity = iota
	// SeverityDegraded 降级错误：节点可用但性能下降（如响应慢、部分功能不可用）
	SeverityDegraded
	// SeverityNodeDown 节点宕机：连接失败、超时等（可恢复）
	SeverityNodeDown
	// SeverityKeyInvalid API Key 失效：需要切换密钥或永久标记（如 401、invalid_api_key）
	SeverityKeyInvalid
	// SeverityAccountIssue 账号问题：余额不足、账号被封等（需人工介入）
	SeverityAccountIssue
	// SeverityPermanent 永久错误：不应重试（如 400 请求格式错误）
	SeverityPermanent
)

func (s ErrorSeverity) String() string {
	switch s {
	case SeverityTransient:
		return "transient"
	case SeverityDegraded:
		return "degraded"
	case SeverityNodeDown:
		return "node_down"
	case SeverityKeyInvalid:
		return "key_invalid"
	case SeverityAccountIssue:
		return "account_issue"
	case SeverityPermanent:
		return "permanent"
	default:
		return "unknown"
	}
}

// ShouldRetry 是否应该重试（换节点）
func (s ErrorSeverity) ShouldRetry() bool {
	return s == SeverityTransient || s == SeverityNodeDown || s == SeverityKeyInvalid
}

// ShouldSwitchKey 是否应该切换 API Key
func (s ErrorSeverity) ShouldSwitchKey() bool {
	return s == SeverityKeyInvalid
}

// ShouldMarkFailed 是否应该标记节点失败
func (s ErrorSeverity) ShouldMarkFailed() bool {
	return s == SeverityNodeDown
}

// ShouldDisableKey 是否应该禁用当前 Key
func (s ErrorSeverity) ShouldDisableKey() bool {
	return s == SeverityKeyInvalid || s == SeverityAccountIssue
}

// ClassifiedError 分类后的错误信息
type ClassifiedError struct {
	Severity   ErrorSeverity `json:"severity"`
	Code       string        `json:"code"`        // 原始错误码（如 "invalid_api_key"）
	Message    string        `json:"message"`     // 人类可读的错误描述
	HTTPStatus int           `json:"http_status"` // HTTP 状态码
	RawType    string        `json:"raw_type"`    // 原始错误类型
	Retryable  bool          `json:"retryable"`   // 是否可重试
	KeyRelated bool          `json:"key_related"` // 是否与 API Key 相关
}

// anthropicError Anthropic API 错误响应结构
type anthropicError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ClassifyError 根据 HTTP 状态码和响应体分析错误类型
func ClassifyError(statusCode int, body []byte) ClassifiedError {
	result := ClassifiedError{
		HTTPStatus: statusCode,
		Retryable:  false,
	}

	// 先尝试解析 Anthropic 错误响应
	var apiErr anthropicError
	if len(body) > 0 {
		_ = json.Unmarshal(body, &apiErr)
		result.Code = apiErr.Error.Type
		result.Message = apiErr.Error.Message
		result.RawType = apiErr.Type
	}

	// 基于错误类型精确分类
	if result.Code != "" {
		classified := classifyByErrorCode(result.Code, result.Message)
		result.Severity = classified.Severity
		result.Retryable = classified.Severity.ShouldRetry()
		result.KeyRelated = classified.KeyRelated
		if classified.Message != "" {
			result.Message = classified.Message
		}
		return result
	}

	// 基于 HTTP 状态码分类
	result = classifyByHTTPStatus(statusCode, result)

	// 基于响应体关键词补充分析
	if len(body) > 0 {
		result = classifyByBodyKeywords(string(body), result)
	}

	return result
}

// classifyByErrorCode 根据 Anthropic API 错误码精确分类
func classifyByErrorCode(code, message string) ClassifiedError {
	code = strings.ToLower(code)
	msg := strings.ToLower(message)

	switch code {
	// === Key/Auth 相关 ===
	case "authentication_error", "invalid_api_key":
		return ClassifiedError{
			Severity:   SeverityKeyInvalid,
			KeyRelated: true,
			Message:    "API Key 无效或已过期",
		}
	case "permission_error":
		if strings.Contains(msg, "key") || strings.Contains(msg, "token") {
			return ClassifiedError{
				Severity:   SeverityKeyInvalid,
				KeyRelated: true,
				Message:    "API Key 权限不足",
			}
		}
		return ClassifiedError{
			Severity:   SeverityAccountIssue,
			KeyRelated: false,
			Message:    "权限不足",
		}

	// === 账号/计费相关 ===
	case "billing_error", "billing_not_active":
		return ClassifiedError{
			Severity:   SeverityAccountIssue,
			KeyRelated: true,
			Message:    "账号计费未激活",
		}
	case "insufficient_quota", "rate_limit_error":
		if strings.Contains(msg, "quota") || strings.Contains(msg, "credit") {
			return ClassifiedError{
				Severity:   SeverityAccountIssue,
				KeyRelated: true,
				Message:    "账号额度不足",
			}
		}
		// 普通限流是临时的
		return ClassifiedError{
			Severity:   SeverityTransient,
			KeyRelated: false,
			Message:    "请求被限流",
		}

	// === 请求格式错误 ===
	case "invalid_request_error":
		if strings.Contains(msg, "model") {
			return ClassifiedError{
				Severity:   SeverityPermanent,
				KeyRelated: false,
				Message:    "模型不可用或不存在",
			}
		}
		return ClassifiedError{
			Severity:   SeverityPermanent,
			KeyRelated: false,
			Message:    "请求格式错误",
		}
	case "not_found_error":
		return ClassifiedError{
			Severity:   SeverityPermanent,
			KeyRelated: false,
			Message:    "请求的资源不存在",
		}

	// === 服务端错误 ===
	case "overloaded_error":
		return ClassifiedError{
			Severity:   SeverityTransient,
			KeyRelated: false,
			Message:    "服务过载，稍后重试",
		}
	case "api_error", "internal_server_error":
		return ClassifiedError{
			Severity:   SeverityNodeDown,
			KeyRelated: false,
			Message:    "上游服务内部错误",
		}

	default:
		return ClassifiedError{
			Severity:   SeverityNodeDown,
			KeyRelated: false,
			Message:    "未知错误类型: " + code,
		}
	}
}

// classifyByHTTPStatus 根据 HTTP 状态码分类
func classifyByHTTPStatus(status int, result ClassifiedError) ClassifiedError {
	switch {
	case status == http.StatusUnauthorized: // 401
		result.Severity = SeverityKeyInvalid
		result.KeyRelated = true
		result.Retryable = true
		if result.Message == "" {
			result.Message = "认证失败 (401)"
		}
	case status == http.StatusForbidden: // 403
		result.Severity = SeverityKeyInvalid
		result.KeyRelated = true
		result.Retryable = true
		if result.Message == "" {
			result.Message = "访问被拒绝 (403)"
		}
	case status == http.StatusTooManyRequests: // 429
		result.Severity = SeverityTransient
		result.Retryable = true
		if result.Message == "" {
			result.Message = "请求被限流 (429)"
		}
	case status == http.StatusBadRequest: // 400
		result.Severity = SeverityPermanent
		result.Retryable = false
		if result.Message == "" {
			result.Message = "请求格式错误 (400)"
		}
	case status == http.StatusNotFound: // 404
		result.Severity = SeverityPermanent
		result.Retryable = false
		if result.Message == "" {
			result.Message = "资源不存在 (404)"
		}
	case status == http.StatusBadGateway: // 502
		result.Severity = SeverityNodeDown
		result.Retryable = true
		if result.Message == "" {
			result.Message = "上游网关错误 (502)"
		}
	case status == http.StatusServiceUnavailable: // 503
		result.Severity = SeverityTransient
		result.Retryable = true
		if result.Message == "" {
			result.Message = "服务暂时不可用 (503)"
		}
	case status == http.StatusGatewayTimeout: // 504
		result.Severity = SeverityNodeDown
		result.Retryable = true
		if result.Message == "" {
			result.Message = "上游响应超时 (504)"
		}
	case status >= 500:
		result.Severity = SeverityNodeDown
		result.Retryable = true
		if result.Message == "" {
			result.Message = "上游服务错误"
		}
	case status >= 400:
		result.Severity = SeverityPermanent
		result.Retryable = false
		if result.Message == "" {
			result.Message = "客户端错误"
		}
	default:
		result.Severity = SeverityTransient
		result.Retryable = false
	}
	return result
}

// classifyByBodyKeywords 根据响应体关键词补充分析
func classifyByBodyKeywords(body string, result ClassifiedError) ClassifiedError {
	lower := strings.ToLower(body)

	// Key 相关关键词
	keyKeywords := []string{
		"invalid_api_key", "invalid api key", "invalid key",
		"expired key", "expired_key", "key expired",
		"unauthorized", "authentication_error",
		"invalid x-api-key", "invalid authorization",
	}
	for _, kw := range keyKeywords {
		if strings.Contains(lower, kw) {
			result.Severity = SeverityKeyInvalid
			result.KeyRelated = true
			result.Retryable = true
			return result
		}
	}

	// 账号/计费关键词
	billingKeywords := []string{
		"billing_not_active", "billing not active",
		"insufficient_quota", "insufficient quota",
		"credit", "balance", "payment",
		"account_suspended", "account suspended",
		"organization_suspended",
	}
	for _, kw := range billingKeywords {
		if strings.Contains(lower, kw) {
			result.Severity = SeverityAccountIssue
			result.KeyRelated = true
			result.Retryable = false
			return result
		}
	}

	// 过载/限流关键词
	overloadKeywords := []string{
		"overloaded", "rate_limit", "rate limit",
		"too many requests", "capacity",
	}
	for _, kw := range overloadKeywords {
		if strings.Contains(lower, kw) {
			result.Severity = SeverityTransient
			result.Retryable = true
			return result
		}
	}

	return result
}

// HealthCheckResult 健康检查的深度验证结果
type HealthCheckResult struct {
	Success      bool            `json:"success"`
	StatusCode   int             `json:"status_code"`
	Error        ClassifiedError `json:"error,omitempty"`
	InputTokens  int64           `json:"input_tokens,omitempty"`
	OutputTokens int64           `json:"output_tokens,omitempty"`
	ModelID      string          `json:"model_id,omitempty"`
	ResponseMs   int64           `json:"response_ms"`
	HasUsage     bool            `json:"has_usage"`   // 响应中是否包含 usage 字段
	HasContent   bool            `json:"has_content"` // 响应中是否包含有效内容
}

// ValidateHealthCheckResponse 深度验证健康检查响应
// 不仅检查 HTTP 状态码，还验证响应体中的 usage 和 content 字段
func ValidateHealthCheckResponse(statusCode int, body []byte) HealthCheckResult {
	result := HealthCheckResult{
		StatusCode: statusCode,
	}

	if statusCode != http.StatusOK {
		result.Success = false
		result.Error = ClassifyError(statusCode, body)
		return result
	}

	// 200 OK - 深度验证响应体
	if len(body) == 0 {
		result.Success = false
		result.Error = ClassifiedError{
			Severity: SeverityNodeDown,
			Message:  "响应体为空",
		}
		return result
	}

	// 解析响应体
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		// 可能是 SSE 流式响应，尝试从中提取 usage
		if in, out := parseUsage(body); in > 0 || out > 0 {
			result.Success = true
			result.HasUsage = true
			result.InputTokens = in
			result.OutputTokens = out
			return result
		}
		// 非 JSON 也非 SSE，但状态码是 200，视为部分成功
		result.Success = true
		result.HasContent = len(body) > 0
		return result
	}

	// 检查 content 字段
	if content, ok := resp["content"]; ok {
		if arr, ok := content.([]interface{}); ok && len(arr) > 0 {
			result.HasContent = true
		}
	}

	// 检查 usage 字段
	if usageObj, ok := resp["usage"]; ok {
		if usageMap, ok := usageObj.(map[string]interface{}); ok {
			result.HasUsage = true
			if v, ok := usageMap["input_tokens"].(float64); ok {
				result.InputTokens = int64(v)
			}
			if v, ok := usageMap["output_tokens"].(float64); ok {
				result.OutputTokens = int64(v)
			}
		}
	}

	// 检查 model 字段
	if model, ok := resp["model"].(string); ok {
		result.ModelID = model
	}

	// 判定成功条件：有 usage 或有 content
	result.Success = result.HasUsage || result.HasContent

	if !result.Success {
		result.Error = ClassifiedError{
			Severity: SeverityDegraded,
			Message:  "响应缺少 usage 和 content 字段",
		}
	}

	return result
}
