package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB represents the database connection.
type DB struct {
	*sql.DB
}

// InitDB initializes the SQLite database and creates the schema.
func InitDB(dbPath string) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode and standard optimizations
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

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

	CREATE TABLE IF NOT EXISTS sessions (
		app_name TEXT NOT NULL,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		state TEXT NOT NULL, -- JSON serialized session-specific state
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (app_name, user_id, session_id)
	);

	CREATE TABLE IF NOT EXISTS session_events (
		id TEXT PRIMARY KEY, -- uuid
		app_name TEXT NOT NULL,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		event_json TEXT NOT NULL, -- JSON serialized session.Event
		timestamp TIMESTAMP NOT NULL,
		FOREIGN KEY (app_name, user_id, session_id) REFERENCES sessions(app_name, user_id, session_id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS persistent_states (
		scope TEXT NOT NULL, -- 'app' or 'user'
		app_name TEXT NOT NULL,
		user_id TEXT NOT NULL, -- empty if scope is 'app'
		key TEXT NOT NULL,
		value_json TEXT NOT NULL,
		PRIMARY KEY (scope, app_name, user_id, key)
	);
	`
	_, err := db.Exec(schema)
	return err
}
