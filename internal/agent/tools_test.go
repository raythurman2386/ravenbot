package agent

import (
	"context"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/mcp"

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

	// Test Technical Tools
	techTools := a.GetTechnicalTools()
	assert.NotEmpty(t, techTools)
	techNames := []string{"google_search"}
	for _, name := range techNames {
		found := false
		for _, tool := range techTools {
			if tool.Name() == name {
				found = true
				break
			}
		}
		assert.True(t, found, "Technical tool %s not found", name)
	}

	// Test Core Tools
	coreTools := a.GetCoreTools()
	assert.NotEmpty(t, coreTools)
	coreNames := []string{"ReadMCPResource"}
	for _, name := range coreNames {
		found := false
		for _, tool := range coreTools {
			if tool.Name() == name {
				found = true
				break
			}
		}
		assert.True(t, found, "Core tool %s not found", name)
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
