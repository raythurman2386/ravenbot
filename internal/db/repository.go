package db

import (
	"context"
	"fmt"
	"strings"
)

// Headline represents a news headline.
type Headline struct {
	Title string
	URL   string
}

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

// GetExistingHeadlines returns a map of URLs that already exist in the database.
func (db *DB) GetExistingHeadlines(ctx context.Context, urls []string) (map[string]bool, error) {
	if len(urls) == 0 {
		return make(map[string]bool), nil
	}

	placeholders := make([]string, len(urls))
	args := make([]any, len(urls))
	for i, url := range urls {
		placeholders[i] = "?"
		args[i] = url
	}

	query := fmt.Sprintf("SELECT url FROM headlines WHERE url IN (%s)", strings.Join(placeholders, ","))
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing headlines: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("failed to scan url: %w", err)
		}
		existing[url] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return existing, nil
}

// AddHeadlines adds multiple new headline URLs and titles to the database in a single transaction.
func (db *DB) AddHeadlines(ctx context.Context, headlines []Headline) error {
	if len(headlines) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO headlines (title, url) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, h := range headlines {
		if _, err := stmt.ExecContext(ctx, h.Title, h.URL); err != nil {
			return fmt.Errorf("failed to execute statement for url %s: %w", h.URL, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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
