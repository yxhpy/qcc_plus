package proxy

import (
	"net/http"
	"strings"
)

const (
	defaultOpenAIHealthCheckModel = "gpt-5.1-mini"
	defaultCodexHealthCheckModel  = "gpt-4.1-mini"
	defaultGeminiHealthCheckModel = "gemini-2.5-flash"
	legacyOpenAIHealthCheckModel  = "gpt-5.4"
)

func applyUpstreamAuthHeaders(req *http.Request, sourceProtocol, apiKey string) {
	if req == nil {
		return
	}

	req.Header.Del("x-api-key")
	req.Header.Del("x-goog-api-key")
	req.Header.Del("Authorization")

	if apiKey == "" {
		return
	}

	switch NormalizedSourceProtocol(sourceProtocol) {
	case SourceProtocolOpenAI, SourceProtocolCodex:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	case SourceProtocolGemini:
		req.Header.Set("x-goog-api-key", apiKey)
	default:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func defaultHealthCheckModelForProtocol(sourceProtocol string) string {
	switch NormalizedSourceProtocol(sourceProtocol) {
	case SourceProtocolOpenAI:
		return defaultOpenAIHealthCheckModel
	case SourceProtocolCodex:
		return defaultCodexHealthCheckModel
	case SourceProtocolGemini:
		return defaultGeminiHealthCheckModel
	default:
		return defaultHealthCheckModel
	}
}

func effectiveHealthCheckModelForProtocol(sourceProtocol, configuredModel string) string {
	model := strings.TrimSpace(configuredModel)
	switch NormalizedSourceProtocol(sourceProtocol) {
	case SourceProtocolOpenAI:
		if model == "" || model == defaultHealthCheckModel || model == legacyOpenAIHealthCheckModel {
			return defaultOpenAIHealthCheckModel
		}
	case SourceProtocolCodex:
		if model == "" || model == defaultHealthCheckModel {
			return defaultCodexHealthCheckModel
		}
	case SourceProtocolGemini:
		if model == "" || model == defaultHealthCheckModel {
			return defaultGeminiHealthCheckModel
		}
	default:
		if model == "" {
			return defaultHealthCheckModel
		}
	}
	return model
}
