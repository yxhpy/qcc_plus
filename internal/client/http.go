package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func client() *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         dialer.DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}

func send(ctx context.Context, cfg Config, body Body, beta string) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/v1/messages?beta=true", strings.NewReader(string(b)))
	if err != nil {
		return err
	}

	headers := map[string]string{
		"accept":            "application/json",
		"content-type":      "application/json",
		"authorization":     "Bearer " + cfg.Token,
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    beta,
		"anthropic-dangerous-direct-browser-access": "true",
		"user-agent":                "claude-cli/2.0.47 (external, sdk-cli)",
		"x-app":                     "cli",
		"x-stainless-helper-method": "stream",
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return stream(resp.Body)
}
