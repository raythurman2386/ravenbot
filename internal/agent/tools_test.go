package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestHandleToolCall(t *testing.T) {
	// Mock server for tool execution
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock response"))
	}))
	defer server.Close()

	ctx := context.Background()

	t.Run("FetchRSS", func(t *testing.T) {
		call := &genai.FunctionCall{
			Name: "FetchRSS",
			Args: map[string]any{"url": server.URL},
		}
		// Since FetchRSS returns a slice of items, and the server returns a non-RSS response,
		// it might error, but we're testing the routing here.
		// Actually, let's just check if it returns without crashing and reaches the right case.
		_, err := HandleToolCall(ctx, call)
		assert.Error(t, err) // Expecting error due to invalid RSS in mock server
	})

	t.Run("ScrapePage", func(t *testing.T) {
		call := &genai.FunctionCall{
			Name: "ScrapePage",
			Args: map[string]any{"url": server.URL},
		}
		result, err := HandleToolCall(ctx, call)
		assert.NoError(t, err)
		assert.Equal(t, "mock response", result)
	})

	t.Run("Unknown", func(t *testing.T) {
		call := &genai.FunctionCall{
			Name: "UnknownTool",
		}
		result, err := HandleToolCall(ctx, call)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
}
