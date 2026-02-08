package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func BenchmarkMarkReminderDelivered_Sequential(b *testing.B) {
	dbPath := "bench_seq.db"
	_ = os.Remove(dbPath)
	defer func() { _ = os.Remove(dbPath) }()

	database, err := InitDB(dbPath)
	if err != nil {
		b.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	numReminders := 1000
	ids := make([]int64, 0, numReminders)

	// Pre-insert reminders
	for i := 0; i < numReminders; i++ {
		err := database.AddReminder(ctx, "session-1", fmt.Sprintf("reminder %d", i), time.Now())
		if err != nil {
			b.Fatalf("failed to add reminder: %v", err)
		}
	}

	// Get IDs
	pending, err := database.GetPendingReminders(ctx, time.Now().Add(time.Hour))
	if err != nil {
		b.Fatalf("failed to get pending reminders: %v", err)
	}
	for _, p := range pending {
		ids = append(ids, p.ID)
	}

	if len(ids) != numReminders {
		b.Fatalf("expected %d reminders, got %d", numReminders, len(ids))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate processing the batch sequentially
		for _, id := range ids {
			if err := database.MarkReminderDelivered(ctx, id); err != nil {
				b.Fatalf("failed to mark delivered: %v", err)
			}
		}
	}
}

func BenchmarkMarkRemindersDelivered_Batch(b *testing.B) {
	dbPath := "bench_batch.db"
	_ = os.Remove(dbPath)
	defer func() { _ = os.Remove(dbPath) }()

	database, err := InitDB(dbPath)
	if err != nil {
		b.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	numReminders := 1000
	ids := make([]int64, 0, numReminders)

	// Pre-insert reminders
	for i := 0; i < numReminders; i++ {
		err := database.AddReminder(ctx, "session-1", fmt.Sprintf("reminder %d", i), time.Now())
		if err != nil {
			b.Fatalf("failed to add reminder: %v", err)
		}
	}

	// Get IDs
	pending, err := database.GetPendingReminders(ctx, time.Now().Add(time.Hour))
	if err != nil {
		b.Fatalf("failed to get pending reminders: %v", err)
	}
	for _, p := range pending {
		ids = append(ids, p.ID)
	}

	if len(ids) != numReminders {
		b.Fatalf("expected %d reminders, got %d", numReminders, len(ids))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate processing the batch in one go
		if err := database.MarkRemindersDelivered(ctx, ids); err != nil {
			b.Fatalf("failed to mark delivered: %v", err)
		}
	}
}
