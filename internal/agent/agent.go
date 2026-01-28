package agent

import (
	"context"
	"fmt"
	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"strings"
	"sync"

	"google.golang.org/genai"
)

type Agent struct {
	client   *genai.Client
	cfg      *config.Config
	db       *db.DB
	sessions map[string]*genai.Chat // Chat sessions by user/channel ID
	mu       sync.RWMutex
}

const model = "gemini-3-flash-preview"

// Raven's conversational persona
const systemPrompt = `You are ravenbot, a friendly and knowledgeable AI assistant built by Ray Thurman.

Your personality:
- Helpful, conversational, and approachable
- Technically proficient but able to explain things clearly
- Enthusiastic about Go, Python, AI/LLMs, and geospatial technology
- You have a subtle sense of humor

Your capabilities:
- General conversation and Q&A on any topic
- Technical research using web browsing and RSS feeds
- Code assistance and explanations
- System health checks (disk, memory, uptime)
- Delegating complex coding tasks to Jules (Google's AI coding agent)

When responding:
- Be concise but thorough
- Use markdown formatting for code and structured content
- If you don't know something, say so honestly
- For research tasks, use your tools to gather real information

Available commands users might ask about:
- /research <topic> - Deep dive into a technical topic
- /jules <repo> <task> - Delegate a coding task to Jules
- /status - Check server health
- /help - Show available commands`

func NewAgent(ctx context.Context, cfg *config.Config, database *db.DB) (*Agent, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &Agent{
		client:   client,
		cfg:      cfg,
		db:       database,
		sessions: make(map[string]*genai.Chat),
	}, nil
}

// getOrCreateSession retrieves an existing chat session or creates a new one
func (a *Agent) getOrCreateSession(ctx context.Context, sessionID string) (*genai.Chat, error) {
	a.mu.RLock()
	chat, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if exists {
		return chat, nil
	}

	// Create new session
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if chat, exists = a.sessions[sessionID]; exists {
		return chat, nil
	}

	chat, err := a.client.Chats.Create(ctx, model, &genai.GenerateContentConfig{
		Tools: RavenTools,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}

	a.sessions[sessionID] = chat
	return chat, nil
}

// ClearSession removes a chat session (useful for /reset command)
func (a *Agent) ClearSession(sessionID string) {
	a.mu.Lock()
	delete(a.sessions, sessionID)
	a.mu.Unlock()
}

// Chat handles conversational messages with session persistence
func (a *Agent) Chat(ctx context.Context, sessionID, message string) (string, error) {
	chat, err := a.getOrCreateSession(ctx, sessionID)
	if err != nil {
		return "", err
	}

	resp, err := chat.SendMessage(ctx, genai.Part{Text: message})
	if err != nil {
		// Session might be stale, try recreating
		a.ClearSession(sessionID)
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	return a.processResponse(ctx, chat, resp)
}

// RunMission executes a one-shot research mission (no session persistence)
func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	chat, err := a.client.Chats.Create(ctx, model, &genai.GenerateContentConfig{
		Tools: RavenTools,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: `You are ravenbot, a sophisticated technical research assistant. 
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

	return a.processResponse(ctx, chat, resp)
}

// processResponse handles tool calls and extracts final text
func (a *Agent) processResponse(ctx context.Context, chat *genai.Chat, resp *genai.GenerateContentResponse) (string, error) {
	for {
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

		var err error
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
