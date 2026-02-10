package agent

import (
	"context"
	"testing"
	"time"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// MockLLM defined in mock_test.go (same package)

func TestCompressSession(t *testing.T) {
	// 1. Setup DB
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	defer database.Close()

	// 2. Setup Session Service
	svc := session.InMemoryService()
	sessionID := "test-session"
	userID := sessionID

	ctx := context.Background()

	// Create session
	_, err = svc.Create(ctx, &session.CreateRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
	require.NoError(t, err)

	// Get session object for AppendEvent
	resp, err := svc.Get(ctx, &session.GetRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
	require.NoError(t, err)
	sess := resp.Session

	// Add events via AppendEvent(ctx, session, event)
	err = svc.AppendEvent(ctx, sess, &session.Event{
		Author:    "user",
		Timestamp: time.Now(),
		LLMResponse: model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{{Text: "Hello there"}},
			},
		},
	})
	require.NoError(t, err)

	err = svc.AppendEvent(ctx, sess, &session.Event{
		Author:    "model",
		Timestamp: time.Now(),
		LLMResponse: model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{{Text: "General Kenobi"}},
			},
		},
	})
	require.NoError(t, err)

	// 3. Setup Mock LLM
	// Create response struct manually to match model.LLMResponse structure
	summaryResponse := &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{{Text: "Summary: User greeted model with meme reference."}},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			TotalTokenCount: 100,
		},
	}

	// Create MockLLM instance
	// QueuedResponses is [][]*model.LLMResponse (calls -> chunks)
	mockLLM := &MockLLM{
		QueuedResponses: [][]*model.LLMResponse{
			{summaryResponse},
		},
	}

	// 4. Setup Agent
	cfg := &config.Config{
		Bot: config.BotConfig{
			SummaryPrompt: "Summarize this.",
		},
	}

	a := &Agent{
		cfg:            cfg,
		db:             database,
		sessionService: svc,
		flashLLM:       mockLLM,
	}

	// 5. Run Compression
	err = a.compressSession(ctx, sessionID)
	require.NoError(t, err)

	// 6. Verify Summary Saved to DB
	summary, err := database.GetSessionSummary(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, "Summary: User greeted model with meme reference.", summary)

	// 7. Verify Session Deleted/Cleared
	// Getting session should fail or return empty/new
	_, err = svc.Get(ctx, &session.GetRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
	// Depending on implementation, Get might return error if not found.
	// Or Create might be needed again.
	// Usually Delete removes it.
	assert.Error(t, err, "Session should be deleted from service")
}
