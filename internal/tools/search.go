package tools

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SearchResult represents a single search result from DuckDuckGo
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:122.0) Gecko/20100101 Firefox/122.0",
}

// getRandomUserAgent returns a random User-Agent string
func getRandomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
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

	client := NewSafeClient(30 * time.Second)
	var resp *http.Response
	var err error

	// Add a small initial jitter to avoid bursty behavior
	time.Sleep(time.Duration(rand.Intn(500)+100) * time.Millisecond)

	// Retry loop for handling 202 Accepted, 429 Too Many Requests, 403 Forbidden, or temporary failures
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Rotate User-Agent
		req.Header.Set("User-Agent", getRandomUserAgent())
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Referer", "https://duckduckgo.com/")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-User", "?1")

		if i > 0 {
			// Exponential backoff: 1s, 2s, 4s...
			backoff := time.Duration(1<<i) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Backoff before retry
			}
		}

		resp, err = client.Do(req)
		if err != nil {
			// If it's a network error, check if we should retry
			if i == maxRetries-1 {
				return nil, fmt.Errorf("failed to fetch search results after retries: %w", err)
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			// Success! Parse and return.
			defer resp.Body.Close()
			return parseDDGResults(resp.Body, maxResults, query)
		}

		// Handle specific retryable codes
		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			continue
		}

		// For other non-200 codes, stop and return error
		resp.Body.Close()
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	// If we exhausted retries without success or error return inside loop
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	if resp != nil {
		return nil, fmt.Errorf("search failed with status code: %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("search failed: unknown error")
}

func parseDDGResults(body io.Reader, maxResults int, query string) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(body)
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
