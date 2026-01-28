package tools

import (
	"context"
	"fmt"

	"github.com/mmcdole/gofeed"
)

type RSSItem struct {
	Title       string
	Link        string
	Description string
}

func FetchRSS(ctx context.Context, url string) ([]RSSItem, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		// If it's a 404 or other HTTP error, we want the agent to know so it can try another URL.
		return nil, fmt.Errorf("RSS source at %s returned an error: %w", url, err)
	}

	var items []RSSItem
	for _, item := range feed.Items {
		items = append(items, RSSItem{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
		})
	}

	return items, nil
}
