package db

import (
	"context"
	"testing"
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
