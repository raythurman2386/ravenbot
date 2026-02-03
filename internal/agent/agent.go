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
	flashLLM model.LLM
	proLLM   model.LLM

	flashRunner *runner.Runner
	proRunner   *runner.Runner

	sessionService session.Service
}

func NewAgent(ctx context.Context, cfg *config.Config, database *db.DB) (*Agent, error) {
	// 1. Initialize ADK Models (Flash & Pro)
	flashLLM, err := gemini.NewModel(ctx, cfg.Bot.FlashModel, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK Gemini Flash model: %w", err)
	}

	proLLM, err := gemini.NewModel(ctx, cfg.Bot.ProModel, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK Gemini Pro model: %w", err)
	}

	a := &Agent{
		cfg:        cfg,
		db:         database,
		mcpClients: make(map[string]*mcp.Client),
		summaries:  make(map[string]string),
		flashLLM:   flashLLM,
		proLLM:     proLLM,
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
	a.sessionService = sessionService

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

	// Create Research Assistant Sub-Agent (Uses Pro Model)
	researchAssistant, err := llmagent.New(llmagent.Config{
		Name:        "ResearchAssistant",
		Model:       proLLM,
		Description: "A specialized assistant for technical research, web search, system diagnostics, GitHub, and repository management.",
		Instruction: cfg.Bot.ResearchSystemPrompt,
		Tools:       append(technicalTools, researchMCPTools...),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ResearchAssistant: %w", err)
	}

	// Wrap Research Assistant as a Tool
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

	// Instruction provider logic
	instructionProvider := func(ctx agent.ReadonlyContext) (string, error) {
		a.summariesMu.RLock()
		summary := a.summaries[ctx.SessionID()]
		a.summariesMu.RUnlock()

		if summary != "" {
			return fmt.Sprintf("%s\n\n### CONTEXT SUMMARY OF PREVIOUS CONVERSATION:\n%s", a.cfg.Bot.SystemPrompt, summary), nil
		}
		return a.cfg.Bot.SystemPrompt, nil
	}

	// Callback for context compression
	afterModelCallback := func(ctx agent.CallbackContext, llmResponse *model.LLMResponse, llmResponseError error) (*model.LLMResponse, error) {
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
	}

	// 5. Create Root ADK LLMAgents (One for Flash, one for Pro)
	flashAgent, err := llmagent.New(llmagent.Config{
		Name:                "ravenbot-flash",
		Model:               flashLLM,
		Description:         "RavenBot Flash Agent",
		InstructionProvider: instructionProvider,
		Tools:               allRootTools,
		AfterModelCallbacks: []llmagent.AfterModelCallback{afterModelCallback},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create flash agent: %w", err)
	}

	proAgent, err := llmagent.New(llmagent.Config{
		Name:                "ravenbot-pro",
		Model:               proLLM,
		Description:         "RavenBot Pro Agent",
		InstructionProvider: instructionProvider,
		Tools:               allRootTools,
		AfterModelCallbacks: []llmagent.AfterModelCallback{afterModelCallback},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pro agent: %w", err)
	}

	// 6. Create ADK Runners
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

// classifyPrompt determines if the prompt is 'Simple' or 'Complex' using the Flash model.
func (a *Agent) classifyPrompt(ctx context.Context, message string) string {
	prompt := fmt.Sprintf(`You are a request classifier.
Analyze the following user input and determine if it is "Simple" (conversational, greetings, simple questions, short acknowledgments) or "Complex" (requires reasoning, coding, tool use, multi-step logic, technical questions).

User Input: "%s"

Respond with ONLY the word "Simple" or "Complex".`, message)

	respIter := a.flashLLM.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{{
			Role: "user",
			Parts: []*genai.Part{{Text: prompt}},
		}},
	}, false)

	var result string
	for resp, err := range respIter {
		if err != nil {
			slog.Warn("Classification failed, defaulting to Pro", "error", err)
			return "Complex"
		}
		if resp.Content != nil && len(resp.Content.Parts) > 0 {
			result += resp.Content.Parts[0].Text
		}
	}

	result = strings.TrimSpace(result)
	if strings.EqualFold(result, "Simple") {
		return "Simple"
	}
	return "Complex"
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
		modelName = a.cfg.Bot.FlashModel
	} else {
		activeRunner = a.proRunner
		modelName = a.cfg.Bot.ProModel
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
// Missions always default to the Pro model for deep research.
func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	missionID := fmt.Sprintf("mission-%d", time.Now().UnixNano())

	missionAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot-mission",
		Model:       a.proLLM, // Use Pro model for missions
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
