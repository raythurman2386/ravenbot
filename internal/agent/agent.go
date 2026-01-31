package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/mcp"

	"google.golang.org/genai"
)

type Agent struct {
	client     *genai.Client
	cfg        *config.Config
	db         *db.DB
	sessions   map[string]*genai.Chat // Chat sessions by user/channel ID
	summaries  map[string]string      // Compressed context summaries by session ID
	mu         sync.RWMutex
	mcpClients map[string]*mcp.Client
	tools      []*genai.Tool
}

const model = "gemini-3-flash-preview"

// Raven's conversational persona
const systemPrompt = `You are ravenbot, a sophisticated, friendly, and proactive AI assistant built by Ray Thurman. 
You are inspired by autonomous agents like OpenClaw (Clawdbot), acting as a digital partner rather than just a tool.

Your Personality:
- **Warm & Professional**: You are knowledgeable but approachable. Use a conversational tone.
- **Proactive**: Don't just answer questions; suggest next steps or identify related information the user might find useful.
- **Subtly Humorous**: You have a dry, technical sense of humor.
- **Passionate**: You genuinely care about Go, Python, AI/LLMs, and Geospatial tech.

Your Strategic Use of Memory & Tools:
1. **Recall (First Principle)**: At the start of a conversation or when asked a personal question, use your 'memory' tools (e.g., memory_search_nodes or memory_read_graph) to recall who the user is, their current projects, and their preferences.
2. **Learn & Adapt**: When you learn something new about the user (a project they are starting, a technology they like), proactively use 'memory_add_observations' or 'memory_create_entities' to store it.
3. **Multi-Step Execution**: Use your MCP tools (GitHub, Git, Filesystem) and Shell tools to perform real work. If a user asks about a repo, check it on GitHub. If they ask about system health, use ShellExecute.
4. **Research Deep Dives**: Use FetchRSS and BrowseWeb to get real-time data. Always prioritize fresh information.

Standard Commands:
- /research <topic> - Deep dive into a technical topic
- /jules <repo> <task> - Delegate a coding task to Jules
- /status - Check server health
- /help - Show available commands

When responding:
- Address the user by name if you know it from memory.
- Use emojis sparingly but effectively to convey personality (🐦, 🔬, 🤖).
- Always use Markdown for code blocks and headers to keep things readable.`

func NewAgent(ctx context.Context, cfg *config.Config, database *db.DB) (*Agent, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	agent := &Agent{
		client:     client,
		cfg:        cfg,
		db:         database,
		sessions:   make(map[string]*genai.Chat),
		summaries:  make(map[string]string),
		mcpClients: make(map[string]*mcp.Client),
		// Start with native tools
		tools: append([]*genai.Tool{}, RavenTools...),
	}

	// Add generic MCP resource tool
	agent.registerTool(GetReadResourceTool())

	// Initialize MCP Servers
	for name, serverCfg := range cfg.MCPServers {
		slog.Info("Initializing MCP Server", "name", name, "command", serverCfg.Command)
		var mcpClient *mcp.Client
		if strings.HasPrefix(serverCfg.Command, "http://") || strings.HasPrefix(serverCfg.Command, "https://") {
			mcpClient = mcp.NewSSEClient(serverCfg.Command)
		} else {
			mcpClient = mcp.NewStdioClient(serverCfg.Command, serverCfg.Args)
		}

		if err := mcpClient.Start(); err != nil {
			slog.Error("Failed to start MCP server", "name", name, "error", err)
			continue
		}

		if err := mcpClient.Initialize(); err != nil {
			slog.Error("Failed to initialize MCP server", "name", name, "error", err)
			mcpClient.Close()
			continue
		}

		// Register Tools
		tools, err := mcpClient.ListTools()
		if err != nil {
			slog.Error("Failed to list tools from MCP server", "name", name, "error", err)
		} else {
			for _, tool := range tools {
				genTool, err := mcpToolToGenAI(name, tool)
				if err != nil {
					slog.Error("Failed to convert MCP tool", "name", tool.Name, "server", name, "error", err)
					continue
				}
				agent.registerTool(genTool)
				slog.Info("Registered MCP Tool", "name", genTool.Name, "server", name)
			}
		}

		// Register Resources as virtual tools
		resources, err := mcpClient.ListResources()
		if err != nil {
			slog.Debug("MCP server does not support resources", "name", name)
		} else {
			for _, res := range resources {
				genTool := mcpResourceToGenAI(name, res)
				agent.registerTool(genTool)
				slog.Info("Registered MCP Resource as Tool", "uri", res.URI, "server", name)
			}
		}

		agent.mcpClients[name] = mcpClient
	}

	return agent, nil
}

