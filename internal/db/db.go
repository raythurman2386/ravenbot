package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

// DB represents the database connection.
type DB struct {
	*sql.DB
}

// InitDB initializes the SQLite database and creates the schema.
//
// PRAGMAs are set via DSN query parameters so that every connection obtained
// from the pool inherits them automatically:
//   - busy_timeout(5000): wait up to 5 s when the database is locked instead
//     of returning SQLITE_BUSY immediately.
//   - journal_mode(WAL): write-ahead logging for concurrent read/write.
//   - synchronous(NORMAL): safe with WAL and much faster than FULL.
//   - foreign_keys(1): enforce FK constraints.
//
// MaxOpenConns is set to 1 because SQLite only supports a single writer;
// funnelling all access through one connection avoids lock contention entirely.
// The same *sql.DB is shared with GORM (ADK session service) via the Conn
// field on the dialector, so both layers use this single pool.
func InitDB(dbPath string) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	dsn := dbPath +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_txlock=immediate"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite only supports one writer at a time.  A single connection
	// eliminates SQLITE_BUSY between goroutines sharing this pool.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	instance := &DB{db}
	if err := instance.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return instance, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS headlines (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS briefings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS session_summaries (
		session_id TEXT PRIMARY KEY,
		summary TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		message TEXT NOT NULL,
		remind_at TIMESTAMP NOT NULL,
		delivered INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}
