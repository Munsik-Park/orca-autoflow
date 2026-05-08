package store

import (
	"database/sql"
	"fmt"
	"sync/atomic"

	_ "modernc.org/sqlite" // register "sqlite" driver; CGO_ENABLED=0
)

var memCounter atomic.Int64

// Store wraps a SQLite database connection and exposes domain-level operations.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path, runs all embedded
// migrations, and returns a ready-to-use Store.
//
// Use ":memory:" for in-process testing (each call gets an isolated DB).
// WAL mode is set via PRAGMA after opening, outside any transaction.
func Open(path string) (*Store, error) {
	var dsn string
	if path == ":memory:" {
		// Each call gets its own named in-memory DB so parallel tests are isolated.
		n := memCounter.Add(1)
		dsn = fmt.Sprintf("file:orcamem%d?mode=memory&cache=private&_foreign_keys=on", n)
	} else {
		dsn = fmt.Sprintf("file:%s?_busy_timeout=5000&_foreign_keys=on", path)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite in WAL mode works best with a single writer connection.
	db.SetMaxOpenConns(1)

	// Apply WAL outside any transaction; PRAGMA journal_mode fails inside a tx.
	if path != ":memory:" {
		if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set WAL mode: %w", err)
		}
	}

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced use cases.
// Prefer the domain-level methods over direct DB access.
func (s *Store) DB() *sql.DB {
	return s.db
}
