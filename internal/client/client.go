package client

import (
	"context"
	"fmt"
	"os"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// Run executes warmups (unless disabled) and the main request.
func Run(cfg Config) error {
	system1 := renderSystem1(cfg, cfg.Model)
	tools := loadTools()

	ctx := context.Background()

	if !cfg.NoWarmup {
		fmt.Fprintln(os.Stderr, "Warmup #1 with", cfg.WarmupModel)
		if err := send(ctx, cfg, warmupBody(cfg, cfg.WarmupModel), "interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "\nWarmup #2 with", cfg.Model)
		if err := send(ctx, cfg, warmupBody(cfg, cfg.Model), "claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"); err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stderr, "\nMain request with", cfg.Model)
	body := messageBody(cfg, cfg.Model, tools, system1)
	return send(ctx, cfg, body, "claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14")
}
