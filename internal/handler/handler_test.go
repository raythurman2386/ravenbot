package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/stats"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHandler creates a handler with a real in-memory DB for testing command routing.
// Agent is nil because we only test commands that don't call the AI model.
func newTestHandler(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		Bot: config.BotConfig{
			HelpMessage: "test help message",
		},
	}
	s := stats.New()
	h := New(nil, database, cfg, s, nil)
	return h, database
}

func TestHandleMessage_Help(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()

	var got string
	h.HandleMessage(context.Background(), "test-session", "/help", nil, func(reply string) {
		got = reply
	})

	assert.Equal(t, "test help message", got)
}

func TestHandleMessage_Uptime(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()

	var got string
	h.HandleMessage(context.Background(), "test-session", "/uptime", nil, func(reply string) {
		got = reply
	})

	assert.Contains(t, got, "RavenBot Stats")
	assert.Contains(t, got, "Messages Processed")
}

func TestHandleMessage_Reset(t *testing.T) {
	t.Parallel()
	// Reset requires bot.ClearSession — we skip since bot is nil.
	// This test verifies the routing reaches the reset branch.
	// Full integration test would need a real agent.
	t.Skip("Requires agent instance for ClearSession")
}

func TestHandleMessage_Remind(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()
	ctx := context.Background()

	t.Run("valid reminder", func(t *testing.T) {
		var got string
		h.HandleMessage(ctx, "test-session", "/remind 30m Check Docker", nil, func(reply string) {
			got = reply
		})
		assert.Contains(t, got, "Reminder set")
		assert.Contains(t, got, "30m")

		// Verify it was persisted
		pending, err := database.GetPendingReminders(ctx, time.Now().Add(1*time.Hour))
		require.NoError(t, err)
		assert.Len(t, pending, 1)
		assert.Equal(t, "Check Docker", pending[0].Message)
	})

	t.Run("missing message", func(t *testing.T) {
		var got string
		h.HandleMessage(ctx, "test-session", "/remind 30m", nil, func(reply string) {
			got = reply
		})
		assert.Contains(t, got, "Usage")
	})

	t.Run("invalid duration", func(t *testing.T) {
		var got string
		h.HandleMessage(ctx, "test-session", "/remind xyz Check Docker", nil, func(reply string) {
			got = reply
		})
		assert.Contains(t, got, "Invalid duration")
	})
}

func TestHandleMessage_Export_Empty(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()

	var got string
	h.HandleMessage(context.Background(), "test-session", "/export", nil, func(reply string) {
		got = reply
	})

	assert.Contains(t, got, "No briefings found")
}

func TestHandleMessage_Export_WithData(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()
	ctx := context.Background()

	_ = database.SaveBriefing(ctx, "Briefing content here")

	var got string
	h.HandleMessage(ctx, "test-session", "/export", nil, func(reply string) {
		got = reply
	})

	assert.Contains(t, got, "Exported 1 Briefing")
	assert.Contains(t, got, "Briefing content here")
}

func TestHandleMessage_EmptyText(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()

	called := false
	h.HandleMessage(context.Background(), "test-session", "   ", nil, func(_ string) {
		called = true
	})

	assert.False(t, called, "Reply should not be called for empty input")
}

func TestHandleMessage_TooLong(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()

	longText := strings.Repeat("a", MaxInputLength+1)

	var got string
	h.HandleMessage(context.Background(), "test-session", longText, nil, func(reply string) {
		got = reply
	})

	assert.Contains(t, got, "Message too long")
}

func TestHandleMessage_StatsIncrement(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()

	h.HandleMessage(context.Background(), "test-session", "/help", nil, func(_ string) {})
	h.HandleMessage(context.Background(), "test-session", "/uptime", nil, func(_ string) {})

	assert.Equal(t, int64(2), h.stats.MessagesProcessed())
}

func TestIsAdequateReport(t *testing.T) {
	t.Parallel()

	// A realistic "good" report that exceeds minReportLength (1024 bytes).
	goodReport := strings.Repeat("Today's briefing covers weather, tech news, and project updates. ", 30)

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{
			name: "adequate report",
			in:   goodReport,
			want: true,
		},
		{
			name: "too short",
			in:   "Sorry, I could not complete the request.",
			want: false,
		},
		{
			name: "exactly at threshold",
			in:   strings.Repeat("x", minReportLength),
			want: true,
		},
		{
			name: "one byte below threshold",
			in:   strings.Repeat("x", minReportLength-1),
			want: false,
		},
		{
			name: "empty report",
			in:   "",
			want: false,
		},
		{
			name: "contains unable to fulfill",
			in:   goodReport + " I am unable to fulfill this request due to tool errors.",
			want: false,
		},
		{
			name: "contains tools are not found",
			in:   goodReport + " The tools are not found in my current environment.",
			want: false,
		},
		{
			name: "contains tools are not available",
			in:   goodReport + " The tools are not available right now.",
			want: false,
		},
		{
			name: "contains not found or available to me",
			in:   goodReport + " These tools are not found or available to me.",
			want: false,
		},
		{
			name: "contains encountering persistent errors",
			in:   goodReport + " I am encountering persistent errors with the tools.",
			want: false,
		},
		{
			name: "failure signal case insensitive",
			in:   goodReport + " I am UNABLE TO FULFILL this request.",
			want: false,
		},
		{
			name: "no false positive on partial match",
			in:   goodReport + " The system is fully available and tools are working.",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isAdequateReport(tt.in)
			assert.Equal(t, tt.want, got, "isAdequateReport(%q...)", tt.in[:min(len(tt.in), 60)])
		})
	}
}

func TestDeliverReminders(t *testing.T) {
	t.Parallel()
	h, database := newTestHandler(t)
	defer func() { _ = database.Close() }()
	ctx := context.Background()

	// Add a past reminder
	_ = database.AddReminder(ctx, "test-session", "Time to deploy", time.Now().Add(-1*time.Hour))

	// Register a reply function
	var delivered string
	h.mu.Lock()
	h.replies["test-session"] = func(msg string) { delivered = msg }
	h.mu.Unlock()

	h.DeliverReminders(ctx)

	assert.Contains(t, delivered, "Time to deploy")

	// Should be marked as delivered
	pending, _ := database.GetPendingReminders(ctx, time.Now())
	assert.Len(t, pending, 0)
}
