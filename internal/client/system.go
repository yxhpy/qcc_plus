package client

import (
	"os"
	"runtime"
	"strings"

	"qcc_plus/cccli"
	"qcc_plus/internal/timeutil"
)

func system0(minimal bool) string {
	if minimal {
		return "You are Claude Code, Anthropic's official CLI for Claude."
	}
	return strings.TrimSpace(cccli.System0)
}

func renderSystem1(cfg Config, model string) string {
	if cfg.Minimal {
		return "You are a file search specialist for Claude Code, Anthropic's official CLI for Claude."
	}
	tpl := cccli.System1
	envBlock := strings.Join([]string{
		"Working directory: " + mustPwd(),
		"Is directory a git repo: " + gitFlag(),
		"Platform: " + strings.ToLower(os.Getenv("OSTYPE")),
		"OS Version: " + uname(),
		"Today's date: " + timeutil.FormatBeijingTime(timeutil.NowBeijing()),
	}, "\n")
	tpl = strings.Replace(tpl, "<env>", "<env>\n"+envBlock, 1)
	tpl = strings.ReplaceAll(tpl, "The exact model ID is claude-haiku-4-5-20251001.", "The exact model ID is "+model+".")
	tpl = strings.ReplaceAll(tpl, "The exact model ID is claude-sonnet-4-5-20250929.", "The exact model ID is "+model+".")
	return tpl
}

func mustPwd() string {
	p, err := os.Getwd()
	must(err)
	return p
}

func uname() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func gitFlag() string {
	return "Unknown"
}