func (a *Agent) registerTool(genTool *genai.FunctionDeclaration) {
	if len(a.tools) > 0 && a.tools[0].FunctionDeclarations != nil {
		a.tools[0].FunctionDeclarations = append(a.tools[0].FunctionDeclarations, genTool)
	} else {
		a.tools = append(a.tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{genTool},
		})
	}
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

	// Check if we have a compressed summary for this session
	a.mu.RLock()
	summary := a.summaries[sessionID]
	a.mu.RUnlock()

	effectivePrompt := systemPrompt
	if summary != "" {
		effectivePrompt = fmt.Sprintf("%s\n\n### CONTEXT SUMMARY OF PREVIOUS CONVERSATION:\n%s", systemPrompt, summary)
	}

	chat, err := a.client.Chats.Create(ctx, model, &genai.GenerateContentConfig{
		Tools: a.tools,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: effectivePrompt},
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

	response, err := a.processResponse(ctx, chat, resp)
	if err != nil {
		return "", err
	}

	// Proactively check if context needs compression (async)
	go func() {
		if err := a.checkAndCompressContext(context.Background(), sessionID, chat); err != nil {
			slog.Error("Context compression check failed", "sessionID", sessionID, "error", err)
		}
	}()

	return response, nil
}

// RunMission executes a one-shot research mission (no session persistence)
func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	chat, err := a.client.Chats.Create(ctx, model, &genai.GenerateContentConfig{
		Tools: a.tools,
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
					// We log the error but try to continue or return it to the model
					// Ideally, return an error message to the model so it can retry
					slog.Error("Tool execution failed", "tool", part.FunctionCall.Name, "error", err)
					result = map[string]string{"error": err.Error()}
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

// checkAndCompressContext counts tokens and triggers summarization if threshold is hit
func (a *Agent) checkAndCompressContext(ctx context.Context, sessionID string, chat *genai.Chat) error {
	// gemini-3-flash-preview has a 1M token window.
	// We'll set a conservative threshold or use the requested 80%.
	const tokenLimit = 1000000
	const threshold = 0.8

	// Count tokens in the current history
	resp, err := a.client.Models.CountTokens(ctx, model, chat.History(false), nil)
	if err != nil {
		return fmt.Errorf("failed to count tokens: %w", err)
	}

	if float64(resp.TotalTokens) < tokenLimit*threshold {
		return nil
	}

	slog.Info("Context window threshold reached, compressing...", "sessionID", sessionID, "tokens", resp.TotalTokens)

	// 1. Generate a concise summary of the conversation
	summaryPrompt := "SYSTEM: Our conversation history is very long. Please provide a concise but comprehensive summary of our interaction so far. Focus on key technical decisions, projects we discussed, user preferences, and any important entities. This summary will serve as our 'anchor context' for a fresh session."

	summaryResp, err := chat.SendMessage(ctx, genai.Part{Text: summaryPrompt})
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	var summaryText strings.Builder
	if len(summaryResp.Candidates) > 0 && summaryResp.Candidates[0].Content != nil {
		for _, p := range summaryResp.Candidates[0].Content.Parts {
			if p.Text != "" {
				summaryText.WriteString(p.Text)
			}
		}
	}

	if summaryText.Len() == 0 {
		return fmt.Errorf("empty summary generated")
	}

	// 2. Store the summary and flag the session for recreation
	// By deleting the session from the map, getOrCreateSession will
	// rebuild it using the new summary in the system instructions.
	a.mu.Lock()
	a.summaries[sessionID] = summaryText.String()
	delete(a.sessions, sessionID)
	a.mu.Unlock()

	slog.Info("Context compressed successfully", "sessionID", sessionID, "summaryLength", summaryText.Len())
	return nil
}
