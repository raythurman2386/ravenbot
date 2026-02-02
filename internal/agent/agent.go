package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/mcp"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

const AppName = "ravenbot"

type Agent struct {
	cfg        *config.Config
	db         *db.DB
	mcpClients map[string]*mcp.Client
	mu         sync.RWMutex

	// Summaries for context compression
	summaries   map[string]string
	summariesMu sync.RWMutex

	// ADK components
	adkLLM         model.LLM
	adkAgent       agent.Agent
	adkRunner      *runner.Runner
	sessionService session.Service
}

func NewAgent(ctx context.Context, cfg *config.Config, database *db.DB) (*Agent, error) {
	// 1. Initialize ADK Model (LLM)
	adkLLM, err := gemini.NewModel(ctx, cfg.Bot.Model, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK Gemini model: %w", err)
	}

	a := &Agent{
		cfg:        cfg,
		db:         database,
		mcpClients: make(map[string]*mcp.Client),
		summaries:  make(map[string]string),
		adkLLM:     adkLLM,
	}

	// 2. Initialize MCP Servers
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

		a.mu.Lock()
		a.mcpClients[name] = mcpClient
		a.mu.Unlock()
	}

	// 3. Gather Tools
	nativeTools := a.GetRavenTools()
	mcpTools := a.GetMCPTools(ctx)
	allTools := append(nativeTools, mcpTools...)

	// 4. Create ADK LLMAgent
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot",
		Model:       adkLLM,
		Description: "RavenBot autonomous research agent",
		InstructionProvider: func(ctx agent.ReadonlyContext) (string, error) {
			a.summariesMu.RLock()
			summary := a.summaries[ctx.SessionID()]
			a.summariesMu.RUnlock()

			if summary != "" {
				return fmt.Sprintf("%s\n\n### CONTEXT SUMMARY OF PREVIOUS CONVERSATION:\n%s", a.cfg.Bot.SystemPrompt, summary), nil
			}
			return a.cfg.Bot.SystemPrompt, nil
		},
		Tools: allTools,
		AfterModelCallbacks: []llmagent.AfterModelCallback{
			func(ctx agent.CallbackContext, llmResponse *model.LLMResponse, llmResponseError error) (*model.LLMResponse, error) {
				if llmResponseError != nil || llmResponse == nil || llmResponse.UsageMetadata == nil {
					return llmResponse, llmResponseError
				}

				// Check if context needs compression
				totalTokens := llmResponse.UsageMetadata.TotalTokenCount
				threshold := float64(a.cfg.Bot.TokenLimit) * a.cfg.Bot.TokenThreshold

				if float64(totalTokens) >= threshold {
					slog.Info("Context window threshold reached, triggering compression", "sessionID", ctx.SessionID(), "tokens", totalTokens)
					go a.compressContext(ctx.SessionID())
				}

				return llmResponse, nil
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK agent: %w", err)
	}
	a.adkAgent = adkAgent

	// 5. Create ADK Runner
	sessionService := session.InMemoryService()
	adkRunner, err := runner.New(runner.Config{
		AppName:        AppName,
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK runner: %w", err)
	}
	a.adkRunner = adkRunner
	a.sessionService = sessionService

	return a, nil
}

// compressContext generates a summary of the session and clears it to free up the context window.
func (a *Agent) compressContext(sessionID string) {
	ctx := context.Background()
	slog.Info("Generating conversation summary for compression", "sessionID", sessionID)

	summary, err := a.RunMission(ctx, a.cfg.Bot.SummaryPrompt)
	if err != nil {
		slog.Error("Failed to generate summary for context compression", "sessionID", sessionID, "error", err)
		return
	}

	a.summariesMu.Lock()
	a.summaries[sessionID] = summary
	a.summariesMu.Unlock()

	// Clear the session so the next turn starts with a fresh context (using the summary in instructions)
	a.ClearSession(sessionID)
	slog.Info("Context compressed successfully", "sessionID", sessionID)
}

// ClearSession removes a chat session (useful for /reset command)
func (a *Agent) ClearSession(sessionID string) {
	userID := "default-user"
	a.sessionService.Delete(context.Background(), &session.DeleteRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
}

// Chat handles conversational messages with session persistence
func (a *Agent) Chat(ctx context.Context, sessionID, message string) (string, error) {
	slog.Info("Agent.Chat called", "sessionID", sessionID, "messageLength", len(message))

	userID := "default-user"

	events := a.adkRunner.Run(ctx, userID, sessionID, &genai.Content{
		Parts: []*genai.Part{{Text: message}},
	}, agent.RunConfig{})

	var lastText strings.Builder
	for event, err := range events {
		if err != nil {
			return "", fmt.Errorf("ADK runner error: %w", err)
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					lastText.WriteString(part.Text)
				}
			}
		}
	}

	response := strings.TrimSpace(lastText.String())
	if response == "" {
		return "", fmt.Errorf("no response from ADK agent")
	}

	return response, nil
}

// RunMission executes a one-shot research mission (no session persistence)
func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	missionID := fmt.Sprintf("mission-%d", time.Now().UnixNano())

	missionAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot-mission",
		Model:       a.adkLLM,
		Description: "RavenBot research mission agent",
		Instruction: a.cfg.Bot.ResearchSystemPrompt,
		Tools:       a.GetRavenTools(),
	})
	if err != nil {
		return "", err
	}

	// Create a temporary runner for the mission
	missionRunner, err := runner.New(runner.Config{
		AppName:        AppName,
		Agent:          missionAgent,
		SessionService: a.sessionService,
	})
	if err != nil {
		return "", err
	}

	events := missionRunner.Run(ctx, "mission-user", missionID, &genai.Content{
		Parts: []*genai.Part{{Text: prompt}},
	}, agent.RunConfig{})

	var lastText strings.Builder
	for event, err := range events {
		if err != nil {
			return "", err
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					lastText.WriteString(part.Text)
				}
			}
		}
	}

	return strings.TrimSpace(lastText.String()), nil
}
