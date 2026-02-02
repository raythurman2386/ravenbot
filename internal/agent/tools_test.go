package agent

import (
	"context"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/mcp"
	"github.com/raythurman2386/ravenbot/internal/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session"
)

func TestGetRavenTools(t *testing.T) {
	a := &Agent{
		cfg: &config.Config{
			JulesAPIKey: "test-key",
		},
	}
	toolsList := a.GetRavenTools()
	assert.NotEmpty(t, toolsList)

	for _, tool := range toolsList {
		t.Logf("Found tool: %s", tool.Name())
	}

	expectedNames := []string{
		"FetchRSS",
		"ScrapePage",
		"ShellExecute",
		"BrowseWeb",
		"JulesTask",
		"google_search", // Changed from GoogleSearch
		"ReadMCPResource",
	}

	for _, name := range expectedNames {
		found := false
		for _, tool := range toolsList {
			if tool == nil {
				t.Fatalf("Found nil tool in list when looking for %s", name)
			}
			if tool.Name() == name {
				found = true
				break
			}
		}
		assert.True(t, found, "Tool %s not found", name)
	}
}

func TestGetMCPTools(t *testing.T) {
	ctx := context.Background()

	// Test the logic with an empty client map
	a := &Agent{
		mcpClients: make(map[string]*mcp.Client),
	}

	mcpTools := a.GetMCPTools(ctx)
	assert.Empty(t, mcpTools)
}

func TestDeduplicationToolLogic(t *testing.T) {
	ctx := context.Background()
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	defer database.Close()

	a := &Agent{db: database}

	// This tests the deduplicateRSSItems helper used by the FetchRSS tool
	err = a.db.AddHeadline(ctx, "Test Title", "https://example.com/item1")
	require.NoError(t, err)

	items := []tools.RSSItem{
		{Title: "Title 1", Link: "https://example.com/item1"}, // Duplicate
		{Title: "Title 2", Link: "https://example.com/item2"}, // New
	}

	newItems, err := a.deduplicateRSSItems(ctx, items)
	assert.NoError(t, err)
	assert.Len(t, newItems, 1)
	assert.Equal(t, "https://example.com/item2", newItems[0].Link)
}

func TestClearSession(t *testing.T) {
	service := session.InMemoryService()
	a := &Agent{sessionService: service}

	ctx := context.Background()
	userID := "default-user"
	sessionID := "test-session"
	appName := AppName

	// Create a session
	_, err := service.Create(ctx, &session.CreateRequest{
		UserID:    userID,
		SessionID: sessionID,
		AppName:   appName,
	})
	require.NoError(t, err)

	// Verify it exists
	resp, err := service.Get(ctx, &session.GetRequest{
		UserID:    userID,
		SessionID: sessionID,
		AppName:   appName,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp.Session)

	// Clear it
	a.ClearSession(sessionID)

	// Verify it's gone
	_, err = service.Get(ctx, &session.GetRequest{
		UserID:    userID,
		SessionID: sessionID,
		AppName:   appName,
	})
	assert.Error(t, err)
}
