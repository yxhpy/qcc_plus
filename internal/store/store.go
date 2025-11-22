package store

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct{ db *sql.DB }

// Open initializes a MySQL-backed store (dsn example: user:pass@tcp(host:3306)/dbname?parseTime=true).
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	if err := s.ensureAccountsTable(ctx); err != nil {
		return err
	}
	if err := s.ensureAccountPassword(ctx); err != nil {
		return err
	}
	if err := s.ensureNodesTable(ctx); err != nil {
		return err
	}
	return s.ensureConfigTable(ctx)
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
