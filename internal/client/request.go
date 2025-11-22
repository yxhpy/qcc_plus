package client

import (
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"

	"qcc_plus/cccli"
)

func cacheCtrl() *CacheControl { return &CacheControl{Type: "ephemeral"} }

func buildSystem(cfg Config, system1 string) []SystemBlock {
	return []SystemBlock{
		{Type: "text", Text: system0(cfg.Minimal), CacheControl: cacheCtrl()},
		{Type: "text", Text: system1, CacheControl: cacheCtrl()},
	}
}

func loadTools() any {
	var v any
	must(json.Unmarshal([]byte(cccli.ToolsJSON), &v))
	return v
}

func messageBody(cfg Config, model string, tools any, system1 string) Body {
	return Body{
		Model: model,
		Messages: []Message{{
			Role: "user",
			Content: []ContentItem{{
				Type:         "text",
				Text:         cfg.Message,
				CacheControl: cacheCtrl(),
			}},
		}},
		System:    buildSystem(cfg, system1),
		Tools:     tools,
		Metadata:  map[string]any{"user_id": fmt.Sprintf("user_%s_account__session_%s", computeUserHash(cfg), uuid())},
		MaxTokens: 32000,
		Stream:    true,
	}
}

func warmupBody(cfg Config, model string) Body {
	return Body{
		Model: model,
		Messages: []Message{{
			Role: "user",
			Content: []ContentItem{{
				Type:         "text",
				Text:         "Warmup",
				CacheControl: cacheCtrl(),
			}},
		}},
		System:    buildSystem(cfg, renderSystem1(cfg, model)),
		MaxTokens: 32000,
		Stream:    true,
		Metadata: map[string]any{
			"user_id": fmt.Sprintf("user_%s_account__session_%s", computeUserHash(cfg), uuid()),
		},
	}
}

func uuid() string {
	b := make([]byte, 16)
	_, _ = io.ReadFull(cryptoRand.Reader, b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
