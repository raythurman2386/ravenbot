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

	summary := s.Summary()
	assert.Contains(t, summary, "RavenBot Stats")
	assert.Contains(t, summary, "Messages Processed")
	assert.Contains(t, summary, "Research Missions")
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
