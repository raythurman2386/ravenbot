package agent

import (
	"context"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"

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

	// Technical tools should now be empty by default
	techTools := a.GetTechnicalTools()
	assert.Empty(t, techTools)

	// Test Core Tools
	coreTools := a.GetCoreTools()
	assert.Empty(t, coreTools)
}

func TestClearSession(t *testing.T) {
	service := session.InMemoryService()
	a := &Agent{sessionService: service}

	ctx := context.Background()
	sessionID := "test-session"
	userID := sessionID // userIDFromSession returns sessionID
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
