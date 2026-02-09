package store

import (
	"context"
	"testing"
	"time"
)

func TestCreateMonitorShare(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account first
	acc := AccountRecord{
		ID:   "test-acc-share",
		Name: "Share Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("create monitor share successfully", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-1",
			AccountID: "test-acc-share",
			Token:     "token-123",
			CreatedBy: "admin",
		}

		err := s.CreateMonitorShare(ctx, share)
		if err != nil {
			t.Fatalf("CreateMonitorShare failed: %v", err)
		}

		// Verify creation
		retrieved, err := s.GetMonitorShareByID(ctx, "share-1")
		if err != nil {
			t.Fatalf("GetMonitorShareByID failed: %v", err)
		}

		if retrieved.ID != share.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, share.ID)
		}
		if retrieved.Token != share.Token {
			t.Errorf("Token mismatch: got %s, want %s", retrieved.Token, share.Token)
		}
	})

	t.Run("create monitor share with expiration", func(t *testing.T) {
		expireAt := time.Now().Add(24 * time.Hour)
		share := MonitorShareRecord{
			ID:        "share-2",
			AccountID: "test-acc-share",
			Token:     "token-456",
			ExpireAt:  expireAt,
			CreatedBy: "admin",
		}

		err := s.CreateMonitorShare(ctx, share)
		if err != nil {
			t.Fatalf("CreateMonitorShare failed: %v", err)
		}

		retrieved, err := s.GetMonitorShareByID(ctx, "share-2")
		if err != nil {
			t.Fatalf("GetMonitorShareByID failed: %v", err)
		}

		if retrieved.ExpireAt.IsZero() {
			t.Error("ExpireAt should not be zero")
		}
	})

	t.Run("create monitor share with empty ID fails", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "",
			AccountID: "test-acc-share",
			Token:     "token-789",
			CreatedBy: "admin",
		}

		err := s.CreateMonitorShare(ctx, share)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("create monitor share with empty token fails", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-3",
			AccountID: "test-acc-share",
			Token:     "",
			CreatedBy: "admin",
		}

		err := s.CreateMonitorShare(ctx, share)
		if err == nil {
			t.Fatal("Expected error for empty token, got nil")
		}
	})

	t.Run("create monitor share with empty account_id fails", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-4",
			AccountID: "",
			Token:     "token-abc",
			CreatedBy: "admin",
		}

		err := s.CreateMonitorShare(ctx, share)
		if err == nil {
			t.Fatal("Expected error for empty account_id, got nil")
		}
	})

	t.Run("create monitor share with empty created_by fails", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-5",
			AccountID: "test-acc-share",
			Token:     "token-def",
			CreatedBy: "",
		}

		err := s.CreateMonitorShare(ctx, share)
		if err == nil {
			t.Fatal("Expected error for empty created_by, got nil")
		}
	})
}

