package proxy

import "qcc_plus/internal/proxy/protocol"

const (
	SourceProtocolClaude = "claude"
	SourceProtocolOpenAI = "openai"
	SourceProtocolGemini = "gemini"
)

// NormalizedSourceProtocol returns a safe source protocol value.
func NormalizedSourceProtocol(raw string) string {
	switch raw {
	case SourceProtocolOpenAI, SourceProtocolGemini:
		return raw
	default:
		return SourceProtocolClaude
	}
}

func normalizeNodeWireAPI(sourceProtocol, raw string) string {
	if NormalizedSourceProtocol(sourceProtocol) != SourceProtocolOpenAI {
		return ""
	}
	return normalizeOpenAIWireAPI(raw)
}

// ParsedCapabilities decodes node capability JSON with defaults.
func (n *Node) ParsedCapabilities() protocol.Capabilities {
	if n == nil {
		return protocol.DefaultCapabilities()
	}
	return protocol.ParseCapabilities(n.Capabilities)
}
