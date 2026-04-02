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
