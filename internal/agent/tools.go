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
)

// GetTechnicalTools returns the list of tools intended for the ResearchAssistant sub-agent.
func (a *Agent) GetTechnicalTools() []tool.Tool {
	var technicalTools []tool.Tool

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
	if err == nil {
		technicalTools = append(technicalTools, fetchRSSTool)
	}

	// ScrapePage Tool
	type ScrapePageArgs struct {
		URL string `json:"url" jsonschema:"The URL of the webpage to scrape."`
	}
	scrapePageTool, err := functiontool.New(functiontool.Config{
		Name:        "ScrapePage",
		Description: "Extracts textual content from a static webpage URL. Use this for standard HTML pages.",
	}, func(ctx tool.Context, args ScrapePageArgs) (string, error) {
		return tools.ScrapePage(ctx, args.URL)
	})
	if err == nil {
		technicalTools = append(technicalTools, scrapePageTool)
	}

	// BrowseWeb Tool
	type BrowseWebArgs struct {
		URL string `json:"url" jsonschema:"The URL of the webpage to browse."`
	}
	browseWebTool, err := functiontool.New(functiontool.Config{
		Name:        "BrowseWeb",
		Description: "Renders a webpage using a headless browser. Use this for JavaScript-heavy or single-page applications.",
	}, func(ctx tool.Context, args BrowseWebArgs) (string, error) {
		return a.browserManager.Browse(ctx, args.URL)
	})
	if err == nil {
		technicalTools = append(technicalTools, browseWebTool)
	}

	// WebSearch Tool
	type WebSearchArgs struct {
		Query      string `json:"query" jsonschema:"The search query."`
		MaxResults int    `json:"max_results,omitempty" jsonschema:"Max results (default 5)."`
	}
	webSearchTool, err := functiontool.New(functiontool.Config{
		Name:        "WebSearch",
		Description: "Searches the web for real-time information and documentation.",
	}, func(ctx tool.Context, args WebSearchArgs) ([]tools.SearchResult, error) {
		return tools.DuckDuckGoSearch(ctx, args.Query, args.MaxResults)
	})
	if err == nil {
		technicalTools = append(technicalTools, webSearchTool)
	}

	return technicalTools
}

// GetCoreTools returns the tools for the root conversational agent.
func (a *Agent) GetCoreTools() []tool.Tool {
	var coreTools []tool.Tool

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
