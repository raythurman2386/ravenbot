package agent

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/raythurman2386/ravenbot/internal/backend"
	"github.com/raythurman2386/ravenbot/internal/config"
	raven "github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/mcp"
	"github.com/raythurman2386/ravenbot/internal/stats"
	"github.com/raythurman2386/ravenbot/internal/tools"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	adkdb "google.golang.org/adk/session/database"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

const AppName = "ravenbot"

type Agent struct {
	cfg            *config.Config
	db             *raven.DB
	stats          *stats.Stats
	mcpClients     map[string]*mcp.Client
	browserManager *tools.BrowserManager
	mu             sync.RWMutex

	// ADK components
	flashLLM model.LLM
	proLLM   model.LLM

	flashRunner *runner.Runner
	proRunner   *runner.Runner

	sessionService session.Service

	// Tool storage for missions and sub-agents
	technicalTools   []tool.Tool
	researchMCPTools []tool.Tool
}

func NewAgent(ctx context.Context, cfg *config.Config, database *raven.DB, botStats *stats.Stats, dialector gorm.Dialector) (*Agent, error) {
	slog.Info("Initializing agent", "backend", cfg.AIBackend)

	a := &Agent{
		cfg:            cfg,
		db:             database,
		stats:          botStats,
		mcpClients:     make(map[string]*mcp.Client),
		browserManager: tools.NewBrowserManager(ctx),
	}

	// 1. Initialize ADK Models (Flash & Pro) via configured backend
	var err error
	a.flashLLM, err = backend.NewFlashModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Flash model: %w", err)
	}

	a.proLLM, err = backend.NewProModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pro model: %w", err)
	}

	// 3. Initialize MCP Servers
	var wg sync.WaitGroup
	for name, serverCfg := range cfg.MCPServers {
		wg.Add(1)
		go func(name string, serverCfg config.MCPServerConfig) {
			defer wg.Done()
			slog.Info("Initializing MCP Server", "name", name, "command", serverCfg.Command)
			var mcpClient *mcp.Client
			if strings.HasPrefix(serverCfg.Command, "http://") || strings.HasPrefix(serverCfg.Command, "https://") {
				mcpClient = mcp.NewSSEClient(serverCfg.Command)
			} else {
				mcpClient = mcp.NewStdioClient(serverCfg.Command, serverCfg.Args, serverCfg.Env)
			}

			if err := mcpClient.Start(); err != nil {
				slog.Error("Failed to start MCP server", "name", name, "error", err)
				return
			}

			if err := mcpClient.Initialize(); err != nil {
				slog.Error("Failed to initialize MCP server", "name", name, "error", err)
				if err := mcpClient.Close(); err != nil {
					slog.Error("Failed to close MCP client", "name", name, "error", err)
				}
				return
			}

			a.mu.Lock()
			a.mcpClients[name] = mcpClient
			a.mu.Unlock()
		}(name, serverCfg)
	}
	wg.Wait()

	// 4. Initialize Session Service (SQLite Persistent via GORM Dialector)
	sessionService, err := adkdb.NewSessionService(dialector)
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK session service: %w", err)
	}

	// Auto-migrate the session schema
	if err := adkdb.AutoMigrate(sessionService); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate session schema: %w", err)
	}

	a.sessionService = sessionService

	// 5. Gather Tools and Create Sub-Agents
	a.technicalTools = a.GetTechnicalTools()
	mcpTools := a.GetMCPTools(ctx)
	coreTools := a.GetCoreTools()

	var rootMCPTools []tool.Tool
	var systemManagerMCPTools []tool.Tool
	var githubMCPTools []tool.Tool

	for _, t := range mcpTools {
		name := t.Name()
		// Memory tools are shared between the root agent and research assistant
		if strings.HasPrefix(name, "memory_") {
			rootMCPTools = append(rootMCPTools, t)
			a.researchMCPTools = append(a.researchMCPTools, t)
		} else if strings.HasPrefix(name, "sysmetrics") {
			systemManagerMCPTools = append(systemManagerMCPTools, t)
		} else if strings.HasPrefix(name, "github") {
			githubMCPTools = append(githubMCPTools, t)
		} else {
			a.researchMCPTools = append(a.researchMCPTools, t)
		}
	}

	// Create Research Assistant Sub-Agent (Uses Flash Model for speed and efficiency)
	researchAssistant, err := llmagent.New(llmagent.Config{
		Name:        "ResearchAssistant",
		Model:       a.flashLLM,
		Description: "A specialized assistant for technical research.",
		Instruction: cfg.Bot.ResearchSystemPrompt,
		Tools:       append(a.technicalTools, a.researchMCPTools...),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ResearchAssistant: %w", err)
	}

	// Create System Manager Sub-Agent (Uses Flash Model)
	systemManagerAgent, err := llmagent.New(llmagent.Config{
		Name:        "SystemManager",
		Model:       a.flashLLM,
		Description: "A specialized assistant for system diagnostics and health checks.",
		Instruction: cfg.Bot.SystemManagerPrompt,
		Tools:       systemManagerMCPTools,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SystemManager: %w", err)
	}

	// Wrap JulesTask as a Tool (External Service) - Available to the Jules Sub-Agent
	type JulesTaskArgs struct {
		Repo string `json:"repo" jsonschema:"The repository in 'owner/repo' format."`
		Task string `json:"task" jsonschema:"The coding task description."`
	}
	julesTaskTool, err := functiontool.New(functiontool.Config{
		Name:        "JulesTask",
		Description: "Delegates a coding task to the external Jules service.",
	}, func(ctx tool.Context, args JulesTaskArgs) (string, error) {
		return tools.DelegateToJules(ctx, cfg.JulesAPIKey, args.Repo, args.Task)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create JulesTask tool: %w", err)
	}

	// Create Jules Sub-Agent (Uses Pro Model for coding)
	julesAgent, err := llmagent.New(llmagent.Config{
		Name:        "Jules",
		Model:       a.proLLM,
		Description: "A specialized AI software engineer for coding tasks and GitHub operations.",
		Instruction: cfg.Bot.JulesPrompt,
		Tools:       append(githubMCPTools, julesTaskTool),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Jules agent: %w", err)
	}

	// Final toolset for the Root Agent
	allRootTools := append(coreTools, rootMCPTools...)

	// Instruction provider logic
	instructionProvider := func(ctx agent.ReadonlyContext) (string, error) {
		var summary string
		var err error
		if a.db != nil {
			summary, err = a.db.GetSessionSummary(ctx, ctx.SessionID())
			if err != nil {
				slog.Error("Failed to fetch session summary from DB", "sessionID", ctx.SessionID(), "error", err)
			}
		}

		if summary != "" {
			return fmt.Sprintf("%s\n\n### CONTEXT SUMMARY OF PREVIOUS CONVERSATION:\n%s", a.cfg.Bot.SystemPrompt, summary), nil
		}
		return a.cfg.Bot.SystemPrompt, nil
	}

	// Callback factory for context compression
	makeCompressionCallback := func(limit int64) llmagent.AfterModelCallback {
		return func(ctx agent.CallbackContext, llmResponse *model.LLMResponse, llmResponseError error) (*model.LLMResponse, error) {
			if llmResponseError != nil || llmResponse == nil || llmResponse.UsageMetadata == nil {
				return llmResponse, llmResponseError
			}

			// Check if context needs compression
			totalTokens := llmResponse.UsageMetadata.TotalTokenCount
			threshold := float64(limit) * a.cfg.Bot.TokenThreshold

			if float64(totalTokens) >= threshold {
				slog.Info("Context window threshold reached, triggering compression", "sessionID", ctx.SessionID(), "tokens", totalTokens, "limit", limit)
				go a.compressContext(ctx.SessionID())
			}

			return llmResponse, nil
		}
	}

	// 6. Create Root ADK LLMAgents (One for Flash, one for Pro)
	flashAgent, err := llmagent.New(llmagent.Config{
		Name:                "ravenbot-flash",
		Model:               a.flashLLM,
		Description:         "RavenBot Flash Agent",
		InstructionProvider: instructionProvider,
		Tools:               allRootTools,
		SubAgents:           []agent.Agent{researchAssistant, systemManagerAgent, julesAgent},
		AfterModelCallbacks: []llmagent.AfterModelCallback{makeCompressionCallback(a.cfg.Bot.FlashTokenLimit)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create flash agent: %w", err)
	}

	proAgent, err := llmagent.New(llmagent.Config{
		Name:                "ravenbot-pro",
		Model:               a.proLLM,
		Description:         "RavenBot Pro Agent",
		InstructionProvider: instructionProvider,
		Tools:               allRootTools,
		SubAgents:           []agent.Agent{researchAssistant, systemManagerAgent, julesAgent},
		AfterModelCallbacks: []llmagent.AfterModelCallback{makeCompressionCallback(a.cfg.Bot.ProTokenLimit)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pro agent: %w", err)
	}

	// 7. Create ADK Runners
	flashRunner, err := runner.New(runner.Config{
		AppName:        AppName,
		Agent:          flashAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Flash runner: %w", err)
	}
	a.flashRunner = flashRunner

	proRunner, err := runner.New(runner.Config{
		AppName:        AppName,
		Agent:          proAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Pro runner: %w", err)
	}
	a.proRunner = proRunner

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

	if a.db != nil {
		if err := a.db.SaveSessionSummary(ctx, sessionID, summary); err != nil {
			slog.Error("Failed to persist session summary", "sessionID", sessionID, "error", err)
		}
	}

	// Clear only the ADK session events/state to free up the window.
	// We do NOT call a.ClearSession here because that would also delete the summary we just saved.
	if err := a.sessionService.Delete(ctx, &session.DeleteRequest{
		AppName:   AppName,
		UserID:    "default-user",
		SessionID: sessionID,
	}); err != nil {
		slog.Error("Failed to clear session during compression", "sessionID", sessionID, "error", err)
	}

	slog.Info("Context compressed successfully", "sessionID", sessionID)
}

// Close cleans up the agent's resources.
func (a *Agent) Close() {
	if a.browserManager != nil {
		a.browserManager.Close()
	}
}

// ClearSession removes a chat session (useful for /reset command)
func (a *Agent) ClearSession(sessionID string) {
	userID := "default-user"
	ctx := context.Background()

	// Clear persisted summary when session is cleared
	if a.db != nil {
		if err := a.db.DeleteSessionSummary(ctx, sessionID); err != nil {
			slog.Warn("Failed to delete session summary during clear", "sessionID", sessionID, "error", err)
		}
	}

	if err := a.sessionService.Delete(ctx, &session.DeleteRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		slog.Error("Failed to delete session", "sessionID", sessionID, "error", err)
	}
}

// classifyPrompt determines if the prompt is 'Simple' or 'Complex' using the Flash model.
// Heavily biased towards "Simple" to maximize the usage of the highly capable Flash model.
func (a *Agent) classifyPrompt(ctx context.Context, message string) string {
	prompt := fmt.Sprintf(a.cfg.Bot.RoutingPrompt, message)

	respIter := a.flashLLM.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{{
			Role:  "user",
			Parts: []*genai.Part{{Text: prompt}},
		}},
	}, false)

	var result strings.Builder
	for resp, err := range respIter {
		if err != nil {
			slog.Warn("Classification failed, defaulting to Flash", "error", err)
			return "Simple"
		}
		if resp.Content != nil && len(resp.Content.Parts) > 0 {
			result.WriteString(resp.Content.Parts[0].Text)
		}
	}

	finalResult := strings.TrimSpace(result.String())
	if strings.EqualFold(finalResult, "Complex") {
		return "Complex"
	}
	return "Simple"
}

// Chat handles conversational messages with session persistence and dynamic model routing.
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

	// 1. Router Logic: Classify the prompt
	classification := a.classifyPrompt(ctx, message)
	var activeRunner *runner.Runner
	var modelName string

	if classification == "Simple" {
		activeRunner = a.flashRunner
		modelName = a.cfg.GeminiFlashModel
	} else {
		activeRunner = a.proRunner
		modelName = a.cfg.GeminiProModel
	}

	slog.Info("Routed request", "classification", classification, "model", modelName)

	// 2. Run with selected model
	events := activeRunner.Run(ctx, userID, sessionID, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: message}},
	}, agent.RunConfig{})

	return a.consumeRunnerEvents(sessionID, events)
}

// RunMission executes a one-shot research mission (no session persistence).
// Missions now default to the Flash model for speed and efficiency.
func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	missionID := fmt.Sprintf("mission-%d", time.Now().UnixNano())
	userID := "mission-user"

	// Create session for the mission
	_, err := a.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: missionID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create mission session: %w", err)
	}

	// Ensure cleanup after mission
	defer func() {
		if err := a.sessionService.Delete(context.Background(), &session.DeleteRequest{
			AppName:   AppName,
			UserID:    userID,
			SessionID: missionID,
		}); err != nil {
			slog.Warn("Failed to cleanup mission session", "sessionID", missionID, "error", err)
		}
	}()

	missionAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot-mission",
		Model:       a.flashLLM, // Use Flash model for missions
		Description: "RavenBot research mission agent",
		Instruction: a.cfg.Bot.ResearchSystemPrompt,
		Tools:       append(a.technicalTools, a.researchMCPTools...),
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
			// Handle common ADK/Gemini turn order issues.
			if strings.Contains(msg, "Error 400") && strings.Contains(msg, "function call turn") {
				slog.Warn("Turn order corruption detected, performing emergency session reset", "sessionID", sessionID)
				a.ClearSession(sessionID)
				return "", fmt.Errorf("encountered a technical glitch in conversation turn order; session reset, please try again")
			}
			return "", fmt.Errorf("ADK runner error: %w", err)
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					lastText.WriteString(part.Text)
				}
			}
		}
		// Track token usage from every event
		if a.stats != nil && event.UsageMetadata != nil {
			a.stats.RecordTokens(
				int64(event.UsageMetadata.PromptTokenCount),
				int64(event.UsageMetadata.CandidatesTokenCount),
			)
		}
	}

	response := strings.TrimSpace(lastText.String())
	if response == "" {
		return "", fmt.Errorf("no response from ADK agent")
	}

	return response, nil
}
