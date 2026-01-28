package agent

import (
	"context"
	"fmt"
	"ravenbot/internal/config"
	"ravenbot/internal/db"
	"strings"

	"google.golang.org/genai"
)

type Agent struct {
	client *genai.Client
	cfg    *config.Config
	db     *db.DB
}

func NewAgent(ctx context.Context, cfg *config.Config, database *db.DB) (*Agent, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &Agent{
		client: client,
		cfg:    cfg,
		db:     database,
	}, nil
}

func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	model := "gemini-3-pro-preview"

	// Create a new session for multi-turn interaction
	chat, err := a.client.Chats.Create(ctx, model, &genai.GenerateContentConfig{
		Tools: RavenTools,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: `You are RavenBot, a sophisticated technical research assistant. 
Your goal is to generate high-quality, structured briefings or research reports in Markdown format.
Focus on providing accurate, technical, and well-sourced information.

Formatting Requirements:
- Use a clear # Title.
- Use ## Sections for major topics or findings.
- Provide [Source Name](link) where applicable.
- Ensure the tone is professional yet engaging.`},
			},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create chat session: %w", err)
	}

	resp, err := chat.SendMessage(ctx, genai.Part{Text: prompt})
	if err != nil {
		return "", fmt.Errorf("failed to send initial message: %w", err)
	}

	for {
		// Check for function calls in the last candidate
		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			break
		}

		var toolResponses []genai.Part
		hasCalls := false

		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				hasCalls = true
				result, err := a.handleToolCall(ctx, part.FunctionCall)
				if err != nil {
					return "", fmt.Errorf("tool call %s failed: %w", part.FunctionCall.Name, err)
				}

				toolResponses = append(toolResponses, genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     part.FunctionCall.Name,
						Response: map[string]any{"result": result},
					},
				})
			}
		}

		if !hasCalls {
			break
		}

		// Send tool results back to the model
		resp, err = chat.SendMessage(ctx, toolResponses...)
		if err != nil {
			return "", fmt.Errorf("failed to send tool responses: %w", err)
		}
	}

	// Return the final text response
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		var finalParts []string
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				finalParts = append(finalParts, part.Text)
			}
		}
		return strings.Join(finalParts, "\n"), nil
	}

	return "", fmt.Errorf("no response from Gemini")
}
