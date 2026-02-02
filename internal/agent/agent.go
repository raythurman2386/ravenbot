package agent

import (
	"context"
	"fmt"
	"iter"
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
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
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

	// 3. Initialize MCP Servers
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

	// 4. Initialize Session Service
	sessionService := session.InMemoryService()

	// 5. Gather Tools and Create Sub-Agents
	technicalTools := a.GetTechnicalTools()
	coreTools := a.GetCoreTools()

	// Categorize MCP Tools
	mcpTools := a.GetMCPTools(ctx)
	var rootMCPTools []tool.Tool
	var researchMCPTools []tool.Tool

	for _, t := range mcpTools {
		// Memory tools stay in the root agent for personalization
		if strings.HasPrefix(t.Name(), "memory_") {
			rootMCPTools = append(rootMCPTools, t)
		} else {
			researchMCPTools = append(researchMCPTools, t)
		}
	}

	// Create Research Assistant Sub-Agent
	researchAssistant, err := llmagent.New(llmagent.Config{
		Name:        "ResearchAssistant",
		Model:       adkLLM,
		Description: "A specialized assistant for technical research, web search, system diagnostics, GitHub, and repository management.",
		Instruction: cfg.Bot.ResearchSystemPrompt,
		Tools:       append(technicalTools, researchMCPTools...),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ResearchAssistant: %w", err)
	}

	// Wrap Research Assistant as a Tool with a custom implementation to fix streaming output loss in ADK's agenttool
	type ResearchAssistantArgs struct {
		Request string `json:"request" jsonschema:"The technical research or diagnostic request."`
	}
	researchTool, err := functiontool.New(functiontool.Config{
		Name:        "ResearchAssistant",
		Description: "A specialized assistant for technical research, web search, system diagnostics, and shell commands.",
	}, func(ctx tool.Context, args ResearchAssistantArgs) (map[string]any, error) {
		// 1. Create sub-session for the research assistant
		stateMap := make(map[string]any)
		for k, v := range ctx.State().All() {
			if !strings.HasPrefix(k, "_adk") {
				stateMap[k] = v
			}
		}

		subSession, err := sessionService.Create(ctx, &session.CreateRequest{
			AppName: "ResearchAssistant",
			UserID:  ctx.UserID(),
			State:   stateMap,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create sub-session: %w", err)
		}

		// 2. Create a one-off runner for this call
		r, err := runner.New(runner.Config{
			AppName:        "ResearchAssistant",
			Agent:          researchAssistant,
			SessionService: sessionService,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create sub-runner: %w", err)
		}

		// 3. Execute and accumulate text output
		var fullOutput strings.Builder
		events := r.Run(ctx, ctx.UserID(), subSession.Session.ID(), &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: args.Request}},
		}, agent.RunConfig{
			StreamingMode: agent.StreamingModeSSE,
		})

		for event, err := range events {
			if err != nil {
				return nil, err
			}
			if event.LLMResponse.Content != nil {
				for _, part := range event.LLMResponse.Content.Parts {
					if part.Text != "" {
						fullOutput.WriteString(part.Text)
					}
				}
			}
		}

		result := fullOutput.String()
		if result == "" {
			return nil, fmt.Errorf("research assistant returned no output")
		}

		return map[string]any{"result": result}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create research tool: %w", err)
	}

	// Final toolset for the Root Agent
	allRootTools := append(coreTools, rootMCPTools...)
	allRootTools = append(allRootTools, researchTool)

	// 5. Create Root ADK LLMAgent
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot",
		Model:       adkLLM,
		Description: "RavenBot autonomous conversation agent",
		InstructionProvider: func(ctx agent.ReadonlyContext) (string, error) {
			a.summariesMu.RLock()
			summary := a.summaries[ctx.SessionID()]
			a.summariesMu.RUnlock()

			if summary != "" {
				return fmt.Sprintf("%s\n\n### CONTEXT SUMMARY OF PREVIOUS CONVERSATION:\n%s", a.cfg.Bot.SystemPrompt, summary), nil
			}
			return a.cfg.Bot.SystemPrompt, nil
		},
		Tools: allRootTools,
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
		return nil, fmt.Errorf("failed to create root ADK agent: %w", err)
	}
	a.adkAgent = adkAgent

	// 7. Create ADK Runner
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
	if err := a.sessionService.Delete(context.Background(), &session.DeleteRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		slog.Error("Failed to delete session", "sessionID", sessionID, "error", err)
	}
}

// Chat handles conversational messages with session persistence
func (a *Agent) Chat(ctx context.Context, sessionID, message string) (string, error) {
	slog.Info("Agent.Chat called", "sessionID", sessionID, "messageLength", len(message))

	userID := "default-user"

	// Ensure session exists
	_, err := a.sessionService.Get(ctx, &session.GetRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		slog.Info("Session not found, creating new one", "sessionID", sessionID)
		_, err = a.sessionService.Create(ctx, &session.CreateRequest{
			AppName:   AppName,
			UserID:    userID,
			SessionID: sessionID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create session: %w", err)
		}
	}

	events := a.adkRunner.Run(ctx, userID, sessionID, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: message}},
	}, agent.RunConfig{})

	return a.consumeRunnerEvents(sessionID, events)
}

// RunMission executes a one-shot research mission (no session persistence)
func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	missionID := fmt.Sprintf("mission-%d", time.Now().UnixNano())

	missionAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot-mission",
		Model:       a.adkLLM,
		Description: "RavenBot research mission agent",
		Instruction: a.cfg.Bot.ResearchSystemPrompt,
		Tools:       a.GetTechnicalTools(),
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
		Role:  "user",
		Parts: []*genai.Part{{Text: prompt}},
	}, agent.RunConfig{})

	return a.consumeRunnerEvents(missionID, events)
}

// consumeRunnerEvents processes the event stream from the ADK runner
func (a *Agent) consumeRunnerEvents(sessionID string, events iter.Seq2[*session.Event, error]) (string, error) {
	var lastText strings.Builder
	for event, err := range events {
		if err != nil {
			slog.Error("ADK runner yielded error", "error", err)
			msg := err.Error()
			// Improved error handling for common ADK/Gemini turn order issues.
			if strings.Contains(msg, "Error 400") && strings.Contains(msg, "function call turn") {
				slog.Warn("Turn order corruption detected, performing emergency session reset", "sessionID", sessionID)
				a.ClearSession(sessionID)
				// We return a user-friendly error suggesting a retry
				return "", fmt.Errorf("encountered a technical glitch in conversation turn order; session reset, please try again")
			}
			return "", fmt.Errorf("ADK runner error: %w", err)
		}
		if event.LLMResponse.Content != nil {
			for _, part := range event.LLMResponse.Content.Parts {
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
