package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/raythurman2386/ravenbot/internal/mcp"
	"github.com/raythurman2386/ravenbot/internal/tools"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/geminitool"
)

// GetTechnicalTools returns the list of tools intended for the ResearchAssistant sub-agent.
func (a *Agent) GetTechnicalTools() []tool.Tool {
	var technicalTools []tool.Tool

	// GoogleSearch Tool
	technicalTools = append(technicalTools, &geminitool.GoogleSearch{})

	return technicalTools
}

// GetCoreTools returns the tools for the root conversational agent.
func (a *Agent) GetCoreTools() []tool.Tool {
	var coreTools []tool.Tool

	// JulesTask Tool
	type JulesTaskArgs struct {
		Repo string `json:"repo" jsonschema:"The GitHub repository (e.g., owner/repo)."`
		Task string `json:"task" jsonschema:"The description of the coding task to perform."`
	}
	julesTaskTool, err := functiontool.New(functiontool.Config{
		Name:        "JulesTask",
		Description: "Delegates complex coding and repository tasks to the Jules Agent.",
	}, func(ctx tool.Context, args JulesTaskArgs) (string, error) {
		return tools.DelegateToJules(ctx, a.cfg.JulesAPIKey, args.Repo, args.Task)
	})
	if err == nil {
		coreTools = append(coreTools, julesTaskTool)
	}

	// ReadMCPResource Tool
	type ReadMCPResourceArgs struct {
		Server string `json:"server" jsonschema:"The name of the MCP server."`
		URI    string `json:"uri" jsonschema:"The URI of the resource to read."`
	}
	readResourceTool, err := functiontool.New(functiontool.Config{
		Name:        "ReadMCPResource",
		Description: "Reads the content of an MCP resource from a specific server and URI.",
	}, func(ctx tool.Context, args ReadMCPResourceArgs) (any, error) {
		a.mu.RLock()
		client, ok := a.mcpClients[args.Server]
		a.mu.RUnlock()
		if ok {
			contents, err := client.ReadResource(args.URI)
			if err != nil {
				return nil, fmt.Errorf("failed to read MCP resource: %w", err)
			}
			return contents, nil
		}
		return nil, fmt.Errorf("unknown MCP server: %s", args.Server)
	})
	if err == nil {
		coreTools = append(coreTools, readResourceTool)
	}

	return coreTools
}

// GetMCPTools dynamically discovers and registers tools from configured MCP servers.
func (a *Agent) GetMCPTools(ctx context.Context) []tool.Tool {
	var mcpTools []tool.Tool

	a.mu.RLock()
	defer a.mu.RUnlock()

	for serverName, client := range a.mcpClients {
		tools, err := client.ListTools()
		if err != nil {
			slog.Error("Failed to list tools from MCP server", "name", serverName, "error", err)
			continue
		}

		for _, t := range tools {
			adkTool, err := a.createADKToolFromMCP(serverName, t)
			if err != nil {
				slog.Error("Failed to convert MCP tool to ADK tool", "server", serverName, "tool", t.Name, "error", err)
				continue
			}
			mcpTools = append(mcpTools, adkTool)
			slog.Info("Registered MCP Tool as ADK tool", "name", adkTool.Name(), "server", serverName)
		}
	}

	return mcpTools
}

func (a *Agent) createADKToolFromMCP(serverName string, mcpTool mcp.Tool) (tool.Tool, error) {
	namespacedName := fmt.Sprintf("%s_%s", serverName, mcpTool.Name)

	var schema jsonschema.Schema
	if err := schema.UnmarshalJSON(mcpTool.InputSchema); err != nil {
		return nil, fmt.Errorf("failed to parse MCP tool schema: %w", err)
	}

	handler := func(ctx tool.Context, args map[string]any) (any, error) {
		a.mu.RLock()
		client, ok := a.mcpClients[serverName]
		a.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("MCP server not found: %s", serverName)
		}

		result, err := client.CallTool(mcpTool.Name, args)
		if err != nil {
			return nil, fmt.Errorf("MCP tool call failed: %w", err)
		}

		if result.IsError {
			return map[string]any{
				"error":   true,
				"content": result.Content,
			}, nil
		}

		return result.Content, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        namespacedName,
		Description: fmt.Sprintf("[%s] %s", serverName, mcpTool.Description),
		InputSchema: &schema,
	}, handler)
}

