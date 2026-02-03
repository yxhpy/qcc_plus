package proxy

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateShareToken(t *testing.T) {
	t.Run("generates non-empty token", func(t *testing.T) {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err := generateShareToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		token2, err := generateShareToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if token1 == token2 {
			t.Error("expected unique tokens")
		}
	})

	t.Run("generates hex string", func(t *testing.T) {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be hex characters only
		for _, c := range token {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("expected hex character, got '%c'", c)
			}
		}
	})

	t.Run("token length is 32 characters", func(t *testing.T) {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 16 bytes = 32 hex characters
		if len(token) != 32 {
			t.Errorf("expected token length 32, got %d", len(token))
		}
	})
}

func TestBuildShareURL(t *testing.T) {
	t.Run("HTTP request without headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		url := buildShareURL(req, "test-token")

		expected := "http://example.com/monitor/share/test-token"
		if url != expected {
			t.Errorf("expected '%s', got '%s'", expected, url)
		}
	})

	t.Run("HTTPS request with TLS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/path", nil)
		req.TLS = &tls.ConnectionState{}
		url := buildShareURL(req, "test-token")

		expected := "https://example.com/monitor/share/test-token"
		if url != expected {
			t.Errorf("expected '%s', got '%s'", expected, url)
		}
	})

	t.Run("HTTP request with X-Forwarded-Proto header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		url := buildShareURL(req, "test-token")

		expected := "https://example.com/monitor/share/test-token"
		if url != expected {
			t.Errorf("expected '%s', got '%s'", expected, url)
		}
	})

	t.Run("X-Forwarded-Proto case insensitive", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		req.Header.Set("X-Forwarded-Proto", "HTTPS")
		url := buildShareURL(req, "test-token")

		if !strings.HasPrefix(url, "https://") {
			t.Errorf("expected HTTPS URL, got '%s'", url)
		}
	})

	t.Run("nil request returns http://", func(t *testing.T) {
		url := buildShareURL(nil, "test-token")

		expected := "http:///monitor/share/test-token"
		if url != expected {
			t.Errorf("expected '%s', got '%s'", expected, url)
		}
	})

	t.Run("different hosts", func(t *testing.T) {
		hosts := []string{
			"example.com",
			"localhost:8000",
			"api.example.com:443",
		}

		for _, host := range hosts {
			req := httptest.NewRequest(http.MethodGet, "http://"+host+"/path", nil)
			url := buildShareURL(req, "token123")

			if !strings.Contains(url, host) {
				t.Errorf("expected URL to contain host '%s', got '%s'", host, url)
			}
			if !strings.HasSuffix(url, "/monitor/share/token123") {
				t.Errorf("expected URL to end with '/monitor/share/token123', got '%s'", url)
			}
		}
	})

	t.Run("special characters in token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		token := "abc-123_xyz"
		url := buildShareURL(req, token)

		if !strings.HasSuffix(url, "/monitor/share/"+token) {
			t.Errorf("expected URL to contain token '%s', got '%s'", token, url)
		}
	})
}
