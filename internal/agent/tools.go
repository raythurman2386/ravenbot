package agent

import (
	"context"
	"ravenbot/internal/tools"

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
		},
	},
}

// Map function names to actual implementations
func HandleToolCall(ctx context.Context, call *genai.FunctionCall) (any, error) {
	switch call.Name {
	case "FetchRSS":
		url := call.Args["url"].(string)
		return tools.FetchRSS(ctx, url)
	case "ScrapePage":
		url := call.Args["url"].(string)
		return tools.ScrapePage(ctx, url)
	default:
		return nil, nil
	}
}
