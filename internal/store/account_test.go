package store

import (
	"context"
	"testing"
	"time"
)

func TestCreateAccount(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	t.Run("create account successfully", func(t *testing.T) {
		acc := AccountRecord{
			ID:          "test-acc-1",
			Name:        "Test Account",
			Password:    "password123",
			ProxyAPIKey: "test-key-1",
			IsAdmin:     false,
		}

		err := s.CreateAccount(ctx, acc)
		if err != nil {
			t.Fatalf("CreateAccount failed: %v", err)
		}

		// Verify account was created
		retrieved, err := s.GetAccountByID(ctx, "test-acc-1")
		if err != nil {
			t.Fatalf("GetAccountByID failed: %v", err)
		}

		if retrieved.Name != acc.Name {
			t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, acc.Name)
		}
		if retrieved.Password != acc.Password {
			t.Errorf("Password mismatch: got %s, want %s", retrieved.Password, acc.Password)
		}
		if retrieved.ProxyAPIKey != acc.ProxyAPIKey {
			t.Errorf("ProxyAPIKey mismatch: got %s, want %s", retrieved.ProxyAPIKey, acc.ProxyAPIKey)
		}
		if retrieved.IsAdmin != acc.IsAdmin {
			t.Errorf("IsAdmin mismatch: got %v, want %v", retrieved.IsAdmin, acc.IsAdmin)
		}
	})

	t.Run("create account with empty ID fails", func(t *testing.T) {
		acc := AccountRecord{
			ID:   "",
			Name: "Test",
		}
		err := s.CreateAccount(ctx, acc)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("create account with empty name fails", func(t *testing.T) {
		acc := AccountRecord{
			ID:   "test-acc-2",
			Name: "",
		}
		err := s.CreateAccount(ctx, acc)
		if err == nil {
			t.Fatal("Expected error for empty name, got nil")
		}
	})

	t.Run("create account with auto timestamps", func(t *testing.T) {
		acc := AccountRecord{
			ID:   "test-acc-3",
			Name: "Auto Timestamp",
		}

		before := time.Now()
		err := s.CreateAccount(ctx, acc)
		after := time.Now()

		if err != nil {
			t.Fatalf("CreateAccount failed: %v", err)
		}

		retrieved, err := s.GetAccountByID(ctx, "test-acc-3")
		if err != nil {
			t.Fatalf("GetAccountByID failed: %v", err)
		}

		if retrieved.CreatedAt.Before(before) || retrieved.CreatedAt.After(after) {
			t.Errorf("CreatedAt not in expected range: %v", retrieved.CreatedAt)
		}
		if retrieved.UpdatedAt.Before(before) || retrieved.UpdatedAt.After(after) {
			t.Errorf("UpdatedAt not in expected range: %v", retrieved.UpdatedAt)
		}
	})
}

func TestGetAccountByProxyKey(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:          "test-acc-proxy",
		Name:        "Proxy Test",
		ProxyAPIKey: "proxy-key-123",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("get account by valid proxy key", func(t *testing.T) {
		retrieved, err := s.GetAccountByProxyKey(ctx, "proxy-key-123")
		if err != nil {
			t.Fatalf("GetAccountByProxyKey failed: %v", err)
		}

		if retrieved.ID != acc.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, acc.ID)
		}
		if retrieved.ProxyAPIKey != acc.ProxyAPIKey {
			t.Errorf("ProxyAPIKey mismatch: got %s, want %s", retrieved.ProxyAPIKey, acc.ProxyAPIKey)
		}
	})

	t.Run("get account by invalid proxy key returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetAccountByProxyKey(ctx, "invalid-key")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestGetAccountByID(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-id",
		Name: "ID Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("get account by valid ID", func(t *testing.T) {
		retrieved, err := s.GetAccountByID(ctx, "test-acc-id")
		if err != nil {
			t.Fatalf("GetAccountByID failed: %v", err)
		}

		if retrieved.ID != acc.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, acc.ID)
		}
		if retrieved.Name != acc.Name {
			t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, acc.Name)
		}
	})

	t.Run("get account by invalid ID returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetAccountByID(ctx, "invalid-id")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestListAccounts(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	t.Run("list empty accounts", func(t *testing.T) {
		accounts, err := s.ListAccounts(ctx)
		if err != nil {
			t.Fatalf("ListAccounts failed: %v", err)
		}

		if len(accounts) != 0 {
			t.Errorf("Expected 0 accounts, got %d", len(accounts))
		}
	})

	t.Run("list multiple accounts", func(t *testing.T) {
		// Create test accounts
		accounts := []AccountRecord{
			{ID: "acc-1", Name: "Account 1"},
			{ID: "acc-2", Name: "Account 2"},
			{ID: "acc-3", Name: "Account 3"},
		}

		for _, acc := range accounts {
			if err := s.CreateAccount(ctx, acc); err != nil {
				t.Fatalf("CreateAccount failed: %v", err)
			}
		}

		// List accounts
		retrieved, err := s.ListAccounts(ctx)
		if err != nil {
			t.Fatalf("ListAccounts failed: %v", err)
		}

		if len(retrieved) != 3 {
			t.Errorf("Expected 3 accounts, got %d", len(retrieved))
		}

		// Verify order (should be by created_at ASC)
		for i, acc := range accounts {
			if retrieved[i].ID != acc.ID {
				t.Errorf("Account %d ID mismatch: got %s, want %s", i, retrieved[i].ID, acc.ID)
			}
		}
	})
}

