package agent

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/raythurman2386/ravenbot/internal/backend"
	"github.com/raythurman2386/ravenbot/internal/config"
	raven "github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/stats"
	"github.com/raythurman2386/ravenbot/internal/tools"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	adkdb "google.golang.org/adk/session/database"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

const AppName = "ravenbot"

type Agent struct {
	cfg   *config.Config
	db    *raven.DB
	stats *stats.Stats

	// ADK components
	flashLLM model.LLM
	proLLM   model.LLM

	flashRunner *runner.Runner
	proRunner   *runner.Runner

	sessionService session.Service

	// Sub-agents
	researchAssistant agent.Agent
	systemManager     agent.Agent
	julesAgent        agent.Agent
}

func NewAgent(ctx context.Context, cfg *config.Config, database *raven.DB, botStats *stats.Stats, dialector gorm.Dialector) (*Agent, error) {
	slog.Info("Initializing production agent", "backend", cfg.AIBackend)

	a := &Agent{
		cfg:   cfg,
		db:    database,
		stats: botStats,
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

	// 2. Initialize Session Service (SQLite Persistent via GORM Dialector)
	sessionService, err := adkdb.NewSessionService(dialector)
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK session service: %w", err)
	}

	if err := adkdb.AutoMigrate(sessionService); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate session schema: %w", err)
	}
	a.sessionService = sessionService

	// 3. Initialize MCP Servers — keyed by server name for targeted assignment
	mcpToolsetsByName := make(map[string]tool.Toolset)
	var mcpWG sync.WaitGroup
	var mcpMu sync.Mutex

	for name, serverCfg := range cfg.MCPServers {
		mcpWG.Add(1)
		go func(name string, serverCfg config.MCPServerConfig) {
			defer mcpWG.Done()

			slog.Info("Initializing official MCP Toolset", "name", name)
			var transport officialmcp.Transport
			if strings.HasPrefix(serverCfg.Command, "http://") || strings.HasPrefix(serverCfg.Command, "https://") {
				transport = &officialmcp.SSEClientTransport{Endpoint: serverCfg.Command}
			} else {
				cmd := exec.Command(serverCfg.Command, serverCfg.Args...)
				if len(serverCfg.Env) > 0 {
					cmd.Env = os.Environ()
					for k, v := range serverCfg.Env {
						cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, os.ExpandEnv(v)))
					}
				}
				transport = &officialmcp.CommandTransport{Command: cmd}
			}

			ts, err := mcptoolset.New(mcptoolset.Config{
				Transport: transport,
			})
			if err != nil {
				slog.Error("Failed to create MCP toolset", "name", name, "error", err)
				return
			}

			mcpMu.Lock()
			mcpToolsetsByName[name] = ts
			mcpMu.Unlock()
		}(name, serverCfg)
	}
	mcpWG.Wait()

	// Build targeted MCP toolset slices per sub-agent.
	// ResearchAssistant: weather, memory, filesystem, sequential-thinking
	// SystemManager:     sysmetrics
	// Jules:             github
	researchMCPNames := []string{"weather", "memory", "filesystem", "sequential-thinking"}
	systemMCPNames := []string{"sysmetrics"}
	julesMCPNames := []string{"github"}

	collectToolsets := func(names []string) []tool.Toolset {
		var ts []tool.Toolset
		for _, n := range names {
			if t, ok := mcpToolsetsByName[n]; ok {
				ts = append(ts, t)
			} else {
				slog.Warn("MCP toolset not available for agent assignment", "name", n)
			}
		}
		return ts
	}

	researchToolsets := collectToolsets(researchMCPNames)
	systemToolsets := collectToolsets(systemMCPNames)
	julesToolsets := collectToolsets(julesMCPNames)

	// 5. Create Sub-Agents

	// Create System Manager Sub-Agent
	systemManagerAgent, err := llmagent.New(llmagent.Config{
		Name:        "SystemManager",
		Model:       a.flashLLM,
		Description: "A specialized assistant for system diagnostics and health checks.",
		Instruction: cfg.Bot.SystemManagerPrompt,
		Toolsets:    systemToolsets,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SystemManager: %w", err)
	}
	a.systemManager = systemManagerAgent

	// JulesTask Tool
	type JulesTaskArgs struct {
		Repo string `json:"repo" jsonschema:"The repository in 'owner/repo' format."`
		Task string `json:"task" jsonschema:"The coding task description."`
	}
	julesTaskTool, err := functiontool.New(functiontool.Config{
		Name:        "JulesTask",
		Description: "Delegates a coding task to the external Jules service. REQUIRED for any code modification, refactoring, or repository creation.",
	}, func(ctx tool.Context, args JulesTaskArgs) (string, error) {
		return tools.DelegateToJules(ctx, cfg.JulesAPIKey, args.Repo, args.Task)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create JulesTask tool: %w", err)
	}

	// Create Jules Sub-Agent
	julesAgent, err := llmagent.New(llmagent.Config{
		Name:        "Jules",
		Model:       a.proLLM,
		Description: "A specialized AI software engineer for coding tasks and GitHub operations.",
		Instruction: cfg.Bot.JulesPrompt,
		Tools:       []tool.Tool{julesTaskTool},
		Toolsets:    julesToolsets,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Jules agent: %w", err)
	}
	a.julesAgent = julesAgent

	// Create Research Assistant Sub-Agent
	// Use a custom web_search function tool that wraps a standalone Gemini API
	// call with GoogleSearch grounding. This avoids the Gemini API restriction
	// that prevents mixing grounding tools with function-calling tools (which
	// the ADK injects via transfer_to_agent and MCP toolsets).
	type WebSearchArgs struct {
		Query string `json:"query" jsonschema:"The search query to look up on the web."`
	}
	webSearchTool, err := functiontool.New(functiontool.Config{
		Name:        "web_search",
		Description: "Search the web using Google Search to find current, up-to-date information. Use this for any question requiring recent data, news, documentation, or facts you are unsure about.",
	}, func(ctx tool.Context, args WebSearchArgs) (string, error) {
		return tools.WebSearch(ctx, cfg.GeminiAPIKey, cfg.GeminiFlashModel, args.Query)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create web_search tool: %w", err)
	}

	researchTools := []tool.Tool{webSearchTool}
	researchAssistant, err := llmagent.New(llmagent.Config{
		Name:        "ResearchAssistant",
		Model:       a.flashLLM,
		Description: "A specialized assistant for technical research and web searches.",
		Instruction: cfg.Bot.ResearchSystemPrompt + "\n\nUse the web_search tool for all web searches to find up-to-date information.",
		Tools:       researchTools,
		Toolsets:    researchToolsets,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ResearchAssistant: %w", err)
	}
	a.researchAssistant = researchAssistant

	// 6. Instruction provider logic
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

	// 7. Create Root ADK LLMAgents
	allSubAgents := []agent.Agent{researchAssistant, systemManagerAgent, julesAgent}

	flashAgent, err := llmagent.New(llmagent.Config{
		Name:                "ravenbot-flash",
		Model:               a.flashLLM,
		Description:         "RavenBot Flash Agent",
		InstructionProvider: instructionProvider,
		Tools:               nil,
		SubAgents:           allSubAgents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create flash agent: %w", err)
	}

	proAgent, err := llmagent.New(llmagent.Config{
		Name:                "ravenbot-pro",
		Model:               a.proLLM,
		Description:         "RavenBot Pro Agent",
		InstructionProvider: instructionProvider,
		Tools:               nil,
		SubAgents:           allSubAgents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pro agent: %w", err)
	}

	// 8. Create ADK Runners
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

func (a *Agent) Close() {
	// No-op: retained for interface compatibility.
	// Browser and MCP cleanup happens via context cancellation.
}

func (a *Agent) ClearSession(sessionID string) {
	userID := sessionID
	ctx := context.Background()
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

func (a *Agent) Chat(ctx context.Context, sessionID, message string) (string, error) {
	slog.Info("Agent.Chat called", "sessionID", sessionID, "messageLength", len(message))
	userID := sessionID

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

	classification := a.classifyPrompt(ctx, message)
	var activeRunner *runner.Runner
	if classification == "Simple" {
		activeRunner = a.flashRunner
	} else {
		activeRunner = a.proRunner
	}

	slog.Info("Routed request", "classification", classification)

	events := activeRunner.Run(ctx, userID, sessionID, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: message}},
	}, agent.RunConfig{})

	return a.consumeRunnerEvents(sessionID, events)
}

func (a *Agent) RunMission(ctx context.Context, prompt string) (string, error) {
	missionID := fmt.Sprintf("mission-%d", time.Now().UnixNano())
	userID := "mission-user"

	_, err := a.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   AppName,
		UserID:    userID,
		SessionID: missionID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create mission session: %w", err)
	}

	defer func() {
		cleanupCtx := context.Background()
		if a.db != nil {
			if err := a.db.DeleteSessionSummary(cleanupCtx, missionID); err != nil {
				slog.Warn("Failed to cleanup mission summary", "sessionID", missionID, "error", err)
			}
		}
		if err := a.sessionService.Delete(cleanupCtx, &session.DeleteRequest{
			AppName:   AppName,
			UserID:    userID,
			SessionID: missionID,
		}); err != nil {
			slog.Warn("Failed to cleanup mission session", "sessionID", missionID, "error", err)
		}
	}()

	missionAgent, err := llmagent.New(llmagent.Config{
		Name:        "ravenbot-mission",
		Model:       a.flashLLM,
		Description: "RavenBot research mission agent",
		Instruction: a.cfg.Bot.ResearchSystemPrompt,
		SubAgents:   []agent.Agent{a.researchAssistant},
	})
	if err != nil {
		return "", err
	}

	missionRunner, err := runner.New(runner.Config{
		AppName:        AppName,
		Agent:          missionAgent,
		SessionService: a.sessionService,
	})
	if err != nil {
		return "", err
	}

	events := missionRunner.Run(ctx, userID, missionID, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: prompt}},
	}, agent.RunConfig{})

	return a.consumeRunnerEvents(missionID, events)
}

func (a *Agent) consumeRunnerEvents(sessionID string, events iter.Seq2[*session.Event, error]) (string, error) {
	var lastText string
	for event, err := range events {
		if err != nil {
			slog.Error("ADK runner yielded error", "error", err)
			return "", fmt.Errorf("ADK runner error: %w", err)
		}

		// Diagnostic: log every event for debugging
		hasContent := event.Content != nil && len(event.Content.Parts) > 0
		var textPreview string
		if hasContent {
			for _, p := range event.Content.Parts {
				if p.Text != "" {
					textPreview = p.Text
					if len(textPreview) > 80 {
						textPreview = textPreview[:80] + "..."
					}
					break
				}
			}
		}
		slog.Debug("ADK event",
			"sessionID", sessionID,
			"author", event.Author,
			"isFinal", event.IsFinalResponse(),
			"hasContent", hasContent,
			"textPreview", textPreview,
		)

		// Track token usage from every event
		if a.stats != nil && event.UsageMetadata != nil {
			a.stats.RecordTokens(
				int64(event.UsageMetadata.PromptTokenCount),
				int64(event.UsageMetadata.CandidatesTokenCount),
			)
		}

		// Only collect text from final response events to avoid
		// mixing intermediate sub-agent/tool output into the reply.
		if !event.IsFinalResponse() {
			if event.Content != nil {
				for _, part := range event.Content.Parts {
					if part.FunctionCall != nil {
						slog.Info("Model called tool", "name", part.FunctionCall.Name, "args", part.FunctionCall.Args)
					}
				}
			}
			continue
		}

		if event.Content != nil {
			var sb strings.Builder
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					sb.WriteString(part.Text)
				}
			}
			if text := sb.String(); text != "" {
				lastText = text
			}
		}
	}

	response := strings.TrimSpace(lastText)
	if response == "" {
		return "", fmt.Errorf("no response from ADK agent")
	}

	return response, nil
}
