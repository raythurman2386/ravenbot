package notifier

import (
	"context"
	"strings"
)

// Notifier defines the interface for sending reports to various channels.
type Notifier interface {
	Send(ctx context.Context, message string) error
	Name() string
	StartTyping(ctx context.Context) func()
}

func splitMessage(message string, limit int) []string {
	var chunks []string
	for len(message) > limit {
		// Find the last newline within the limit to avoid breaking lines or markdown
		index := strings.LastIndex(message[:limit], "\n")
		if index <= 0 {
			// No newline found, just split at the limit
			index = limit
			chunks = append(chunks, message[:index])
			message = message[index:]
		} else {
			chunks = append(chunks, message[:index])
			message = message[index+1:] // Skip the newline we split on
		}
	}
	if len(message) > 0 {
		chunks = append(chunks, message)
	}
	return chunks
}
