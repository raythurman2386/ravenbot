package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSItem struct {
	Title       string
	Link        string
	Description string
	Published   string
}

func FetchRSS(ctx context.Context, url string) ([]RSSItem, error) {
	if err := ValidateURL(ctx, url); err != nil {
		return nil, err
	}

	fp := gofeed.NewParser()
	fp.Client = NewSafeClient(30 * time.Second)
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("RSS source at %s returned an error: %w", url, err)
	}

	var items []RSSItem
	for _, item := range feed.Items {
		pubDate := ""
		if item.Published != "" {
			pubDate = item.Published
		} else if item.Updated != "" {
			pubDate = item.Updated
		}

		items = append(items, RSSItem{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Published:   pubDate,
		})
	}

	return items, nil
}
