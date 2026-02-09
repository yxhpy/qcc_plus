package store

import (
	"context"
	"testing"
)

func TestTunnelConfig(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	t.Run("get tunnel config when not exists returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetTunnelConfig(ctx)
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("save and get tunnel config", func(t *testing.T) {
		cfg := TunnelConfig{
			ID:        "default",
			APIToken:  "test-api-token-123",
			Subdomain: "my-tunnel",
			Zone:      "example.com",
			Enabled:   true,
			PublicURL: "https://my-tunnel.example.com",
			Status:    "active",
		}

		err := s.SaveTunnelConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("SaveTunnelConfig failed: %v", err)
		}

		// Retrieve and verify
		retrieved, err := s.GetTunnelConfig(ctx)
		if err != nil {
			t.Fatalf("GetTunnelConfig failed: %v", err)
		}

		if retrieved.APIToken != cfg.APIToken {
			t.Errorf("APIToken mismatch: got %s, want %s", retrieved.APIToken, cfg.APIToken)
		}
		if retrieved.Subdomain != cfg.Subdomain {
			t.Errorf("Subdomain mismatch: got %s, want %s", retrieved.Subdomain, cfg.Subdomain)
		}
		if retrieved.Zone != cfg.Zone {
			t.Errorf("Zone mismatch: got %s, want %s", retrieved.Zone, cfg.Zone)
		}
		if retrieved.Enabled != cfg.Enabled {
			t.Errorf("Enabled mismatch: got %v, want %v", retrieved.Enabled, cfg.Enabled)
		}
		if retrieved.PublicURL != cfg.PublicURL {
			t.Errorf("PublicURL mismatch: got %s, want %s", retrieved.PublicURL, cfg.PublicURL)
		}
		if retrieved.Status != cfg.Status {
			t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, cfg.Status)
		}
	})

	t.Run("update tunnel config", func(t *testing.T) {
		cfg := TunnelConfig{
			ID:        "default",
			APIToken:  "updated-token",
			Subdomain: "updated-tunnel",
			Zone:      "updated.com",
			Enabled:   false,
			PublicURL: "https://updated-tunnel.updated.com",
			Status:    "inactive",
			LastError: "test error",
		}

		err := s.SaveTunnelConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("SaveTunnelConfig failed: %v", err)
		}

		// Verify update
		retrieved, err := s.GetTunnelConfig(ctx)
		if err != nil {
			t.Fatalf("GetTunnelConfig failed: %v", err)
		}

		if retrieved.APIToken != cfg.APIToken {
			t.Errorf("APIToken not updated: got %s, want %s", retrieved.APIToken, cfg.APIToken)
		}
		if retrieved.Subdomain != cfg.Subdomain {
			t.Errorf("Subdomain not updated: got %s, want %s", retrieved.Subdomain, cfg.Subdomain)
		}
		if retrieved.LastError != cfg.LastError {
			t.Errorf("LastError not updated: got %s, want %s", retrieved.LastError, cfg.LastError)
		}
	})

	t.Run("save tunnel config without api token preserves existing token", func(t *testing.T) {
		// First save with token
		cfg1 := TunnelConfig{
			ID:        "default",
			APIToken:  "original-token",
			Subdomain: "test",
			Zone:      "test.com",
			Enabled:   true,
		}
		if err := s.SaveTunnelConfig(ctx, cfg1); err != nil {
			t.Fatalf("SaveTunnelConfig failed: %v", err)
		}

		// Update without token
		cfg2 := TunnelConfig{
			ID:        "default",
			APIToken:  "", // Empty token
			Subdomain: "updated-test",
			Zone:      "test.com",
			Enabled:   false,
		}
		if err := s.SaveTunnelConfig(ctx, cfg2); err != nil {
			t.Fatalf("SaveTunnelConfig failed: %v", err)
		}

		// Verify token is preserved
		retrieved, err := s.GetTunnelConfig(ctx)
		if err != nil {
			t.Fatalf("GetTunnelConfig failed: %v", err)
		}

		if retrieved.APIToken != "original-token" {
			t.Errorf("Expected token to be preserved, got %s", retrieved.APIToken)
		}
		if retrieved.Subdomain != "updated-test" {
			t.Errorf("Subdomain should be updated: got %s, want %s", retrieved.Subdomain, "updated-test")
		}
	})

	t.Run("save tunnel config with empty ID uses default", func(t *testing.T) {
		cfg := TunnelConfig{
			ID:        "", // Empty ID
			APIToken:  "test-token",
			Subdomain: "test",
			Zone:      "test.com",
			Enabled:   true,
		}

		err := s.SaveTunnelConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("SaveTunnelConfig failed: %v", err)
		}

		// Verify it was saved with default ID
		retrieved, err := s.GetTunnelConfig(ctx)
		if err != nil {
			t.Fatalf("GetTunnelConfig failed: %v", err)
		}

		if retrieved.ID != "default" {
			t.Errorf("Expected ID to be 'default', got %s", retrieved.ID)
		}
	})

	t.Run("token encoding and decoding", func(t *testing.T) {
		originalToken := "my-secret-token-12345"

		cfg := TunnelConfig{
			ID:        "default",
			APIToken:  originalToken,
			Subdomain: "test",
			Zone:      "test.com",
			Enabled:   true,
		}

		err := s.SaveTunnelConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("SaveTunnelConfig failed: %v", err)
		}

		// Retrieve and verify token is decoded correctly
		retrieved, err := s.GetTunnelConfig(ctx)
		if err != nil {
			t.Fatalf("GetTunnelConfig failed: %v", err)
		}

		if retrieved.APIToken != originalToken {
			t.Errorf("Token not decoded correctly: got %s, want %s", retrieved.APIToken, originalToken)
		}
	})
}
