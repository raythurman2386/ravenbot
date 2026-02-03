package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/tools"
)

func BenchmarkDeduplication(b *testing.B) {
	ctx := context.Background()
	database, err := db.InitDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	a := &Agent{db: database}

	// Prepare some items
	numItems := 100
	items := make([]tools.RSSItem, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = tools.RSSItem{
			Title: fmt.Sprintf("Title %d", i),
			Link:  fmt.Sprintf("https://example.com/%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// We need to clear the database or use new items each time to actually measure deduplication
		// but for N+1 query issue, even if they all exist or none exist, it still does N queries.

		// Let's simulate a mix of existing and new items.
		// For simplicity, let's just run it as is.
		// The first iteration will insert them, subsequent will just check.

		if _, err := a.deduplicateRSSItems(ctx, items); err != nil {
			b.Fatal(err)
		}
	}
}
