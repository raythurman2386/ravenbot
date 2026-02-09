package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/genai"
)

// WebSearch performs a grounded Google Search via the Gemini API.
// It makes a standalone GenerateContent call with only the GoogleSearch
// grounding tool enabled, avoiding the Gemini API restriction that
// prevents mixing grounding tools with function-calling tools.
func WebSearch(ctx context.Context, apiKey, model, query string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is required for web search")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}

	slog.Info("WebSearch: performing grounded search", "query", query, "model", model)

	result, err := client.Models.GenerateContent(ctx, model, []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: query}},
		},
	}, &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("web search failed: %w", err)
	}

	if result == nil || len(result.Candidates) == 0 {
		return "", fmt.Errorf("web search returned no results")
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("web search returned empty content")
	}

	// Collect text from response.
	var sb strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
	}

	// Append grounding sources if available.
	if gm := candidate.GroundingMetadata; gm != nil && len(gm.GroundingChunks) > 0 {
		sb.WriteString("\n\n---\nSources:\n")
		for i, chunk := range gm.GroundingChunks {
			if chunk.Web != nil {
				sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, chunk.Web.Title, chunk.Web.URI))
			}
		}
	}

	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf("web search returned no text content")
	}

	return text, nil
}
