package db

import (
	"context"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	return db
}

func TestHasHeadline(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	url := "https://example.com/test"
	_ = db.AddHeadline(ctx, "Title", url)

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Exists", url, true},
		{"Not Exists", "https://other.com", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.HasHeadline(ctx, tt.url)
			if err != nil {
				t.Fatalf("HasHeadline failed: %v", err)
			}
			if got != tt.expected {
				t.Errorf("HasHeadline(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

func TestAddHeadline(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name    string
		title   string
		url     string
		wantErr bool
	}{
		{"Normal", "Title", "https://example.com", false},
		{"Duplicate URL", "Title 2", "https://example.com", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := db.AddHeadline(ctx, tt.title, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddHeadline() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetExistingHeadlines(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	_ = db.AddHeadline(ctx, "T1", "https://example.com/1")
	_ = db.AddHeadline(ctx, "T2", "https://example.com/2")

	tests := []struct {
		name     string
		urls     []string
		expected map[string]bool
	}{
		{
			name: "Some exist",
			urls: []string{"https://example.com/1", "https://example.com/3"},
			expected: map[string]bool{
				"https://example.com/1": true,
			},
		},
		{
			name:     "Empty input",
			urls:     []string{},
			expected: map[string]bool{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetExistingHeadlines(ctx, tt.urls)
			if err != nil {
				t.Fatalf("GetExistingHeadlines failed: %v", err)
			}
			if len(got) != len(tt.expected) {
				t.Errorf("got %d results, want %d", len(got), len(tt.expected))
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("url %s: got %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestAddHeadlines_Batch(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	headlines := []Headline{
		{Title: "Batch 1", URL: "https://example.com/b1"},
		{Title: "Batch 2", URL: "https://example.com/b2"},
	}

	err := db.AddHeadlines(ctx, headlines)
	if err != nil {
		t.Fatalf("AddHeadlines failed: %v", err)
	}

	for _, h := range headlines {
		exists, _ := db.HasHeadline(ctx, h.URL)
		if !exists {
			t.Errorf("Headline %s not found", h.URL)
		}
	}
}

func TestSaveBriefing(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	err := db.SaveBriefing(ctx, "Briefing content")
	if err != nil {
		t.Errorf("SaveBriefing failed: %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM briefings").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 briefing, got %d", count)
	}
}

func TestGetRecentBriefings(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	// Empty DB
	results, err := db.GetRecentBriefings(ctx, 5)
	if err != nil {
		t.Fatalf("GetRecentBriefings (empty) failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 briefings, got %d", len(results))
	}

	// Add multiple briefings
	_ = db.SaveBriefing(ctx, "Briefing One")
	_ = db.SaveBriefing(ctx, "Briefing Two")
	_ = db.SaveBriefing(ctx, "Briefing Three")

	// Limit respected
	results, err = db.GetRecentBriefings(ctx, 2)
	if err != nil {
		t.Fatalf("GetRecentBriefings (limit) failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 briefings, got %d", len(results))
	}

	// Default retrieves all 3
	results, err = db.GetRecentBriefings(ctx, 0)
	if err != nil {
		t.Fatalf("GetRecentBriefings (default) failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 briefings, got %d", len(results))
	}
}

func TestAddReminder(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	remindAt := time.Now().Add(1 * time.Hour)
	err := db.AddReminder(ctx, "cli-local", "Check Docker", remindAt)
	if err != nil {
		t.Fatalf("AddReminder failed: %v", err)
	}

	// Should not be pending yet (remind_at is in the future)
	pending, err := db.GetPendingReminders(ctx, time.Now())
	if err != nil {
		t.Fatalf("GetPendingReminders failed: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending reminders, got %d", len(pending))
	}
}

func TestGetPendingReminders(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	// Add a reminder in the past
	pastTime := time.Now().Add(-1 * time.Hour)
	_ = db.AddReminder(ctx, "cli-local", "Past reminder", pastTime)

	// Add a reminder in the future
	futureTime := time.Now().Add(1 * time.Hour)
	_ = db.AddReminder(ctx, "cli-local", "Future reminder", futureTime)

	pending, err := db.GetPendingReminders(ctx, time.Now())
	if err != nil {
		t.Fatalf("GetPendingReminders failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending reminder, got %d", len(pending))
	}
	if pending[0].Message != "Past reminder" {
		t.Errorf("expected 'Past reminder', got %q", pending[0].Message)
	}
}

func TestMarkReminderDelivered(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	pastTime := time.Now().Add(-1 * time.Hour)
	_ = db.AddReminder(ctx, "cli-local", "Deliver me", pastTime)

	pending, _ := db.GetPendingReminders(ctx, time.Now())
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	err := db.MarkReminderDelivered(ctx, pending[0].ID)
	if err != nil {
		t.Fatalf("MarkReminderDelivered failed: %v", err)
	}

	// Should be empty now
	pending, _ = db.GetPendingReminders(ctx, time.Now())
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after delivery, got %d", len(pending))
	}
}
