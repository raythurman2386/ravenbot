package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestSubAgentDelegation_Native(t *testing.T) {
	// Goal: Verify that the ADK's native SubAgent feature works as expected.
	// This proves that we can safely refactor away from the manual "functiontool" wrapper.

	// 1. Setup Sub-Agent (The "ResearchAssistant")
	// It should receive a request and return a response.
	subMockResponse := "I have researched the topic."
	subMockLLM := &MockLLM{
		QueuedResponses: [][]*model.LLMResponse{
			{NewTextResponse(subMockResponse)},
		},
	}

	subAgent, err := llmagent.New(llmagent.Config{
		Name:  "TestSubAgent",
		Model: subMockLLM,
	})
	require.NoError(t, err)

	// 2. Setup Root Agent (The "RavenBot")
	// It should have the SubAgent registered natively.
	// It intercepts the user message, calls the sub-agent, and returns the result.
	rootMockLLM := &MockLLM{
		QueuedResponses: [][]*model.LLMResponse{
			// Response 1: Root agent decides to call the sub-agent
			{NewToolCallResponse("TestSubAgent", map[string]any{"instruction": "do research"})},
			// Response 2: Root agent receives the sub-agent's output and summarizes
			{NewTextResponse("The sub-agent says: " + subMockResponse)},
		},
	}

	rootAgent, err := llmagent.New(llmagent.Config{
		Name:      "TestRootAgent",
		Model:     rootMockLLM,
		SubAgents: []agent.Agent{subAgent}, // <--- The Feature Under Test
	})
	require.NoError(t, err)

	// 3. Run the interaction
	ctx := context.Background()
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        "test-app",
		Agent:          rootAgent,
		SessionService: sessionService,
	})
	require.NoError(t, err)

	// Create session
	sess, err := sessionService.Create(ctx, &session.CreateRequest{
		SessionID: "subagent-test-session",
		UserID:    "user1",
		AppName:   "test-app",
	})
	require.NoError(t, err)

	// Execute Run
	events := r.Run(ctx, "user1", sess.Session.ID(), &genai.Content{
		Parts: []*genai.Part{{Text: "Please research this."}},
	}, agent.RunConfig{})

	// Collect outputs
	var finalOutput string
	for event, err := range events {
		require.NoError(t, err)
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				finalOutput += part.Text
			}
		}
	}

	// 4. Verification
	assert.Contains(t, finalOutput, "The sub-agent says: I have researched the topic.")
}
