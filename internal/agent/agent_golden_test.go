package agent

import (
	"context"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
)

func TestChat_Golden(t *testing.T) {
	// 1. Setup Mock LLM
	mockResponseText := "Hello! I am a simulated RavenBot."
	mockLLM := &MockLLM{
		Responses: []*model.LLMResponse{
			NewTextResponse(mockResponseText),
		},
	}

	// 2. Setup ADK Components
	// We need a minimal config
	cfg := &config.Config{
		Bot: config.BotConfig{
			Model:        "mock-model",
			SystemPrompt: "You are a test bot.",
		},
	}

	// Initialize ADK Agent with Mock LLM
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:  "test-agent",
		Model: mockLLM,
	})
	require.NoError(t, err)

	// Initialize Session Service (In-Memory)
	sessionService := session.InMemoryService()

	// Initialize Runner
	adkRunner, err := runner.New(runner.Config{
		AppName:        "test-app",
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	require.NoError(t, err)

	// 3. Construct the RavenBot Agent manually
	// We don't use NewAgent because we want to inject our mocked components
	// and we don't want to trigger real API calls.
	ravenAgent := &Agent{
		cfg:            cfg,
		db:             &db.DB{}, // Mock DB if needed, but nil might panic if used
		adkLLM:         mockLLM,
		adkAgent:       adkAgent,
		adkRunner:      adkRunner,
		sessionService: sessionService,
	}

	// 4. Run the Golden Test
	ctx := context.Background()
	sessionID := "test-session-golden"

	// Create the session explicitly
	_, err = sessionService.Create(ctx, &session.CreateRequest{
		SessionID: sessionID,
		UserID:    "default-user",
		AppName:   "test-app",
	})
	require.NoError(t, err)

	userMessage := "Hello, bot!"

	response, err := ravenAgent.Chat(ctx, sessionID, userMessage)

	// 5. Assertions
	require.NoError(t, err)
	assert.Equal(t, mockResponseText, response)
	assert.Equal(t, 1, mockLLM.CallCount, "LLM should have been called exactly once")
}
