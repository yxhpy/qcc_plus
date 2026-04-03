package proxy

import "strings"

const (
	openAIChatCompletionsPath = "/v1/chat/completions"
	openAIResponsesPath       = "/v1/responses"
	openAIModelsPath          = "/v1/models"
	openAIWireAPIChat         = "chat_completions"
	openAIWireAPIResponses    = "responses"
	geminiModelsPrefix        = "/v1beta/models/"
	geminiGenerateSuffix      = ":generateContent"
)

func normalizeOpenAIWireAPI(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "chat/completions", "chat-completions", openAIWireAPIChat:
		return openAIWireAPIChat
	case openAIWireAPIResponses:
		return openAIWireAPIResponses
	default:
		return openAIWireAPIResponses
	}
}

func defaultOpenAIUpstreamPath(wireAPI string) string {
	if normalizeOpenAIWireAPI(wireAPI) == openAIWireAPIChat {
		return openAIChatCompletionsPath
	}
	return openAIResponsesPath
}

func trimProtocolPrefix(path, prefix string) string {
	if path == prefix {
		return "/"
	}
	if strings.HasPrefix(path, prefix+"/") {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}

func normalizeOpenAIIngressPath(path string) (string, bool) {
	trimmed := trimProtocolPrefix(path, "/openai")
	switch trimmed {
	case openAIChatCompletionsPath, openAIResponsesPath, openAIModelsPath:
		return trimmed, true
	default:
		return "", false
	}
}

func normalizeGeminiIngressPath(path string) (string, bool) {
	trimmed := trimProtocolPrefix(path, "/gemini")
	if strings.HasPrefix(trimmed, geminiModelsPrefix) && strings.HasSuffix(trimmed, geminiGenerateSuffix) {
		return trimmed, true
	}
	return "", false
}

func extractModelFromGeminiPath(path string) string {
	normalized, ok := normalizeGeminiIngressPath(path)
	if !ok {
		return ""
	}
	modelPart := strings.TrimPrefix(normalized, geminiModelsPrefix)
	modelPart = strings.TrimSuffix(modelPart, geminiGenerateSuffix)
	return strings.TrimSpace(modelPart)
}

func detectIngressProtocol(path string) string {
	if _, ok := normalizeOpenAIIngressPath(path); ok {
		return SourceProtocolOpenAI
	}
	if _, ok := normalizeGeminiIngressPath(path); ok {
		return SourceProtocolGemini
	}
	if path == "/openai" || strings.HasPrefix(path, "/openai/") {
		return SourceProtocolOpenAI
	}
	if path == "/gemini" || strings.HasPrefix(path, "/gemini/") {
		return SourceProtocolGemini
	}
	return SourceProtocolClaude
}

func rewriteIngressPathForUpstream(originalPath, targetProtocol, requestModelID string) string {
	switch NormalizedSourceProtocol(targetProtocol) {
	case SourceProtocolOpenAI:
		if normalized, ok := normalizeOpenAIIngressPath(originalPath); ok {
			return normalized
		}
		return openAIChatCompletionsPath
	case SourceProtocolGemini:
		if normalized, ok := normalizeGeminiIngressPath(originalPath); ok {
			return normalized
		}
		if strings.TrimSpace(requestModelID) == "" {
			return originalPath
		}
		return geminiModelsPrefix + strings.TrimSpace(requestModelID) + geminiGenerateSuffix
	default:
		return originalPath
	}
}