func TestGetMonitorShareByToken(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-token",
		Name: "Token Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("get valid monitor share by token", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-token-1",
			AccountID: "test-acc-token",
			Token:     "valid-token",
			CreatedBy: "admin",
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		retrieved, err := s.GetMonitorShareByToken(ctx, "valid-token")
		if err != nil {
			t.Fatalf("GetMonitorShareByToken failed: %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected share, got nil")
		}
		if retrieved.Token != "valid-token" {
			t.Errorf("Token mismatch: got %s, want %s", retrieved.Token, "valid-token")
		}
	})

	t.Run("get expired monitor share returns nil", func(t *testing.T) {
		expireAt := time.Now().Add(-1 * time.Hour) // Expired
		share := MonitorShareRecord{
			ID:        "share-token-2",
			AccountID: "test-acc-token",
			Token:     "expired-token",
			ExpireAt:  expireAt,
			CreatedBy: "admin",
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		retrieved, err := s.GetMonitorShareByToken(ctx, "expired-token")
		if err != nil {
			t.Fatalf("GetMonitorShareByToken failed: %v", err)
		}

		if retrieved != nil {
			t.Error("Expected nil for expired share")
		}
	})

	t.Run("get revoked monitor share returns nil", func(t *testing.T) {
		now := time.Now()
		share := MonitorShareRecord{
			ID:        "share-token-3",
			AccountID: "test-acc-token",
			Token:     "revoked-token",
			CreatedBy: "admin",
			Revoked:   true,
			RevokedAt: &now,
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		retrieved, err := s.GetMonitorShareByToken(ctx, "revoked-token")
		if err != nil {
			t.Fatalf("GetMonitorShareByToken failed: %v", err)
		}

		if retrieved != nil {
			t.Error("Expected nil for revoked share")
		}
	})

	t.Run("get monitor share with empty token fails", func(t *testing.T) {
		_, err := s.GetMonitorShareByToken(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty token, got nil")
		}
	})

	t.Run("get non-existent monitor share returns nil", func(t *testing.T) {
		retrieved, err := s.GetMonitorShareByToken(ctx, "non-existent-token")
		if err != nil {
			t.Fatalf("GetMonitorShareByToken failed: %v", err)
		}

		if retrieved != nil {
			t.Error("Expected nil for non-existent share")
		}
	})
}

func TestGetMonitorShareByID(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-id",
		Name: "ID Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("get monitor share by valid ID", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-id-1",
			AccountID: "test-acc-id",
			Token:     "token-id-1",
			CreatedBy: "admin",
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		retrieved, err := s.GetMonitorShareByID(ctx, "share-id-1")
		if err != nil {
			t.Fatalf("GetMonitorShareByID failed: %v", err)
		}

		if retrieved.ID != share.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, share.ID)
		}
	})

	t.Run("get monitor share by empty ID fails", func(t *testing.T) {
		_, err := s.GetMonitorShareByID(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("get non-existent monitor share returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetMonitorShareByID(ctx, "non-existent-id")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestListMonitorShares(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test accounts
	acc1 := AccountRecord{ID: "acc-list-1", Name: "List Account 1"}
	acc2 := AccountRecord{ID: "acc-list-2", Name: "List Account 2"}
	if err := s.CreateAccount(ctx, acc1); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.CreateAccount(ctx, acc2); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("list all monitor shares", func(t *testing.T) {
		// Create shares
		shares := []MonitorShareRecord{
			{ID: "list-1", AccountID: "acc-list-1", Token: "token-1", CreatedBy: "admin"},
			{ID: "list-2", AccountID: "acc-list-1", Token: "token-2", CreatedBy: "admin"},
			{ID: "list-3", AccountID: "acc-list-2", Token: "token-3", CreatedBy: "admin"},
		}
		for _, share := range shares {
			if err := s.CreateMonitorShare(ctx, share); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
		}

		params := QueryMonitorSharesParams{}
		retrieved, err := s.ListMonitorShares(ctx, params)
		if err != nil {
			t.Fatalf("ListMonitorShares failed: %v", err)
		}

		if len(retrieved) < 3 {
			t.Errorf("Expected at least 3 shares, got %d", len(retrieved))
		}
	})

	t.Run("list monitor shares by account", func(t *testing.T) {
		params := QueryMonitorSharesParams{
			AccountID: "acc-list-1",
		}
		retrieved, err := s.ListMonitorShares(ctx, params)
		if err != nil {
			t.Fatalf("ListMonitorShares failed: %v", err)
		}

		for _, share := range retrieved {
			if share.AccountID != "acc-list-1" {
				t.Errorf("Expected account_id acc-list-1, got %s", share.AccountID)
			}
		}
	})

	t.Run("list monitor shares with limit", func(t *testing.T) {
		params := QueryMonitorSharesParams{
			Limit: 2,
		}
		retrieved, err := s.ListMonitorShares(ctx, params)
		if err != nil {
			t.Fatalf("ListMonitorShares failed: %v", err)
		}

		if len(retrieved) > 2 {
			t.Errorf("Expected at most 2 shares, got %d", len(retrieved))
		}
	})

	t.Run("list monitor shares with offset", func(t *testing.T) {
		params := QueryMonitorSharesParams{
			Offset: 1,
		}
		retrieved, err := s.ListMonitorShares(ctx, params)
		if err != nil {
			t.Fatalf("ListMonitorShares failed: %v", err)
		}

		// Should skip first share
		if len(retrieved) == 0 {
			t.Error("Expected at least one share after offset")
		}
	})

	t.Run("list monitor shares excluding revoked", func(t *testing.T) {
		// Create revoked share
		now := time.Now()
		share := MonitorShareRecord{
			ID:        "list-revoked",
			AccountID: "acc-list-1",
			Token:     "token-revoked",
			CreatedBy: "admin",
			Revoked:   true,
			RevokedAt: &now,
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		params := QueryMonitorSharesParams{
			IncludeRevoked: false,
		}
		retrieved, err := s.ListMonitorShares(ctx, params)
		if err != nil {
			t.Fatalf("ListMonitorShares failed: %v", err)
		}

		for _, s := range retrieved {
			if s.Revoked {
				t.Error("Expected no revoked shares")
			}
		}
	})

	t.Run("list monitor shares including revoked", func(t *testing.T) {
		params := QueryMonitorSharesParams{
			IncludeRevoked: true,
		}
		retrieved, err := s.ListMonitorShares(ctx, params)
		if err != nil {
			t.Fatalf("ListMonitorShares failed: %v", err)
		}

		hasRevoked := false
		for _, s := range retrieved {
			if s.Revoked {
				hasRevoked = true
				break
			}
		}

		if !hasRevoked {
			t.Error("Expected at least one revoked share")
		}
	})
}

func TestRevokeMonitorShare(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-revoke",
		Name: "Revoke Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("revoke monitor share successfully", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-revoke-1",
			AccountID: "test-acc-revoke",
			Token:     "token-revoke-1",
			CreatedBy: "admin",
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		err := s.RevokeMonitorShare(ctx, "share-revoke-1")
		if err != nil {
			t.Fatalf("RevokeMonitorShare failed: %v", err)
		}

		// Verify revocation
		retrieved, err := s.GetMonitorShareByID(ctx, "share-revoke-1")
		if err != nil {
			t.Fatalf("GetMonitorShareByID failed: %v", err)
		}

		if !retrieved.Revoked {
			t.Error("Expected share to be revoked")
		}
		if retrieved.RevokedAt == nil {
			t.Error("Expected RevokedAt to be set")
		}
	})

	t.Run("revoke monitor share with empty ID fails", func(t *testing.T) {
		err := s.RevokeMonitorShare(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("revoke non-existent monitor share returns error", func(t *testing.T) {
		err := s.RevokeMonitorShare(ctx, "non-existent-revoke")
		if err == nil {
			t.Fatal("Expected error for non-existent share, got nil")
		}
	})
}

func TestDeleteMonitorShare(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-delete",
		Name: "Delete Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("delete monitor share successfully", func(t *testing.T) {
		share := MonitorShareRecord{
			ID:        "share-delete-1",
			AccountID: "test-acc-delete",
			Token:     "token-delete-1",
			CreatedBy: "admin",
		}
		if err := s.CreateMonitorShare(ctx, share); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		err := s.DeleteMonitorShare(ctx, "share-delete-1")
		if err != nil {
			t.Fatalf("DeleteMonitorShare failed: %v", err)
		}

		// Verify deletion
		_, err = s.GetMonitorShareByID(ctx, "share-delete-1")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})

	t.Run("delete monitor share with empty ID fails", func(t *testing.T) {
		err := s.DeleteMonitorShare(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("delete non-existent monitor share returns ErrNotFound", func(t *testing.T) {
		err := s.DeleteMonitorShare(ctx, "non-existent-delete")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}
