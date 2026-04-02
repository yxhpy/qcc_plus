package protocol

import "encoding/json"

// Capabilities declares per-node protocol features.
type Capabilities struct {
	SupportsStream      bool `json:"supports_stream"`
	SupportsTools       bool `json:"supports_tools"`
	SupportsJSONMode    bool `json:"supports_json_mode"`
	SupportsVisionInput bool `json:"supports_vision_input"`
}

// DefaultCapabilities provides conservative defaults.
func DefaultCapabilities() Capabilities {
	return Capabilities{
		SupportsStream:      true,
		SupportsTools:       true,
		SupportsJSONMode:    false,
		SupportsVisionInput: false,
	}
}

// ParseCapabilities parses JSON and applies defaults.
func ParseCapabilities(raw string) Capabilities {
	caps := DefaultCapabilities()
	if raw == "" {
		return caps
	}
	_ = json.Unmarshal([]byte(raw), &caps)
	return caps
}

func (c Capabilities) AllowStream(stream bool) bool {
	if !stream {
		return true
	}
	return c.SupportsStream
}

func (c Capabilities) AllowTools(hasTools bool) bool {
	if !hasTools {
		return true
	}
	return c.SupportsTools
}

func (c Capabilities) AllowJSONMode(enabled bool) bool {
	if !enabled {
		return true
	}
	return c.SupportsJSONMode
}
