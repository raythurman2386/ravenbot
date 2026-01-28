package db

import (
	"context"
	"fmt"
)

// HasHeadline checks if a headline URL already exists in the database.
func (db *DB) HasHeadline(ctx context.Context, url string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM headlines WHERE url = ?)`
	err := db.QueryRowContext(ctx, query, url).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check headline existence: %w", err)
	}
	return exists, nil
}

// AddHeadline adds a new headline URL and title to the database.
func (db *DB) AddHeadline(ctx context.Context, title, url string) error {
	query := `INSERT INTO headlines (title, url) VALUES (?, ?)`
	_, err := db.ExecContext(ctx, query, title, url)
	if err != nil {
		return fmt.Errorf("failed to add headline: %w", err)
	}
	return nil
}

// SaveBriefing saves a generated briefing to the database.
func (db *DB) SaveBriefing(ctx context.Context, content string) error {
	query := `INSERT INTO briefings (content) VALUES (?)`
	_, err := db.ExecContext(ctx, query, content)
	if err != nil {
		return fmt.Errorf("failed to save briefing: %w", err)
	}
	return nil
}
