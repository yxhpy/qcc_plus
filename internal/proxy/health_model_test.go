package proxy

import "testing"

func TestBuildHealthProbeSpec(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wireAPI    string
		model      string
		wantPath   string
		wantHdrKey string
	}{
		{"claude", "claude", "", "claude-haiku-4-5-20251001", "/v1/messages", "anthropic-version"},
		{"openai responses default", "openai", "", "gpt-4o-mini", "/v1/responses", "Content-Type"},
		{"openai chat completions", "openai", "chat_completions", "gpt-4o-mini", "/v1/chat/completions", "Content-Type"},
		{"gemini default model", "gemini", "", "", "/v1beta/models/gemini-2.5-flash:generateContent", "Content-Type"},
		{"gemini custom model", "gemini", "", "gemini-2.5-pro", "/v1beta/models/gemini-2.5-pro:generateContent", "Content-Type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := BuildHealthProbeSpec(tt.source, tt.wireAPI, tt.model)
			if err != nil {
				t.Fatalf("BuildHealthProbeSpec error: %v", err)
			}
			if spec.Path != tt.wantPath {
				t.Fatalf("path=%s want=%s", spec.Path, tt.wantPath)
			}
			if spec.Headers[tt.wantHdrKey] == "" {
				t.Fatalf("missing header %s", tt.wantHdrKey)
			}
			if len(spec.Body) == 0 {
				t.Fatal("expected non-empty body")
			}
		})
	}
}

func TestNormalizeHealthCheckMethodForProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		method   string
		want     string
	}{
		{"openai keeps head", SourceProtocolOpenAI, HealthCheckMethodHEAD, HealthCheckMethodHEAD},
		{"openai converts cli to api", SourceProtocolOpenAI, HealthCheckMethodCLI, HealthCheckMethodAPI},
		{"gemini forces api", SourceProtocolGemini, HealthCheckMethodHEAD, HealthCheckMethodAPI},
		{"claude keeps cli", SourceProtocolClaude, HealthCheckMethodCLI, HealthCheckMethodCLI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeHealthCheckMethodForProtocol(tt.protocol, tt.method)
			if got != tt.want {
				t.Fatalf("normalizeHealthCheckMethodForProtocol(%q, %q) = %q, want %q", tt.protocol, tt.method, got, tt.want)
			}
		})
	}
}

func TestEffectiveHealthCheckModelForProtocol(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		configured string
		want       string
	}{
		{"claude empty keeps claude default", "claude", "", defaultHealthCheckModel},
		{"openai empty uses openai default", "openai", "", defaultOpenAIHealthCheckModel},
		{"openai legacy claude default maps to openai default", "openai", defaultHealthCheckModel, defaultOpenAIHealthCheckModel},
		{"openai legacy gpt-5.4 default maps to openai default", "openai", legacyOpenAIHealthCheckModel, defaultOpenAIHealthCheckModel},
		{"gemini empty uses gemini default", "gemini", "", defaultGeminiHealthCheckModel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveHealthCheckModelForProtocol(tt.source, tt.configured)
			if got != tt.want {
				t.Fatalf("effectiveHealthCheckModelForProtocol(%q, %q) = %q, want %q", tt.source, tt.configured, got, tt.want)
			}
		})
	}
}

func TestDetectModelFamily(t *testing.T) {
	tests := []struct {
		model string
		want  ModelFamily
	}{
		{"claude-3-opus", ModelFamilyClaude},
		{"claude-haiku-4-5-20251001", ModelFamilyClaude},
		{"", ModelFamilyClaude},
		{"gpt-4o", ModelFamilyOpenAI},
		{"gpt-4o-mini", ModelFamilyOpenAI},
		{"o1-preview", ModelFamilyOpenAI},
		{"o3-mini", ModelFamilyOpenAI},
		{"o4-mini", ModelFamilyOpenAI},
		{"gemini-2.5-flash", ModelFamilyGemini},
		{"gemini-2.5-pro", ModelFamilyGemini},
		{"unknown-model", ModelFamilyClaude},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := DetectModelFamily(tt.model)
			if got != tt.want {
				t.Errorf("DetectModelFamily(%q) = %s, want %s", tt.model, got, tt.want)
			}
		})
	}
}

func TestChooseHealthCheckModel(t *testing.T) {
	tests := []struct {
		name      string
		nodeModel string
		cfg       HealthCheckModelConfig
		want      string
	}{
		{
			name:      "uses node model by default",
			nodeModel: "gpt-5.4",
			cfg:       HealthCheckModelConfig{ModelAwareEnabled: true},
			want:      "gpt-5.4",
		},
		{
			name:      "falls back when model aware disabled",
			nodeModel: "gpt-5.4",
			cfg:       HealthCheckModelConfig{ModelAwareEnabled: false},
			want:      defaultHealthCheckModel,
		},
		{
			name:      "falls back when node model empty",
			nodeModel: "",
			cfg:       HealthCheckModelConfig{ModelAwareEnabled: true},
			want:      defaultHealthCheckModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChooseHealthCheckModel(tt.nodeModel, tt.cfg)
			if got != tt.want {
				t.Fatalf("ChooseHealthCheckModel(%q, %+v) = %q, want %q", tt.nodeModel, tt.cfg, got, tt.want)
			}
		})
	}
}
