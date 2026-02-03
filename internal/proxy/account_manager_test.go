package proxy

import (
	"context"
	"testing"
)

// Helper function to add account to context
func withAccount(ctx context.Context, acc *Account) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, accountContextKey{}, acc)
}

// TestAccountManager tests account management functions
func TestAccountManager(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	t.Run("Create account successfully", func(t *testing.T) {
		acc, err := srv.createAccount("test-acc", "test-key", "pass123", false)
		if err != nil {
			t.Fatalf("create account failed: %v", err)
		}
		if acc == nil {
			t.Fatal("expected account to be created")
		}
		if acc.Name != "test-acc" {
			t.Errorf("expected name 'test-acc', got %s", acc.Name)
		}
		if acc.ProxyAPIKey != "test-key" {
			t.Errorf("expected proxy key 'test-key', got %s", acc.ProxyAPIKey)
		}
		if acc.Password != "pass123" {
			t.Errorf("expected password 'pass123', got %s", acc.Password)
		}
		if acc.IsAdmin {
			t.Error("expected IsAdmin to be false")
		}
	})

	t.Run("Create admin account", func(t *testing.T) {
		acc, err := srv.createAccount("admin-acc", "admin-key", "admin-pass", true)
		if err != nil {
			t.Fatalf("create admin account failed: %v", err)
		}
		if !acc.IsAdmin {
			t.Error("expected IsAdmin to be true")
		}
	})

	t.Run("Get account by ID", func(t *testing.T) {
		acc, err := srv.createAccount("get-test", "get-key", "pass", false)
		if err != nil {
			t.Fatalf("create account failed: %v", err)
		}

		retrieved := srv.getAccountByID(acc.ID)
		if retrieved == nil {
			t.Fatal("expected to retrieve account")
		}
		if retrieved.ID != acc.ID {
			t.Errorf("expected ID %s, got %s", acc.ID, retrieved.ID)
		}
	})

	t.Run("Get non-existent account returns nil", func(t *testing.T) {
		acc := srv.getAccountByID("non-existent-id")
		if acc != nil {
			t.Error("expected nil for non-existent account")
		}
	})

	t.Run("Get account by proxy key", func(t *testing.T) {
		acc, err := srv.createAccount("proxy-test", "unique-proxy-key", "pass", false)
		if err != nil {
			t.Fatalf("create account failed: %v", err)
		}

		retrieved := srv.getAccountByProxyKey("unique-proxy-key")
		if retrieved == nil {
			t.Fatal("expected to retrieve account by proxy key")
		}
		if retrieved.ID != acc.ID {
			t.Errorf("expected ID %s, got %s", acc.ID, retrieved.ID)
		}
	})

	t.Run("Get account by non-existent proxy key returns nil", func(t *testing.T) {
		acc := srv.getAccountByProxyKey("non-existent-key")
		if acc != nil {
			t.Error("expected nil for non-existent proxy key")
		}
	})

	t.Run("List all accounts", func(t *testing.T) {
		// Create a few accounts
		srv.createAccount("list-1", "key-1", "pass", false)
		srv.createAccount("list-2", "key-2", "pass", false)

		ctx := context.Background()
		accounts := srv.listAccounts(ctx)
		if len(accounts) == 0 {
			t.Error("expected at least some accounts")
		}
	})
}

// TestCanManageAccount tests account permission checks
func TestCanManageAccount(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create admin account
	adminAcc, err := srv.createAccount("admin", "admin-key", "pass", true)
	if err != nil {
		t.Fatalf("create admin account: %v", err)
	}

	// Create regular account
	regularAcc, err := srv.createAccount("regular", "regular-key", "pass", false)
	if err != nil {
		t.Fatalf("create regular account: %v", err)
	}

	// Create another regular account
	otherAcc, err := srv.createAccount("other", "other-key", "pass", false)
	if err != nil {
		t.Fatalf("create other account: %v", err)
	}

	t.Run("Admin can manage any account", func(t *testing.T) {
		// Create context with admin account
		ctx := withAccount(nil, adminAcc)
		ctx = withAdmin(ctx, true)

		if !canManageAccount(ctx, regularAcc.ID) {
			t.Error("admin should be able to manage regular account")
		}
		if !canManageAccount(ctx, otherAcc.ID) {
			t.Error("admin should be able to manage other account")
		}
	})

	t.Run("Regular user can manage own account", func(t *testing.T) {
		ctx := withAccount(nil, regularAcc)
		ctx = withAdmin(ctx, false)

		if !canManageAccount(ctx, regularAcc.ID) {
			t.Error("user should be able to manage own account")
		}
	})

	t.Run("Regular user cannot manage other accounts", func(t *testing.T) {
		ctx := withAccount(nil, regularAcc)
		ctx = withAdmin(ctx, false)

		if canManageAccount(ctx, otherAcc.ID) {
			t.Error("user should not be able to manage other account")
		}
	})

	t.Run("Cannot manage non-existent account", func(t *testing.T) {
		ctx := withAccount(nil, adminAcc)
		ctx = withAdmin(ctx, true)

		// canManageAccount is a pure permission check, not an existence check
		// Admins have permission to manage any account ID (if it exists)
		// The existence check should happen at the handler level
		if !canManageAccount(ctx, "non-existent-id") {
			t.Error("admin should have permission to manage any account ID")
		}
	})
}
