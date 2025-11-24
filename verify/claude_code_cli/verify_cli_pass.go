package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	imageName      = "claude-code-cli-verify"
	dockerfilePath = "verify/claude_code_cli/Dockerfile.verify"
	buildContext   = "verify/claude_code_cli"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	prompt := os.Getenv("CLAUDE_PROMPT")
	if prompt == "" {
		prompt = "健康检查：请回复 OK"
	}

	if apiKey == "" || authToken == "" || baseURL == "" {
		log.Fatalf("missing required envs: ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_BASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := buildImage(ctx); err != nil {
		log.Fatalf("docker build failed: %v", err)
	}

	output, err := runClaude(ctx, apiKey, authToken, baseURL, prompt)
	if err != nil {
		log.Fatalf("claude run failed: %v", err)
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		log.Fatalf("claude returned empty output")
	}

	log.Printf("claude output:\n%s", trimmed)
	log.Printf("SUCCESS: Claude Code CLI responded")
}

func buildImage(ctx context.Context) error {
	log.Printf("building docker image %s using %s", imageName, dockerfilePath)
	cmd := exec.CommandContext(ctx, "docker", "build", "-f", dockerfilePath, "-t", imageName, buildContext)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}

	log.Printf("docker build complete")
	return nil
}

func runClaude(ctx context.Context, apiKey, authToken, baseURL, prompt string) (string, error) {
	log.Printf("running claude in container with prompt: %s", prompt)

	args := []string{
		"run", "--rm",
		"-e", "ANTHROPIC_API_KEY=" + apiKey,
		"-e", "ANTHROPIC_AUTH_TOKEN=" + authToken,
		"-e", "ANTHROPIC_BASE_URL=" + baseURL,
		imageName,
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}

	return stdout.String(), nil
}
