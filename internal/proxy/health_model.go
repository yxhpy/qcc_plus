package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
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
		ModelAwareEnabled: GetEnvBool("HEALTH_MODEL_AWARE", true),
		ValidateUsage:     GetEnvBool("HEALTH_VALIDATE_USAGE", true),
		ValidateContent:   GetEnvBool("HEALTH_VALIDATE_CONTENT", false),
	}
}

// ModelFamily 模型族类型
type ModelFamily string

const (
	ModelFamilyClaude  ModelFamily = "claude"
	ModelFamilyOpenAI  ModelFamily = "openai"
	ModelFamilyGemini  ModelFamily = "gemini"
	ModelFamilyCodex   ModelFamily = "codex"
	ModelFamilyUnknown ModelFamily = "unknown"
)

func DetectModelFamily(modelID string) ModelFamily {
	if modelID == "" {
		return ModelFamilyClaude
	}
	lower := strings.ToLower(modelID)
	if strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o1-") || strings.HasPrefix(lower, "o3-") || strings.HasPrefix(lower, "o4-") {
		return ModelFamilyOpenAI
	}
	if strings.HasPrefix(lower, "gemini-") {
		return ModelFamilyGemini
	}
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
// 默认优先使用节点配置的模型；显式关闭模型感知时才回退到默认轻量模型
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
	return formatUpstreamErrorDetail(result.StatusCode, result.Error, result.RawBody, result.Error.Message)
}

type HealthProbeSpec struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

func BuildHealthProbeSpec(sourceProtocol, model string) (HealthProbeSpec, error) {
	switch sourceProtocol {
	case SourceProtocolCodex:
		effectiveModel := model
		if effectiveModel == "" {
			effectiveModel = "gpt-4.1-mini"
		}
		payload := map[string]any{
			"model":      effectiveModel,
			"input":      "ping",
			"max_output_tokens": 1,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return HealthProbeSpec{}, err
		}
		return HealthProbeSpec{
			Method: "POST",
			Path:   "/v1/responses",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: body,
		}, nil
	case SourceProtocolOpenAI:
		payload := map[string]any{
			"model":      model,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
			"max_tokens": 1,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return HealthProbeSpec{}, err
		}
		return HealthProbeSpec{
			Method: "POST",
			Path:   "/v1/chat/completions",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: body,
		}, nil
	case SourceProtocolGemini:
		effectiveModel := model
		if effectiveModel == "" {
			effectiveModel = "gemini-2.5-flash"
		}
		payload := map[string]any{
			"contents": []map[string]any{{
				"role":  "user",
				"parts": []map[string]string{{"text": "ping"}},
			}},
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return HealthProbeSpec{}, err
		}
		return HealthProbeSpec{
			Method: "POST",
			Path:   fmt.Sprintf("/v1beta/models/%s:generateContent", effectiveModel),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: body,
		}, nil
	default:
		body, err := BuildHealthCheckPayload(model)
		if err != nil {
			return HealthProbeSpec{}, err
		}
		return HealthProbeSpec{
			Method: "POST",
			Path:   "/v1/messages",
			Headers: map[string]string{
				"Content-Type":      "application/json",
				"anthropic-version": "2023-06-01",
			},
			Body: body,
		}, nil
	}
}
