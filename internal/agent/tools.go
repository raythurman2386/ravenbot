package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/raythurman2386/ravenbot/internal/tools"

	"google.golang.org/genai"
)

// Tool definitions for Gemini 3 Flash
var RavenTools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "FetchRSS",
				Description: "Fetches information from an RSS feed URL. Returns a list of titles, links, and descriptions.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {
							Type:        genai.TypeString,
							Description: "The URL of the RSS feed.",
						},
					},
					Required: []string{"url"},
				},
			},
			{
				Name:        "ScrapePage",
				Description: "Scrapes the main text content from a webpage URL.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {
							Type:        genai.TypeString,
							Description: "The URL of the webpage to scrape.",
						},
					},
					Required: []string{"url"},
				},
			},
			{
				Name:        "ShellExecute",
				Description: "Executes a restricted set of shell commands (df, free, uptime, whoami, date).",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"command": {
							Type:        genai.TypeString,
							Description: "The command to run.",
						},
						"args": {
							Type:        genai.TypeArray,
							Items:       &genai.Schema{Type: genai.TypeString},
							Description: "The arguments for the command.",
						},
					},
					Required: []string{"command"},
				},
			},
			{
				Name:        "BrowseWeb",
				Description: "Navigates to a URL using a headless browser to extract content from JS-heavy sites.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {
							Type:        genai.TypeString,
							Description: "The URL of the webpage to browse.",
						},
					},
					Required: []string{"url"},
				},
			},
			{
				Name:        "SearchWeb",
				Description: "Performs a web search to find the latest information and URLs.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query": {
							Type:        genai.TypeString,
							Description: "The search query.",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "JulesTask",
				Description: "Delegates a complex coding or repository task to the Gemini Jules Agent.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"repo": {
							Type:        genai.TypeString,
							Description: "The GitHub repository (e.g., owner/repo).",
						},
						"task": {
							Type:        genai.TypeString,
							Description: "The description of the coding task to perform.",
						},
					},
					Required: []string{"repo", "task"},
				},
			},
		},
	},
}

// Map function names to actual implementations
func (a *Agent) handleToolCall(ctx context.Context, call *genai.FunctionCall) (any, error) {
	// 1. Try Native Tools First
	switch call.Name {
	case "FetchRSS":
		url := call.Args["url"].(string)
		items, err := tools.FetchRSS(ctx, url)
		if err != nil {
			return nil, err
		}

		// Deduplication logic
		var newItems []tools.RSSItem
		for _, item := range items {
			exists, err := a.db.HasHeadline(ctx, item.Link)
			if err != nil {
				return nil, err
			}
			if !exists {
				if err := a.db.AddHeadline(ctx, item.Title, item.Link); err != nil {
					return nil, err
				}
				newItems = append(newItems, item)
			}
		}
		return newItems, nil
	case "ScrapePage":
		url := call.Args["url"].(string)
		return tools.ScrapePage(ctx, url)
	case "ShellExecute":
		command := call.Args["command"].(string)
		var args []string
		if rawArgs, ok := call.Args["args"].([]any); ok {
			for _, arg := range rawArgs {
				if s, ok := arg.(string); ok {
					args = append(args, s)
				}
			}
		}
		return tools.ShellExecute(ctx, command, args)
	case "BrowseWeb":
		url := call.Args["url"].(string)
		return tools.BrowseWeb(ctx, url)
	case "JulesTask":
		repo := call.Args["repo"].(string)
		task := call.Args["task"].(string)
		return tools.DelegateToJules(ctx, a.cfg.JulesAPIKey, repo, task)
	case "SearchWeb":
		query := call.Args["query"].(string)
		return tools.SearchWeb(ctx, query)
	case "ReadMCPResource":
		serverName := call.Args["server"].(string)
		uri := call.Args["uri"].(string)
		if client, ok := a.mcpClients[serverName]; ok {
			contents, err := client.ReadResource(uri)
			if err != nil {
				return nil, fmt.Errorf("failed to read MCP resource: %w", err)
			}
			return contents, nil
		}
		return nil, fmt.Errorf("unknown MCP server: %s", serverName)
	}

	// 2. Try MCP Tools
	// Name format: serverName_toolName
	parts := strings.SplitN(call.Name, "_", 2)
	if len(parts) == 2 {
		serverName := parts[0]
		toolName := parts[1]

		if client, ok := a.mcpClients[serverName]; ok {
			// Convert genai args map[string]any to just map[string]any (same type usually)
			result, err := client.CallTool(toolName, call.Args)
			if err != nil {
				return nil, fmt.Errorf("MCP tool call failed: %w", err)
			}
			if result.IsError {
				// Format error for Gemini to self-correct
				return map[string]any{
					"error":   true,
					"content": result.Content,
				}, nil
			}
			// Return just the content for now
			return result.Content, nil
		}
	}

	return nil, fmt.Errorf("unknown tool: %s", call.Name)
}
