package proxy

import "testing"

func TestDeduplicateV1Prefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/v1/chat/completions", "/v1/chat/completions"},
		{"/v1/v1/chat/completions", "/v1/chat/completions"},
		{"/v1/v1/v1/chat/completions", "/v1/chat/completions"},
		{"/v1/responses", "/v1/responses"},
		{"/v1/v1/responses", "/v1/responses"},
		{"/api/v1/v1/models", "/api/v1/models"},
		{"/", "/"},
		{"", ""},
		{"/v1beta/models", "/v1beta/models"},
	}
	for _, tt := range tests {
		got := deduplicateV1Prefix(tt.input)
		if got != tt.want {
			t.Errorf("deduplicateV1Prefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeOpenAIIngressPath(t *testing.T) {
	tests := []struct {
		path    string
		wantOK  bool
		wantPath string
	}{
		{"/v1/chat/completions", true, "/v1/chat/completions"},
		{"/v1/responses", true, "/v1/responses"},
		{"/v1/responses/compact", true, "/v1/responses/compact"},
		{"/v1/models", true, "/v1/models"},
		{"/openai/v1/chat/completions", true, "/v1/chat/completions"},
		{"/openai/v1/responses/compact", true, "/v1/responses/compact"},
		{"/v1/unknown", false, ""},
		{"/v2/chat/completions", false, ""},
	}
	for _, tt := range tests {
		gotPath, gotOK := normalizeOpenAIIngressPath(tt.path)
		if gotOK != tt.wantOK || gotPath != tt.wantPath {
			t.Errorf("normalizeOpenAIIngressPath(%q) = (%q, %v), want (%q, %v)",
				tt.path, gotPath, gotOK, tt.wantPath, tt.wantOK)
		}
	}
}

func TestRewriteIngressPathForUpstream(t *testing.T) {
	tests := []struct {
		name       string
		origPath   string
		protocol   string
		modelID    string
		want       string
	}{
		{
			name:     "openai responses compact preserved",
			origPath: "/v1/responses/compact",
			protocol: SourceProtocolOpenAI,
			want:     "/v1/responses/compact",
		},
		{
			name:     "openai responses regular preserved",
			origPath: "/v1/responses",
			protocol: SourceProtocolOpenAI,
			want:     "/v1/responses",
		},
		{
			name:     "openai chat completions preserved",
			origPath: "/v1/chat/completions",
			protocol: SourceProtocolOpenAI,
			want:     "/v1/chat/completions",
		},
		{
			name:     "openai fallback to chat completions",
			origPath: "/v1/unknown",
			protocol: SourceProtocolOpenAI,
			want:     "/v1/chat/completions",
		},
		{
			name:     "claude passthrough",
			origPath: "/v1/messages",
			protocol: SourceProtocolClaude,
			want:     "/v1/messages",
		},
		{
			name:     "gemini with model",
			origPath: "/v1beta/models/gemini-pro:generateContent",
			protocol: SourceProtocolGemini,
			modelID:  "gemini-pro",
			want:     "/v1beta/models/gemini-pro:generateContent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteIngressPathForUpstream(tt.origPath, tt.protocol, tt.modelID)
			if got != tt.want {
				t.Errorf("rewriteIngressPathForUpstream(%q, %q, %q) = %q, want %q",
					tt.origPath, tt.protocol, tt.modelID, got, tt.want)
			}
		})
	}
}

func TestIsResponsesCompactPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1/responses/compact", true},
		{"/v1/responses/compact/123", true},
		{"/v1/responses", false},
		{"/v1/chat/completions", false},
	}
	for _, tt := range tests {
		got := isResponsesCompactPath(tt.path)
		if got != tt.want {
			t.Errorf("isResponsesCompactPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestJoinUpstreamPath_DedupV1(t *testing.T) {
	tests := []struct {
		name string
		base string
		req  string
		want string
	}{
		{
			name: "base v1 + req v1 dedup",
			base: "/api/v1",
			req:  "/v1/chat/completions",
			want: "/api/v1/chat/completions",
		},
		{
			name: "no dedup needed",
			base: "/api",
			req:  "/v1/chat/completions",
			want: "/api/v1/chat/completions",
		},
		{
			name: "responses compact path",
			base: "/api/v1",
			req:  "/v1/responses/compact",
			want: "/api/v1/responses/compact",
		},
		{
			name: "empty base",
			base: "",
			req:  "/v1/responses/compact",
			want: "/v1/responses/compact",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinUpstreamPath(tt.base, tt.req)
			if got != tt.want {
				t.Errorf("joinUpstreamPath(%q, %q) = %q, want %q", tt.base, tt.req, got, tt.want)
			}
		})
	}
}
