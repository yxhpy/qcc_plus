package client

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadConfig builds Config from CLI args and envs.
func LoadConfig(args []string) (Config, error) {
	if len(args) == 0 {
		return Config{}, errors.New("need a message argument")
	}
	token := firstEnv("ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY")
	if token == "" {
		return Config{}, errors.New("missing ANTHROPIC_AUTH_TOKEN")
	}
	return Config{
		Token:       token,
		BaseURL:     getenvDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		Model:       getenvDefault("MODEL", "claude-sonnet-4-5-20250929"),
		WarmupModel: getenvDefault("WARMUP_MODEL", "claude-haiku-4-5-20251001"),
		NoWarmup:    os.Getenv("NO_WARMUP") == "1",
		Minimal:     os.Getenv("MINIMAL_SYSTEM") != "0",
		UserHash:    os.Getenv("USER_HASH"),
		Message:     strings.Join(args, " "),
	}, nil
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func computeUserHash(cfg Config) string {
	if cfg.UserHash != "" {
		return cfg.UserHash
	}
	if h := scanCaptureHash(); h != "" {
		return h
	}
	sum := sha256.Sum256([]byte(cfg.Token))
	return hex.EncodeToString(sum[:])
}

func scanCaptureHash() string {
	dir := ".capture"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var newest string
	var newestMod time.Time
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "fetch_") || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestMod) {
			newest = filepath.Join(dir, e.Name())
			newestMod = info.ModTime()
		}
	}
	if newest == "" {
		return ""
	}
	f, err := os.Open(newest)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "\"user_id\"") {
			if m := extractHash(line); m != "" {
				return m
			}
		}
	}
	return ""
}

func extractHash(line string) string {
	line = strings.ReplaceAll(line, "\\\"", "\"")
	line = strings.Trim(line, "\"")
	idx := strings.LastIndex(line, "user_")
	if idx == -1 {
		return ""
	}
	rest := line[idx+5:]
	parts := strings.SplitN(rest, "_account__session", 2)
	if len(parts) == 0 {
		return ""
	}
	hash := strings.Trim(parts[0], "\"")
	if len(hash) == 64 {
		return hash
	}
	return ""
}
