package store

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Dialect represents the SQL database type.
type Dialect string

const (
	dialectMySQL  Dialect = "mysql"
	dialectSQLite Dialect = "sqlite"
)

// Store wraps database connection with dialect awareness.
type Store struct {
	db      *sql.DB
	dialect Dialect
}

// Dialect returns the current database dialect.
func (s *Store) Dialect() Dialect {
	return s.dialect
}

// IsSQLite returns true if the store uses SQLite.
func (s *Store) IsSQLite() bool {
	return s.dialect == dialectSQLite
}

// Open initializes a MySQL-backed store (dsn example: user:pass@tcp(host:3306)/dbname?parseTime=true).
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	configureConnPool(db)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	s := &Store{db: db, dialect: dialectMySQL}
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
	if err := s.ensureHealthHistoryTable(ctx); err != nil {
		return err
	}
	if err := s.ensureMonitorShareTable(ctx); err != nil {
		return err
	}
	if err := s.ensureMetricsTables(ctx); err != nil {
		return err
	}
	if err := s.ensureConfigTable(ctx); err != nil {
		return err
	}
	if err := s.ensureTunnelConfigTable(ctx); err != nil {
		return err
	}
	if err := s.ensureNotificationTables(ctx); err != nil {
		return err
	}
	if err := s.ensureSettingsTable(ctx); err != nil {
		return err
	}
	if err := s.SeedDefaultSettings(); err != nil {
		return err
	}
	if err := s.migrateConfigToSettings(ctx); err != nil {
		return err
	}
	if err := s.ensureMonitorSharesTable(ctx); err != nil {
		return err
	}
	// 模型定价和使用日志表
	if err := s.ensurePricingTables(ctx); err != nil {
		return err
	}
	if err := s.SeedDefaultPricing(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Stats returns the current database pool statistics for monitoring or logging.
func (s *Store) Stats() sql.DBStats {
	if s == nil || s.db == nil {
		return sql.DBStats{}
	}
	return s.db.Stats()
}

const (
	defaultMaxOpenConns   = 25
	defaultMaxIdleConns   = 10
	defaultConnMaxLifeSec = 300 // 5 minutes
	defaultConnMaxIdleSec = 180 // 3 minutes
)

// configureConnPool applies sane defaults and env overrides for the MySQL connection pool.
func configureConnPool(db *sql.DB) {
	maxOpen := getEnvInt("MYSQL_MAX_OPEN_CONNS", defaultMaxOpenConns)
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}

	maxIdle := getEnvInt("MYSQL_MAX_IDLE_CONNS", defaultMaxIdleConns)
	if maxIdle >= 0 {
		db.SetMaxIdleConns(maxIdle)
	}

	lifeSeconds := getEnvInt("MYSQL_CONN_MAX_LIFETIME", defaultConnMaxLifeSec)
	if lifeSeconds > 0 {
		db.SetConnMaxLifetime(time.Duration(lifeSeconds) * time.Second)
	}

	idleSeconds := getEnvInt("MYSQL_CONN_MAX_IDLE_TIME", defaultConnMaxIdleSec)
	if idleSeconds > 0 {
		db.SetConnMaxIdleTime(time.Duration(idleSeconds) * time.Second)
	}

	log.Printf("store: mysql pool configured (maxOpen=%d, maxIdle=%d, life=%ds, idle=%ds)", db.Stats().MaxOpenConnections, maxIdle, lifeSeconds, idleSeconds)
}

// getEnvInt reads an int from env with fallback and safeguards against invalid values.
func getEnvInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v < 0 {
		log.Printf("store: invalid %s=%q, using default %d", key, raw, fallback)
		return fallback
	}
	return v
}
