package stats

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecordMessage(t *testing.T) {
	t.Parallel()
	s := New()
	assert.Equal(t, int64(0), s.MessagesProcessed())

	s.RecordMessage()
	s.RecordMessage()
	s.RecordMessage()

	assert.Equal(t, int64(3), s.MessagesProcessed())
}

func TestRecordMission(t *testing.T) {
	t.Parallel()
	s := New()
	assert.Equal(t, int64(0), s.MissionsRun())

	s.RecordMission()
	s.RecordMission()

	assert.Equal(t, int64(2), s.MissionsRun())
}

func TestUptime(t *testing.T) {
	t.Parallel()
	s := New()
	time.Sleep(10 * time.Millisecond)
	assert.Greater(t, s.Uptime(), time.Duration(0))
}

func TestSummary(t *testing.T) {
	t.Parallel()
	s := New()
	s.RecordMessage()
	s.RecordMission()
	s.RecordTokens(1500, 500)

	summary := s.Summary()
	assert.Contains(t, summary, "RavenBot Stats")
	assert.Contains(t, summary, "Messages Processed")
	assert.Contains(t, summary, "Research Missions")
	assert.Contains(t, summary, "Tokens Used")
	assert.Contains(t, summary, "1,500 in")
	assert.Contains(t, summary, "500 out")
	assert.Contains(t, summary, "2,000 total")
}

func TestRecordTokens(t *testing.T) {
	t.Parallel()
	s := New()
	assert.Equal(t, int64(0), s.InputTokens())
	assert.Equal(t, int64(0), s.OutputTokens())

	s.RecordTokens(100, 50)
	s.RecordTokens(200, 75)

	assert.Equal(t, int64(300), s.InputTokens())
	assert.Equal(t, int64(125), s.OutputTokens())

	// Negative values should be ignored
	s.RecordTokens(-10, -5)
	assert.Equal(t, int64(300), s.InputTokens())
	assert.Equal(t, int64(125), s.OutputTokens())
}

func TestFormatNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero", 0, "0"},
		{"small", 42, "42"},
		{"hundreds", 999, "999"},
		{"thousands", 1234, "1,234"},
		{"millions", 1234567, "1,234,567"},
		{"exact boundary", 1000, "1,000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatNumber(tt.input))
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		dur      time.Duration
		expected string
	}{
		{"Minutes only", 15 * time.Minute, "15m"},
		{"Hours and minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},
		{"Days hours minutes", 50*time.Hour + 15*time.Minute, "2d 2h 15m"},
		{"Zero", 0, "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.dur)
			assert.True(t, strings.Contains(result, tt.expected) || result == tt.expected,
				"formatDuration(%v) = %q, want %q", tt.dur, result, tt.expected)
		})
	}
}
