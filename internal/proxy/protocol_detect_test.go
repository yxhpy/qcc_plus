package proxy

import "testing"

func TestDetectIngressProtocol(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/v1/messages", "claude"},
		{"/openai", "openai"},
		{"/openai/v1/chat/completions", "openai"},
		{"/openai/v1/responses", "openai"},
		{"/v1/chat/completions", "openai"},
		{"/v1/responses", "openai"},
		{"/v1/models", "openai"},
		{"/gemini", "gemini"},
		{"/gemini/v1beta/models/gemini-2.5-flash:generateContent", "gemini"},
		{"/v1beta/models/gemini-2.5-flash:generateContent", "gemini"},
		{"/codex", "codex"},
		{"/codex/v1/responses", "codex"},
		{"/codex/v1/models", "codex"},
		{"/codex/something", "codex"},
	}

	for _, tc := range cases {
		if got := detectIngressProtocol(tc.path); got != tc.want {
			t.Fatalf("path=%s got=%s want=%s", tc.path, got, tc.want)
		}
	}
}

func TestExtractModelFromGeminiPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/gemini/v1beta/models/gemini-2.5-flash:generateContent", "gemini-2.5-flash"},
		{"/v1beta/models/gemini-2.5-pro:generateContent", "gemini-2.5-pro"},
		{"/v1/messages", ""},
	}

	for _, tc := range cases {
		if got := extractModelFromGeminiPath(tc.path); got != tc.want {
			t.Fatalf("path=%s got=%s want=%s", tc.path, got, tc.want)
		}
	}
}

func TestNormalizeCodexIngressPath(t *testing.T) {
	cases := []struct {
		path    string
		want    string
		wantOK  bool
	}{
		{"/codex/v1/responses", "/v1/responses", true},
		{"/codex/v1/models", "/v1/models", true},
		{"/codex/v1/chat/completions", "", false},
		{"/v1/responses", "", false},
		{"/openai/v1/responses", "", false},
	}

	for _, tc := range cases {
		got, ok := normalizeCodexIngressPath(tc.path)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("normalizeCodexIngressPath(%s) = (%s, %v), want (%s, %v)", tc.path, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestRewriteIngressPathForCodex(t *testing.T) {
	cases := []struct {
		original string
		target   string
		model    string
		want     string
	}{
		{"/codex/v1/responses", "codex", "", "/v1/responses"},
		{"/codex/v1/models", "codex", "", "/v1/models"},
		{"/v1/messages", "codex", "", "/v1/responses"},
		{"/codex/v1/responses", "openai", "", "/v1/chat/completions"},
	}

	for _, tc := range cases {
		got := rewriteIngressPathForUpstream(tc.original, tc.target, tc.model)
		if got != tc.want {
			t.Fatalf("rewriteIngressPathForUpstream(%s, %s, %s) = %s, want %s", tc.original, tc.target, tc.model, got, tc.want)
		}
	}
}

func TestNormalizedSourceProtocolCodex(t *testing.T) {
	if got := NormalizedSourceProtocol("codex"); got != "codex" {
		t.Fatalf("NormalizedSourceProtocol(codex) = %s, want codex", got)
	}
}
