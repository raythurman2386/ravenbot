package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/stats"
	"github.com/stretchr/testify/assert"
)

type mockBot struct {
	chatFunc       func(ctx context.Context, sessionID, message string) (string, error)
	runMissionFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *mockBot) Chat(ctx context.Context, sessionID, message string) (string, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, sessionID, message)
	}
	return "", nil
}

func (m *mockBot) RunMission(ctx context.Context, prompt string) (string, error) {
	if m.runMissionFunc != nil {
		return m.runMissionFunc(ctx, prompt)
	}
	return "", nil
}

func (m *mockBot) ClearSession(sessionID string) {}

func TestErrorLeakage(t *testing.T) {
	internalError := "SQL injection detected at 192.168.1.1: secret_key=abc123"

	bot := &mockBot{
		chatFunc: func(ctx context.Context, sessionID, message string) (string, error) {
			return "", errors.New(internalError)
		},
		runMissionFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", errors.New(internalError)
		},
	}

	cfg := &config.Config{
		Bot: config.BotConfig{
			StatusPrompt: "status",
		},
	}
	h := New(bot, nil, cfg, stats.New(), nil)

	t.Run("handleChat error leakage", func(t *testing.T) {
		var got string
		h.handleChat(context.Background(), "test", "hello", func(reply string) {
			got = reply
		})
		assert.NotContains(t, got, internalError)
	})

	t.Run("handleStatus error leakage", func(t *testing.T) {
		var got string
		h.handleStatus(context.Background(), "test", func(reply string) {
			// We skip the first "Checking server health..." reply
			if reply != "🔍 Checking server health..." {
				got = reply
			}
		})
		assert.NotContains(t, got, internalError)
	})

	t.Run("handleResearch error leakage", func(t *testing.T) {
		var got string
		h.handleResearch(context.Background(), "/research topic", func(reply string) {
			if !assert.ObjectsAreEqual(reply, "🔬 Starting research on: **topic**...") {
				got = reply
			}
		})
		assert.NotContains(t, got, internalError)
	})

	t.Run("handleJules error leakage", func(t *testing.T) {
		var got string
		h.handleJules(context.Background(), "test", "/jules owner/repo task", func(reply string) {
			if !assert.ObjectsAreEqual(reply, "🤖 Delegating to Jules for **owner/repo**: task") {
				got = reply
			}
		})
		assert.NotContains(t, got, internalError)
	})
}