func TestUpdateAccount(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:          "test-acc-update",
		Name:        "Original Name",
		Password:    "original-pass",
		ProxyAPIKey: "original-key",
		IsAdmin:     false,
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("update account successfully", func(t *testing.T) {
		updated := AccountRecord{
			ID:          "test-acc-update",
			Name:        "Updated Name",
			Password:    "updated-pass",
			ProxyAPIKey: "updated-key",
			IsAdmin:     true,
		}

		err := s.UpdateAccount(ctx, updated)
		if err != nil {
			t.Fatalf("UpdateAccount failed: %v", err)
		}

		// Verify update
		retrieved, err := s.GetAccountByID(ctx, "test-acc-update")
		if err != nil {
			t.Fatalf("GetAccountByID failed: %v", err)
		}

		if retrieved.Name != updated.Name {
			t.Errorf("Name not updated: got %s, want %s", retrieved.Name, updated.Name)
		}
		if retrieved.Password != updated.Password {
			t.Errorf("Password not updated: got %s, want %s", retrieved.Password, updated.Password)
		}
		if retrieved.ProxyAPIKey != updated.ProxyAPIKey {
			t.Errorf("ProxyAPIKey not updated: got %s, want %s", retrieved.ProxyAPIKey, updated.ProxyAPIKey)
		}
		if retrieved.IsAdmin != updated.IsAdmin {
			t.Errorf("IsAdmin not updated: got %v, want %v", retrieved.IsAdmin, updated.IsAdmin)
		}
	})

	t.Run("update non-existent account returns ErrNotFound", func(t *testing.T) {
		updated := AccountRecord{
			ID:   "non-existent",
			Name: "Test",
		}

		err := s.UpdateAccount(ctx, updated)
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("update account with empty ID fails", func(t *testing.T) {
		updated := AccountRecord{
			ID:   "",
			Name: "Test",
		}

		err := s.UpdateAccount(ctx, updated)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})
}

func TestDeleteAccount(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	t.Run("delete account successfully", func(t *testing.T) {
		// Create test account
		acc := AccountRecord{
			ID:   "test-acc-delete",
			Name: "Delete Test",
		}
		if err := s.CreateAccount(ctx, acc); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Delete account
		err := s.DeleteAccount(ctx, "test-acc-delete")
		if err != nil {
			t.Fatalf("DeleteAccount failed: %v", err)
		}

		// Verify deletion
		_, err = s.GetAccountByID(ctx, "test-acc-delete")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})

	t.Run("delete default account fails", func(t *testing.T) {
		err := s.DeleteAccount(ctx, DefaultAccountID)
		if err == nil {
			t.Fatal("Expected error when deleting default account, got nil")
		}
	})

	t.Run("delete non-existent account returns ErrNotFound", func(t *testing.T) {
		err := s.DeleteAccount(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("delete account with empty ID fails", func(t *testing.T) {
		err := s.DeleteAccount(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("delete account cascades to nodes and config", func(t *testing.T) {
		// Create account with nodes
		acc := AccountRecord{
			ID:   "test-acc-cascade",
			Name: "Cascade Test",
		}
		if err := s.CreateAccount(ctx, acc); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Create a node for this account
		node := NodeRecord{
			ID:        "node-cascade",
			AccountID: "test-acc-cascade",
			Name:      "Test Node",
			BaseURL:   "http://test.com",
			APIKey:    "test-key",
		}
		if err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("UpsertNode failed: %v", err)
		}

		// Delete account
		err := s.DeleteAccount(ctx, "test-acc-cascade")
		if err != nil {
			t.Fatalf("DeleteAccount failed: %v", err)
		}

		// Verify node was also deleted
		nodes, err := s.GetNodesByAccount(ctx, "test-acc-cascade")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}
		if len(nodes) != 0 {
			t.Errorf("Expected nodes to be deleted, got %d nodes", len(nodes))
		}
	})
}

// setupTestStore creates a temporary SQLite store for testing
func setupTestStore(t *testing.T) *Store {
	t.Helper()

	// Use in-memory SQLite for tests
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	return s
}
