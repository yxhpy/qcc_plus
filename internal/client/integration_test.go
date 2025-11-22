//go:build integration
// +build integration

package client

import (
	"context"
	"os"
	"testing"
	"time"
)

// This integration test hits the real Anthropic-compatible endpoint using the
// current environment variables. It is skipped if no real token is provided to
// avoid accidental charges during unit runs.
func TestIntegration_AnthropicMessage(t *testing.T) {
	token := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if token == "" || token == "dummy-token" {
		t.Skip("real ANTHROPIC_AUTH_TOKEN not set; skipping integration")
	}

	cfg, err := LoadConfig([]string{"ping"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.NoWarmup = true // keep integration quick

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- send(ctx, cfg, messageBody(cfg, cfg.Model, loadTools(), "You are Claude Code, Anthropic's official CLI for Claude."), "claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("integration send failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("integration timed out")
	}
}
