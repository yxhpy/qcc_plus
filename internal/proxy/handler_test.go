package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Helper function to add admin context
func withAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, isAdminContextKey{}, isAdmin)
}

// TestHandleVersion tests the /version endpoint
func TestHandleVersion(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("GET returns version info", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		w := httptest.NewRecorder()

		srv.handleVersion(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Check that version info is present
		if _, ok := response["version"]; !ok {
			t.Error("expected 'version' field in response")
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/version", nil)
		w := httptest.NewRecorder()

		srv.handleVersion(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

// TestHandleChangelog tests the /changelog endpoint
func TestHandleChangelog(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("GET returns changelog when file exists", func(t *testing.T) {
		// Create a temporary CHANGELOG.md file
		tmpDir := t.TempDir()
		changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
		content := []byte("# Changelog\n\n## v1.0.0\n- Initial release")
		if err := os.WriteFile(changelogPath, content, 0644); err != nil {
			t.Fatalf("failed to create changelog: %v", err)
		}

		// Change to temp directory
		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		req := httptest.NewRequest(http.MethodGet, "/changelog", nil)
		w := httptest.NewRecorder()

		srv.handleChangelog(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
			t.Errorf("expected markdown content type, got %s", w.Header().Get("Content-Type"))
		}

		if w.Body.String() != string(content) {
			t.Error("changelog content mismatch")
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/changelog", nil)
		w := httptest.NewRecorder()

		srv.handleChangelog(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

// TestHandleEnvVars tests the /admin/api/envvars endpoint
func TestHandleEnvVars(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("GET with admin access returns env vars", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/envvars", nil)
		// Set admin context
		ctx := req.Context()
		ctx = withAdmin(ctx, true)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		srv.handleEnvVars(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["data"]; !ok {
			t.Error("expected 'data' field in response")
		}
	})

	t.Run("GET with category filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/envvars?category=basic", nil)
		ctx := req.Context()
		ctx = withAdmin(ctx, true)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		srv.handleEnvVars(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("GET without admin access returns forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/envvars", nil)
		w := httptest.NewRecorder()

		srv.handleEnvVars(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/envvars", nil)
		ctx := req.Context()
		ctx = withAdmin(ctx, true)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		srv.handleEnvVars(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

// TestHandleEnvVarsCategories tests the /admin/api/envvars/categories endpoint
func TestHandleEnvVarsCategories(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("GET with admin access returns categories", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/envvars/categories", nil)
		ctx := req.Context()
		ctx = withAdmin(ctx, true)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		srv.handleEnvVarsCategories(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["data"]; !ok {
			t.Error("expected 'data' field in response")
		}
	})

	t.Run("GET without admin access returns forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/envvars/categories", nil)
		w := httptest.NewRecorder()

		srv.handleEnvVarsCategories(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/envvars/categories", nil)
		ctx := req.Context()
		ctx = withAdmin(ctx, true)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		srv.handleEnvVarsCategories(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

// TestRequireAuth tests the requireAuth middleware
func TestRequireAuth(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create a test handler that checks if auth was successful
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
	}

	t.Run("admin key in header grants access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("x-admin-key", srv.adminKey)
		w := httptest.NewRecorder()

		handler := srv.requireAuth(testHandler)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("admin key in query grants access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?admin_key="+srv.adminKey, nil)
		w := httptest.NewRecorder()

		handler := srv.requireAuth(testHandler)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("valid proxy key grants access", func(t *testing.T) {
		// Create account with proxy key
		acc, err := srv.createAccount("test-account", "test-proxy-key", "test123", false)
		if err != nil {
			t.Fatalf("create account: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("x-api-key", acc.ProxyAPIKey)
		w := httptest.NewRecorder()

		handler := srv.requireAuth(testHandler)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("no auth returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler := srv.requireAuth(testHandler)
		handler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("invalid admin key returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("x-admin-key", "invalid-key")
		w := httptest.NewRecorder()

		handler := srv.requireAuth(testHandler)
		handler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
