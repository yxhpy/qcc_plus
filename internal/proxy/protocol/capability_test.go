package protocol

import "testing"

func TestParseCapabilities(t *testing.T) {
	caps := ParseCapabilities("")
	if !caps.SupportsStream || !caps.SupportsTools {
		t.Fatalf("default caps should enable stream/tools")
	}

	caps = ParseCapabilities(`{"supports_stream":false,"supports_tools":false,"supports_json_mode":true}`)
	if caps.SupportsStream {
		t.Fatalf("supports_stream should be false")
	}
	if caps.SupportsTools {
		t.Fatalf("supports_tools should be false")
	}
	if !caps.SupportsJSONMode {
		t.Fatalf("supports_json_mode should be true")
	}
}

func TestCapabilityGuards(t *testing.T) {
	caps := Capabilities{SupportsStream: false, SupportsTools: false, SupportsJSONMode: false}

	if caps.AllowStream(true) {
		t.Fatalf("stream should be rejected")
	}
	if caps.AllowTools(true) {
		t.Fatalf("tools should be rejected")
	}
	if caps.AllowJSONMode(true) {
		t.Fatalf("json mode should be rejected")
	}
}
