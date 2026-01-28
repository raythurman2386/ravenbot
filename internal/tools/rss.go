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
		return nil, fmt.Errorf("failed to parse RSS feed from %s: %w", url, err)
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
