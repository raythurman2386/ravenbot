package agent

import (
	"context"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
)

func TestChat_Golden(t *testing.T) {
	// 1. Setup Mock LLMs
	// Flash Mock:
	// Call 1: Classification -> "Simple"
	// Call 2: Chat response -> "Hello! I am a simulated RavenBot."
	mockChatResponse := "Hello! I am a simulated RavenBot."
	mockFlashLLM := &MockLLM{
		QueuedResponses: [][]*model.LLMResponse{
			{NewTextResponse("Simple")},
			{NewTextResponse(mockChatResponse)},
		},
	}

	// Pro Mock (not used in this simple flow)
	mockProLLM := &MockLLM{
		QueuedResponses: [][]*model.LLMResponse{},
	}

	// 2. Setup ADK Components
	cfg := &config.Config{
		Bot: config.BotConfig{
			FlashModel:   "mock-flash",
			ProModel:     "mock-pro",
			SystemPrompt: "You are a test bot.",
		},
	}

	// Initialize ADK Agents
	flashAgent, err := llmagent.New(llmagent.Config{
		Name:  "test-flash",
		Model: mockFlashLLM,
	})
	require.NoError(t, err)

	proAgent, err := llmagent.New(llmagent.Config{
		Name:  "test-pro",
		Model: mockProLLM,
	})
	require.NoError(t, err)

	// Initialize Session Service (In-Memory)
	sessionService := session.InMemoryService()

	// Initialize Runners
	flashRunner, err := runner.New(runner.Config{
		AppName:        "test-app",
		Agent:          flashAgent,
		SessionService: sessionService,
	})
	require.NoError(t, err)

	proRunner, err := runner.New(runner.Config{
		AppName:        "test-app",
		Agent:          proAgent,
		SessionService: sessionService,
	})
	require.NoError(t, err)

	// 3. Construct the RavenBot Agent manually
	ravenAgent := &Agent{
		cfg:            cfg,
		db:             nil,
		flashLLM:       mockFlashLLM,
		proLLM:         mockProLLM,
		flashRunner:    flashRunner,
		proRunner:      proRunner,
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
	assert.Equal(t, mockChatResponse, response)
	assert.Equal(t, 2, mockFlashLLM.CallCount, "Flash LLM should have been called twice (classify + chat)")
	assert.Equal(t, 0, mockProLLM.CallCount, "Pro LLM should not have been called")
}
