package stats

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Stats tracks lightweight bot operational metrics.
type Stats struct {
	startTime         time.Time
	messagesProcessed atomic.Int64
	missionsRun       atomic.Int64
	inputTokens       atomic.Int64
	outputTokens      atomic.Int64
}

// New creates a new Stats tracker pinned to the current time.
func New() *Stats {
	return &Stats{startTime: time.Now()}
}

// RecordMessage increments the messages-processed counter.
func (s *Stats) RecordMessage() {
	s.messagesProcessed.Add(1)
}

// RecordMission increments the missions-run counter.
func (s *Stats) RecordMission() {
	s.missionsRun.Add(1)
}

// RecordTokens adds to the cumulative input/output token counters.
func (s *Stats) RecordTokens(input, output int64) {
	if input > 0 {
		s.inputTokens.Add(input)
	}
	if output > 0 {
		s.outputTokens.Add(output)
	}
}

// Uptime returns the duration since the bot started.
func (s *Stats) Uptime() time.Duration {
	return time.Since(s.startTime)
}

// MessagesProcessed returns the total count of messages handled.
func (s *Stats) MessagesProcessed() int64 {
	return s.messagesProcessed.Load()
}

// MissionsRun returns the total count of research missions executed.
func (s *Stats) MissionsRun() int64 {
	return s.missionsRun.Load()
}

// InputTokens returns the total input tokens consumed.
func (s *Stats) InputTokens() int64 {
	return s.inputTokens.Load()
}

// OutputTokens returns the total output tokens generated.
func (s *Stats) OutputTokens() int64 {
	return s.outputTokens.Load()
}

// formatDuration formats a duration as a human-friendly string like "2d 5h 13m".
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatNumber adds comma separators to large numbers (e.g. 1234567 → "1,234,567").
func formatNumber(n int64) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// Summary returns a human-friendly Markdown summary of bot stats.
func (s *Stats) Summary() string {
	uptime := s.Uptime()
	input := s.inputTokens.Load()
	output := s.outputTokens.Load()
	return fmt.Sprintf(
		"🐦 **RavenBot Stats**\n\n"+
			"⏱ **Uptime**: %s\n"+
			"💬 **Messages Processed**: %d\n"+
			"🔬 **Research Missions**: %d\n"+
			"📊 **Tokens Used**: %s in / %s out (%s total)",
		formatDuration(uptime),
		s.messagesProcessed.Load(),
		s.missionsRun.Load(),
		formatNumber(input),
		formatNumber(output),
		formatNumber(input+output),
	)
}
