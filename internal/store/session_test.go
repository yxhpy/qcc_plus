package store

import (
	"context"
	"testing"
	"time"
)

func TestSessionStoreCRUD(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	record := SessionRecord{
		Token:     "session-token-1",
		AccountID: "account-1",
		IsAdmin:   true,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := s.UpsertSession(ctx, record); err != nil {
		t.Fatalf("UpsertSession failed: %v", err)
	}

	got, err := s.GetSessionByToken(ctx, record.Token)
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if got.AccountID != record.AccountID {
		t.Fatalf("account_id=%s want=%s", got.AccountID, record.AccountID)
	}
	if !got.IsAdmin {
		t.Fatalf("is_admin=%v want=true", got.IsAdmin)
	}

	record.IsAdmin = false
	record.ExpiresAt = now.Add(48 * time.Hour)
	if err := s.UpsertSession(ctx, record); err != nil {
		t.Fatalf("UpsertSession update failed: %v", err)
	}

	updated, err := s.GetSessionByToken(ctx, record.Token)
	if err != nil {
		t.Fatalf("GetSessionByToken after update failed: %v", err)
	}
	if updated.IsAdmin {
		t.Fatalf("is_admin=%v want=false", updated.IsAdmin)
	}
	if !updated.ExpiresAt.Equal(record.ExpiresAt) {
		t.Fatalf("expires_at=%v want=%v", updated.ExpiresAt, record.ExpiresAt)
	}

	if err := s.DeleteSession(ctx, record.Token); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if _, err := s.GetSessionByToken(ctx, record.Token); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSessionStoreDeleteExpiredSessions(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	expired := SessionRecord{
		Token:     "expired-session",
		AccountID: "account-1",
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}
	active := SessionRecord{
		Token:     "active-session",
		AccountID: "account-1",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}

	if err := s.UpsertSession(ctx, expired); err != nil {
		t.Fatalf("insert expired session failed: %v", err)
	}
	if err := s.UpsertSession(ctx, active); err != nil {
		t.Fatalf("insert active session failed: %v", err)
	}

	if err := s.DeleteExpiredSessions(ctx, now); err != nil {
		t.Fatalf("DeleteExpiredSessions failed: %v", err)
	}

	if _, err := s.GetSessionByToken(ctx, expired.Token); err != ErrNotFound {
		t.Fatalf("expected expired session to be deleted, got %v", err)
	}
	if _, err := s.GetSessionByToken(ctx, active.Token); err != nil {
		t.Fatalf("expected active session to remain, got %v", err)
	}
}

func TestDeleteAccountRemovesSessions(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()
	account := AccountRecord{
		ID:          "session-account",
		Name:        "session-account",
		Password:    "secret",
		ProxyAPIKey: "session-key",
	}
	if err := s.CreateAccount(ctx, account); err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	session := SessionRecord{
		Token:     "session-to-delete",
		AccountID: account.ID,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second),
	}
	if err := s.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession failed: %v", err)
	}

	if err := s.DeleteAccount(ctx, account.ID); err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}
	if _, err := s.GetSessionByToken(ctx, session.Token); err != ErrNotFound {
		t.Fatalf("expected session to be removed with account, got %v", err)
	}
}
