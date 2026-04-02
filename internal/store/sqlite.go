package store

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// OpenSQLite initializes a SQLite-backed store.
// The dsn can be a file path like "/path/to/data.db" or ":memory:" for in-memory.
func OpenSQLite(dsn string) (*Store, error) {
	return openSQLiteStore(dsn, true, true, false)
}

// OpenExistingSQLite opens an existing SQLite database without running migrations.
func OpenExistingSQLite(path string) (*Store, error) {
	return openSQLiteStore(path, false, false, true)
}

func openSQLiteStore(dsn string, ensureDir bool, runMigrations bool, requireExisting bool) (*Store, error) {
	if requireExisting && dsn != "" && dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
		if _, err := os.Stat(dsn); err != nil {
			return nil, err
		}
	}

	if ensureDir && dsn != "" && dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
		dir := filepath.Dir(dsn)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	applySQLitePragmas(db)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{db: db, dialect: dialectSQLite}
	if runMigrations {
		if err := s.migrate(ctx); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return s, nil
}

func applySQLitePragmas(db *sql.DB) {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("sqlite: warning: %s failed: %v", pragma, err)
		}
	}
}
