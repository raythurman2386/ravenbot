package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type SearchResult struct {
	Title   string
	Link    string
	Snippet string
}

// SearchWeb performs a web search using DuckDuckGo (HTML version) and returns the top results.
func SearchWeb(ctx context.Context, query string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	doc.Find(".web-result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= 10 {
			return
		}

		title := s.Find(".result__a").Text()
		link, _ := s.Find(".result__a").Attr("href")
		snippet := s.Find(".result__snippet").Text()

		if title != "" && link != "" {
			// Resolve relative links if any (though DDG usually provides absolute ones in this version)
			if strings.HasPrefix(link, "//") {
				link = "https:" + link
			}

			results = append(results, SearchResult{
				Title:   strings.TrimSpace(title),
				Link:    link,
				Snippet: strings.TrimSpace(snippet),
			})
		}
	})

	return results, nil
}
