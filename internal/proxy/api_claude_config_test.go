package proxy

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleClaudeConfigTemplate(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-user", "test-key", "test-pass", false)
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	// Create session
	sess := srv.sessionMgr.Create(acc.ID, false)

	t.Run("GET returns config template", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/template", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp ClaudeConfigTemplate
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.APIKey != "test-key" {
			t.Errorf("expected api_key 'test-key', got '%s'", resp.APIKey)
		}
		if resp.AccountName != "test-user" {
			t.Errorf("expected account_name 'test-user', got '%s'", resp.AccountName)
		}
		if resp.ConfigID == "" {
			t.Error("expected non-empty config_id")
		}
		if resp.ConfigJSON == "" {
			t.Error("expected non-empty config_json")
		}
	})

	t.Run("GET with custom proxy_url", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/template?proxy_url=https://custom.example.com", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp ClaudeConfigTemplate
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.ProxyURL != "https://custom.example.com" {
			t.Errorf("expected proxy_url 'https://custom.example.com', got '%s'", resp.ProxyURL)
		}
	})

	t.Run("GET with allow and deny lists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/template?allow=read&allow=write&deny=delete", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp ClaudeConfigTemplate
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Parse config JSON to verify permissions
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(resp.ConfigJSON), &config); err != nil {
			t.Fatalf("failed to parse config JSON: %v", err)
		}

		perms, ok := config["permissions"].(map[string]interface{})
		if !ok {
			t.Fatal("expected permissions in config")
		}

		allowList, ok := perms["allow"].([]interface{})
		if !ok || len(allowList) != 2 {
			t.Errorf("expected 2 allow items, got %v", allowList)
		}

		denyList, ok := perms["deny"].([]interface{})
		if !ok || len(denyList) != 1 {
			t.Errorf("expected 1 deny item, got %v", denyList)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/claude-config/template", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.Token})
		w := httptest.NewRecorder()

		srv.handleClaudeConfigTemplate(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("GET without session returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/template", nil)
		w := httptest.NewRecorder()

		srv.handleClaudeConfigTemplate(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestHandleClaudeConfigDownload(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Save a config
	configContent := `{"env":{"ANTHROPIC_API_KEY":"test"}}`
	configID := srv.saveClaudeConfig(configContent)

	t.Run("GET downloads config successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/download/"+configID, nil)
		w := httptest.NewRecorder()

		srv.handleClaudeConfigDownload(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if w.Body.String() != configContent {
			t.Errorf("expected content '%s', got '%s'", configContent, w.Body.String())
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
		}

		contentDisposition := w.Header().Get("Content-Disposition")
		if !strings.Contains(contentDisposition, "settings.json") {
			t.Errorf("expected Content-Disposition to contain 'settings.json', got '%s'", contentDisposition)
		}
	})

	t.Run("GET with invalid ID returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/download/invalid-id", nil)
		w := httptest.NewRecorder()

		srv.handleClaudeConfigDownload(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("GET with empty ID returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/claude-config/download/", nil)
		w := httptest.NewRecorder()

		srv.handleClaudeConfigDownload(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/claude-config/download/"+configID, nil)
		w := httptest.NewRecorder()

		srv.handleClaudeConfigDownload(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestSaveAndGetClaudeConfig(t *testing.T) {
	b := NewBuilder().WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("save and get config successfully", func(t *testing.T) {
		content := `{"test":"config"}`
		id := srv.saveClaudeConfig(content)

		if id == "" {
			t.Error("expected non-empty config ID")
		}

		retrieved, ok := srv.getClaudeConfig(id)
		if !ok {
			t.Error("expected to retrieve saved config")
		}
		if retrieved != content {
			t.Errorf("expected content '%s', got '%s'", content, retrieved)
		}
	})

	t.Run("get non-existent config returns false", func(t *testing.T) {
		_, ok := srv.getClaudeConfig("non-existent-id")
		if ok {
			t.Error("expected false for non-existent config")
		}
	})

	t.Run("expired config is removed", func(t *testing.T) {
		content := `{"test":"expired"}`
		id := srv.saveClaudeConfig(content)

		// Manually expire the config
		srv.claudeConfigMu.Lock()
		entry := srv.claudeConfigCache[id]
		entry.expiresAt = time.Now().Add(-1 * time.Hour)
		srv.claudeConfigCache[id] = entry
		srv.claudeConfigMu.Unlock()

		_, ok := srv.getClaudeConfig(id)
		if ok {
			t.Error("expected false for expired config")
		}
	})

	t.Run("save cleans up expired entries", func(t *testing.T) {
		// Save an expired config
		srv.claudeConfigMu.Lock()
		srv.claudeConfigCache["expired-1"] = claudeConfigEntry{
			content:   "old",
			expiresAt: time.Now().Add(-1 * time.Hour),
		}
		srv.claudeConfigMu.Unlock()

		// Save a new config (should trigger cleanup)
		srv.saveClaudeConfig(`{"new":"config"}`)

		// Check that expired entry was removed
		srv.claudeConfigMu.RLock()
		_, exists := srv.claudeConfigCache["expired-1"]
		srv.claudeConfigMu.RUnlock()

		if exists {
			t.Error("expected expired entry to be cleaned up")
		}
	})

	t.Run("saveClaudeConfig with nil server returns empty string", func(t *testing.T) {
		var nilSrv *Server
		id := nilSrv.saveClaudeConfig("test")
		if id != "" {
			t.Errorf("expected empty string for nil server, got '%s'", id)
		}
	})

	t.Run("getClaudeConfig with nil server returns false", func(t *testing.T) {
		var nilSrv *Server
		_, ok := nilSrv.getClaudeConfig("test")
		if ok {
			t.Error("expected false for nil server")
		}
	})
}

func TestGenerateShortID(t *testing.T) {
	t.Run("generates non-empty ID", func(t *testing.T) {
		id := generateShortID()
		if id == "" {
			t.Error("expected non-empty ID")
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		id1 := generateShortID()
		id2 := generateShortID()
		if id1 == id2 {
			t.Error("expected unique IDs")
		}
	})

	t.Run("generates hex string", func(t *testing.T) {
		id := generateShortID()
		// Should be hex characters only
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("expected hex character, got '%c'", c)
			}
		}
	})

	t.Run("ID length is 16 characters", func(t *testing.T) {
		id := generateShortID()
		if len(id) != 16 {
			t.Errorf("expected ID length 16, got %d", len(id))
		}
	})
}

func TestBaseURLFromRequest(t *testing.T) {
	t.Run("HTTP request without headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		url := baseURLFromRequest(req)
		if url != "http://example.com" {
			t.Errorf("expected 'http://example.com', got '%s'", url)
		}
	})

	t.Run("HTTPS request with TLS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/path", nil)
		req.TLS = &tls.ConnectionState{}
		url := baseURLFromRequest(req)
		if url != "https://example.com" {
			t.Errorf("expected 'https://example.com', got '%s'", url)
		}
	})

	t.Run("HTTP request with X-Forwarded-Proto header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		url := baseURLFromRequest(req)
		if url != "https://example.com" {
			t.Errorf("expected 'https://example.com', got '%s'", url)
		}
	})

	t.Run("Request with X-Forwarded-Host header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		req.Header.Set("X-Forwarded-Host", "proxy.example.com")
		url := baseURLFromRequest(req)
		if url != "http://proxy.example.com" {
			t.Errorf("expected 'http://proxy.example.com', got '%s'", url)
		}
	})

	t.Run("Nil request returns localhost", func(t *testing.T) {
		url := baseURLFromRequest(nil)
		if url != "http://localhost" {
			t.Errorf("expected 'http://localhost', got '%s'", url)
		}
	})

	t.Run("Strips trailing slash from host", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		req.Host = "example.com/"
		url := baseURLFromRequest(req)
		if url != "http://example.com" {
			t.Errorf("expected 'http://example.com', got '%s'", url)
		}
	})

	t.Run("X-Forwarded-Proto case insensitive", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		req.Header.Set("X-Forwarded-Proto", "HTTPS")
		url := baseURLFromRequest(req)
		if url != "https://example.com" {
			t.Errorf("expected 'https://example.com', got '%s'", url)
		}
	})
}

