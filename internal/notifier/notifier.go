package notifier

import (
	"context"
)

// Notifier defines the interface for sending reports to various channels.
type Notifier interface {
	Send(ctx context.Context, message string) error
	Name() string
}
