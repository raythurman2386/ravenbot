package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/mcp"
	"github.com/raythurman2386/ravenbot/internal/tools"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/geminitool"
)

// GetRavenTools returns the list of ADK tools for the agent.
func (a *Agent) GetRavenTools() []tool.Tool {
	var ravenTools []tool.Tool

	// FetchRSS Tool
	type FetchRSSArgs struct {
		URL string `json:"url" jsonschema:"The URL of the RSS feed."`
	}
	fetchRSSTool, err := functiontool.New(functiontool.Config{
		Name:        "FetchRSS",
		Description: "Fetches information from an RSS feed URL. Returns a list of titles, links, and descriptions.",
	}, func(ctx tool.Context, args FetchRSSArgs) ([]tools.RSSItem, error) {
		items, err := tools.FetchRSS(ctx, args.URL)
		if err != nil {
			return nil, err
		}
		return a.deduplicateRSSItems(ctx, items)
	})
	if err != nil {
		slog.Error("Failed to create FetchRSS tool", "error", err)
	} else {
		ravenTools = append(ravenTools, fetchRSSTool)
	}

	// ScrapePage Tool
	type ScrapePageArgs struct {
		URL string `json:"url" jsonschema:"The URL of the webpage to scrape."`
	}
	scrapePageTool, err := functiontool.New(functiontool.Config{
		Name:        "ScrapePage",
		Description: "Scrapes the main text content from a webpage URL.",
	}, func(ctx tool.Context, args ScrapePageArgs) (string, error) {
		return tools.ScrapePage(ctx, args.URL)
	})
	if err != nil {
		slog.Error("Failed to create ScrapePage tool", "error", err)
	} else {
		ravenTools = append(ravenTools, scrapePageTool)
	}

	// ShellExecute Tool
	type ShellExecuteArgs struct {
		Command string   `json:"command" jsonschema:"The command to run (df, free, uptime, whoami, date)."`
		Args    []string `json:"args,omitempty" jsonschema:"The arguments for the command."`
	}
	shellExecuteTool, err := functiontool.New(functiontool.Config{
		Name:        "ShellExecute",
		Description: "Executes a restricted set of shell commands.",
	}, func(ctx tool.Context, args ShellExecuteArgs) (string, error) {
		return tools.ShellExecute(ctx, args.Command, args.Args)
	})
	if err != nil {
		slog.Error("Failed to create ShellExecute tool", "error", err)
	} else {
		ravenTools = append(ravenTools, shellExecuteTool)
	}

	// BrowseWeb Tool
	type BrowseWebArgs struct {
		URL string `json:"url" jsonschema:"The URL of the webpage to browse using a headless browser."`
	}
	browseWebTool, err := functiontool.New(functiontool.Config{
		Name:        "BrowseWeb",
		Description: "Navigates to a URL using a headless browser to extract content from JS-heavy sites.",
	}, func(ctx tool.Context, args BrowseWebArgs) (string, error) {
		return tools.BrowseWeb(ctx, args.URL)
	})
	if err != nil {
		slog.Error("Failed to create BrowseWeb tool", "error", err)
	} else {
		ravenTools = append(ravenTools, browseWebTool)
	}

	// JulesTask Tool
	type JulesTaskArgs struct {
		Repo string `json:"repo" jsonschema:"The GitHub repository (e.g., owner/repo)."`
		Task string `json:"task" jsonschema:"The description of the coding task to perform."`
	}
	julesTaskTool, err := functiontool.New(functiontool.Config{
		Name:        "JulesTask",
		Description: "Delegates a complex coding or repository task to the Gemini Jules Agent.",
	}, func(ctx tool.Context, args JulesTaskArgs) (string, error) {
		return tools.DelegateToJules(ctx, a.cfg.JulesAPIKey, args.Repo, args.Task)
	})
	if err != nil {
		slog.Error("Failed to create JulesTask tool", "error", err)
	} else {
		ravenTools = append(ravenTools, julesTaskTool)
	}

	// Native Google Search Tool
	ravenTools = append(ravenTools, geminitool.GoogleSearch{})

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
	if err != nil {
		slog.Error("Failed to create ReadMCPResource tool", "error", err)
	} else {
		ravenTools = append(ravenTools, readResourceTool)
	}

	return ravenTools
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

func (a *Agent) deduplicateRSSItems(ctx context.Context, items []tools.RSSItem) ([]tools.RSSItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	urls := make([]string, len(items))
	for i, item := range items {
		urls[i] = item.Link
	}

	existing, err := a.db.GetExistingHeadlines(ctx, urls)
	if err != nil {
		return nil, err
	}

	var newItems []tools.RSSItem
	var headlinesToInsert []db.Headline

	for _, item := range items {
		if !existing[item.Link] {
			newItems = append(newItems, item)
			headlinesToInsert = append(headlinesToInsert, db.Headline{
				Title: item.Title,
				URL:   item.Link,
			})
			// Prevent duplicates within the same batch
			existing[item.Link] = true
		}
	}

	if err := a.db.AddHeadlines(ctx, headlinesToInsert); err != nil {
		return nil, err
	}

	return newItems, nil
}
