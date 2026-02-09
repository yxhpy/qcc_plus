package proxy

import (
	"encoding/json"
	"fmt"
)

// HealthCheckModelConfig 模型感知健康检查配置
type HealthCheckModelConfig struct {
	// 是否启用模型感知测试（使用实际模型而非固定 haiku）
	ModelAwareEnabled bool
	// 验证响应体中的 usage 字段
	ValidateUsage bool
	// 验证响应体中的 content 字段
	ValidateContent bool
}

func loadHealthCheckModelConfig() HealthCheckModelConfig {
	return HealthCheckModelConfig{
		ModelAwareEnabled: GetEnvBool("HEALTH_MODEL_AWARE", false),
		ValidateUsage:     GetEnvBool("HEALTH_VALIDATE_USAGE", true),
		ValidateContent:   GetEnvBool("HEALTH_VALIDATE_CONTENT", false),
	}
}

// ModelFamily 模型族类型
type ModelFamily string

const (
	ModelFamilyClaude  ModelFamily = "claude" // Claude 系列（chat/messages）
	ModelFamilyUnknown ModelFamily = "unknown"
)

// DetectModelFamily 根据模型 ID 检测模型族
func DetectModelFamily(modelID string) ModelFamily {
	if modelID == "" {
		return ModelFamilyClaude // 默认 Claude
	}
	// 目前 qcc_plus 主要代理 Anthropic Claude API
	return ModelFamilyClaude
}

// BuildHealthCheckPayload 根据模型构造合适的健康检查请求体
// 不同模型可能需要不同的请求格式和最小参数
func BuildHealthCheckPayload(model string) ([]byte, error) {
	family := DetectModelFamily(model)

	switch family {
	case ModelFamilyClaude:
		return buildClaudeHealthPayload(model)
	default:
		return buildClaudeHealthPayload(model)
	}
}

// buildClaudeHealthPayload 构造 Claude Messages API 的最小健康检查请求
func buildClaudeHealthPayload(model string) ([]byte, error) {
	if model == "" {
		model = defaultHealthCheckModel
	}

	payload := map[string]interface{}{
		"model":      model,
		"max_tokens": 1,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "hi",
			},
		},
	}

	return json.Marshal(payload)
}

// ChooseHealthCheckModel 选择健康检查使用的模型
// 如果启用了模型感知，使用节点配置的模型；否则使用默认的轻量模型
func ChooseHealthCheckModel(nodeModel string, cfg HealthCheckModelConfig) string {
	if cfg.ModelAwareEnabled && nodeModel != "" {
		return nodeModel
	}
	return defaultHealthCheckModel
}

// FormatHealthCheckError 格式化健康检查错误信息（包含语义化分析）
func FormatHealthCheckError(result HealthCheckResult) string {
	if result.Success {
		return ""
	}

	msg := fmt.Sprintf("status %d", result.StatusCode)
	if result.Error.Code != "" {
		msg += fmt.Sprintf(" [%s]", result.Error.Code)
	}
	if result.Error.Message != "" {
		msg += ": " + result.Error.Message
	}
	if result.Error.Severity != SeverityTransient {
		msg += fmt.Sprintf(" (severity=%s)", result.Error.Severity)
	}
	return msg
}
