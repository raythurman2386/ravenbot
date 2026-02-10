package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session"
)

func TestClearSession(t *testing.T) {
	service := session.InMemoryService()
	a := &Agent{sessionService: service}

	ctx := context.Background()
	sessionID := "test-session"
	userID := sessionID
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
