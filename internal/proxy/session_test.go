package proxy

import (
	"testing"
	"time"
)

// TestSessionManager tests session management functionality
func TestSessionManager(t *testing.T) {
	mgr := NewSessionManager(0) // Use default TTL

	t.Run("Create session", func(t *testing.T) {
		sess := mgr.Create("account-1", false)
		if sess == nil {
			t.Fatal("expected session to be created")
		}
		if sess.Token == "" {
			t.Error("expected session token to be set")
		}
		if sess.AccountID != "account-1" {
			t.Errorf("expected account ID 'account-1', got %s", sess.AccountID)
		}
		if sess.IsAdmin {
			t.Error("expected IsAdmin to be false")
		}
	})

	t.Run("Create admin session", func(t *testing.T) {
		sess := mgr.Create("admin-1", true)
		if !sess.IsAdmin {
			t.Error("expected IsAdmin to be true")
		}
	})

	t.Run("Get existing session", func(t *testing.T) {
		sess := mgr.Create("account-2", false)
		retrieved := mgr.Get(sess.Token)
		if retrieved == nil {
			t.Fatal("expected to retrieve session")
		}
		if retrieved.Token != sess.Token {
			t.Errorf("expected token %s, got %s", sess.Token, retrieved.Token)
		}
	})

	t.Run("Get non-existent session", func(t *testing.T) {
		sess := mgr.Get("non-existent-token")
		if sess != nil {
			t.Error("expected nil for non-existent session")
		}
	})

	t.Run("Delete session", func(t *testing.T) {
		sess := mgr.Create("account-3", false)
		mgr.Delete(sess.Token)
		retrieved := mgr.Get(sess.Token)
		if retrieved != nil {
			t.Error("expected session to be deleted")
		}
	})

	t.Run("Session expiration", func(t *testing.T) {
		sess := mgr.Create("account-4", false)
		// Set expiration to past
		sess.ExpiresAt = time.Now().Add(-1 * time.Hour)
		mgr.sessions.Store(sess.Token, sess)

		// Validate should return false for expired session
		if mgr.Validate(sess.Token) {
			t.Error("expected expired session to be invalid")
		}
	})

	t.Run("Session validation", func(t *testing.T) {
		sess := mgr.Create("account-5", false)
		if !mgr.Validate(sess.Token) {
			t.Error("expected valid session")
		}
	})

	t.Run("Validate non-existent session", func(t *testing.T) {
		if mgr.Validate("non-existent") {
			t.Error("expected non-existent session to be invalid")
		}
	})
}

// TestRandomToken tests token generation
func TestRandomToken(t *testing.T) {
	token1 := randomToken(32)
	token2 := randomToken(32)

	if token1 == "" {
		t.Error("expected non-empty token")
	}

	if token1 == token2 {
		t.Error("expected unique tokens")
	}

	if len(token1) < 32 {
		t.Errorf("expected token length >= 32, got %d", len(token1))
	}
}