func TestNormalizeList(t *testing.T) {
	t.Run("empty list returns empty", func(t *testing.T) {
		result := normalizeList([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty list, got %v", result)
		}
	})

	t.Run("single item", func(t *testing.T) {
		result := normalizeList([]string{"item1"})
		if len(result) != 1 || result[0] != "item1" {
			t.Errorf("expected ['item1'], got %v", result)
		}
	})

	t.Run("comma-separated items", func(t *testing.T) {
		result := normalizeList([]string{"item1,item2,item3"})
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d: %v", len(result), result)
		}
	})

	t.Run("removes duplicates", func(t *testing.T) {
		result := normalizeList([]string{"item1", "item2", "item1"})
		if len(result) != 2 {
			t.Errorf("expected 2 unique items, got %d: %v", len(result), result)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result := normalizeList([]string{" item1 ", "  item2  "})
		if len(result) != 2 || result[0] != "item1" || result[1] != "item2" {
			t.Errorf("expected trimmed items, got %v", result)
		}
	})

	t.Run("handles newline separator", func(t *testing.T) {
		result := normalizeList([]string{"item1\nitem2\nitem3"})
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d: %v", len(result), result)
		}
	})

	t.Run("handles semicolon separator", func(t *testing.T) {
		result := normalizeList([]string{"item1;item2;item3"})
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d: %v", len(result), result)
		}
	})

	t.Run("skips empty items", func(t *testing.T) {
		result := normalizeList([]string{"item1", "", "item2", "  ", "item3"})
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d: %v", len(result), result)
		}
	})

	t.Run("mixed separators", func(t *testing.T) {
		result := normalizeList([]string{"item1,item2", "item3;item4", "item5\nitem6"})
		if len(result) != 6 {
			t.Errorf("expected 6 items, got %d: %v", len(result), result)
		}
	})

	t.Run("preserves order", func(t *testing.T) {
		result := normalizeList([]string{"c", "a", "b"})
		if len(result) != 3 || result[0] != "c" || result[1] != "a" || result[2] != "b" {
			t.Errorf("expected ['c', 'a', 'b'], got %v", result)
		}
	})
}

