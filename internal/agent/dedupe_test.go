package agent

import (
	"context"
	"ravenbot/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeduplication(t *testing.T) {
	ctx := context.Background()
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	defer database.Close()

	a := &Agent{db: database}

	// First call to AddHeadline
	err = a.db.AddHeadline(ctx, "Test Title", "https://example.com/item1")
	require.NoError(t, err)

	t.Run("CheckExisting", func(t *testing.T) {
		exists, err := a.db.HasHeadline(ctx, "https://example.com/item1")
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("CheckNew", func(t *testing.T) {
		exists, err := a.db.HasHeadline(ctx, "https://example.com/item2")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}
