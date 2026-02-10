package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SaveSessionSummary persists a conversation summary for a specific session.
func (db *DB) SaveSessionSummary(ctx context.Context, sessionID, summary string) error {
	query := `
		INSERT INTO session_summaries (session_id, summary, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(session_id) DO UPDATE SET
			summary = excluded.summary,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := db.ExecContext(ctx, query, sessionID, summary)
	if err != nil {
		return fmt.Errorf("failed to save session summary: %w", err)
	}
	return nil
}

// GetSessionSummary retrieves the persisted summary for a session.
func (db *DB) GetSessionSummary(ctx context.Context, sessionID string) (string, error) {
	var summary string
	query := `SELECT summary FROM session_summaries WHERE session_id = ?`
	err := db.QueryRowContext(ctx, query, sessionID).Scan(&summary)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get session summary: %w", err)
	}
	return summary, nil
}

// DeleteSessionSummary removes a persisted summary.
func (db *DB) DeleteSessionSummary(ctx context.Context, sessionID string) error {
	query := `DELETE FROM session_summaries WHERE session_id = ?`
	_, err := db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session summary: %w", err)
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

// Briefing represents a stored research briefing.
type Briefing struct {
	ID        int64
	Content   string
	CreatedAt string
}

// GetRecentBriefings retrieves the most recent N briefings ordered by creation time.
func (db *DB) GetRecentBriefings(ctx context.Context, limit int) ([]Briefing, error) {
	if limit <= 0 {
		limit = 5
	}
	query := `SELECT id, content, created_at FROM briefings ORDER BY created_at DESC LIMIT ?`
	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent briefings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var briefings []Briefing
	for rows.Next() {
		var b Briefing
		if err := rows.Scan(&b.ID, &b.Content, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan briefing: %w", err)
		}
		briefings = append(briefings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return briefings, nil
}

// Reminder represents a scheduled reminder.
type Reminder struct {
	ID        int64
	SessionID string
	Message   string
	RemindAt  time.Time
}

// AddReminder stores a new reminder in the database.
func (db *DB) AddReminder(ctx context.Context, sessionID, message string, remindAt time.Time) error {
	query := `INSERT INTO reminders (session_id, message, remind_at) VALUES (?, ?, ?)`
	_, err := db.ExecContext(ctx, query, sessionID, message, remindAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to add reminder: %w", err)
	}
	return nil
}

// GetPendingReminders returns all undelivered reminders whose remind_at time has passed.
func (db *DB) GetPendingReminders(ctx context.Context, now time.Time) ([]Reminder, error) {
	query := `SELECT id, session_id, message, remind_at FROM reminders WHERE delivered = 0 AND remind_at <= ?`
	rows, err := db.QueryContext(ctx, query, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to get pending reminders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Message, &r.RemindAt); err != nil {
			return nil, fmt.Errorf("failed to scan reminder: %w", err)
		}
		reminders = append(reminders, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return reminders, nil
}

// MarkReminderDelivered marks a reminder as delivered so it won't be returned again.
func (db *DB) MarkReminderDelivered(ctx context.Context, id int64) error {
	query := `UPDATE reminders SET delivered = 1 WHERE id = ?`
	_, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark reminder delivered: %w", err)
	}
	return nil
}

// MarkRemindersDelivered marks multiple reminders as delivered in a single transaction.
func (db *DB) MarkRemindersDelivered(ctx context.Context, ids []int64) error {
	const batchSize = 500
	if len(ids) == 0 {
		return nil
	}

	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}

		query := fmt.Sprintf("UPDATE reminders SET delivered = 1 WHERE id IN (%s)", strings.Join(placeholders, ","))
		_, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to mark reminders delivered (batch %d-%d): %w", start, end, err)
		}
	}
	return nil
}
