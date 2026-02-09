package store

import (
	"testing"
)

func TestStoreDialect(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	t.Run("dialect returns sqlite for test store", func(t *testing.T) {
		dialect := s.Dialect()
		if dialect != dialectSQLite {
			t.Errorf("Expected dialect %s, got %s", dialectSQLite, dialect)
		}
	})

	t.Run("IsSQLite returns true for test store", func(t *testing.T) {
		if !s.IsSQLite() {
			t.Error("Expected IsSQLite to return true")
		}
	})
}

func TestStoreStats(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	t.Run("stats returns valid stats", func(t *testing.T) {
		stats := s.Stats()
		// Just verify we can call it without panic
		_ = stats
	})

	t.Run("stats on nil store returns empty stats", func(t *testing.T) {
		var s *Store
		stats := s.Stats()
		if stats.OpenConnections != 0 {
			t.Error("Expected empty stats for nil store")
		}
	})
}

func TestStoreClose(t *testing.T) {
	t.Run("close on nil store returns nil", func(t *testing.T) {
		var s *Store
		err := s.Close()
		if err != nil {
			t.Errorf("Expected nil error for nil store, got %v", err)
		}
	})

	t.Run("close on valid store succeeds", func(t *testing.T) {
		s := setupTestStore(t)
		err := s.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}
