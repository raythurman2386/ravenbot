package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// SearchResult represents a single search result from DuckDuckGo
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// DuckDuckGoSearch performs a web search using DuckDuckGo's HTML interface.
// This is a free, no-API-key solution that scrapes the lite HTML version.
func DuckDuckGoSearch(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		maxResults = 10 // Limit to avoid excessive scraping
	}

	// Use the HTML version of DuckDuckGo (no JavaScript required)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult

	// Parse search results from DuckDuckGo's HTML structure
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}

		// Extract title and URL
		titleLink := s.Find(".result__a")
		title := strings.TrimSpace(titleLink.Text())
		href, exists := titleLink.Attr("href")
		if !exists || title == "" {
			return
		}

		// DuckDuckGo wraps URLs in a redirect, extract the actual URL
		actualURL := extractDDGURL(href)
		if actualURL == "" {
			actualURL = href
		}

		// Extract snippet
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		results = append(results, SearchResult{
			Title:   title,
			URL:     actualURL,
			Snippet: snippet,
		})
	})

	if len(results) == 0 {
		return nil, fmt.Errorf("no search results found for query: %s", query)
	}

	return results, nil
}

// extractDDGURL extracts the actual URL from DuckDuckGo's redirect wrapper
func extractDDGURL(href string) string {
	// DuckDuckGo wraps URLs like: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com...
	if strings.Contains(href, "uddg=") {
		parsed, err := url.Parse(href)
		if err != nil {
			return ""
		}
		uddg := parsed.Query().Get("uddg")
		if uddg != "" {
			decoded, err := url.QueryUnescape(uddg)
			if err != nil {
				return uddg
			}
			return decoded
		}
	}
	return href
}

// FormatSearchResults formats search results as a readable string
func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
