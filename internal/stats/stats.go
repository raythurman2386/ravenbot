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

// Summary returns a human-friendly Markdown summary of bot stats.
func (s *Stats) Summary() string {
	uptime := s.Uptime()
	return fmt.Sprintf(
		"🐦 **RavenBot Stats**\n\n"+
			"⏱ **Uptime**: %s\n"+
			"💬 **Messages Processed**: %d\n"+
			"🔬 **Research Missions**: %d",
		formatDuration(uptime),
		s.messagesProcessed.Load(),
		s.missionsRun.Load(),
	)
}
