package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchRSS(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_URLS", "true")

	// Mock RSS feed content
	rssContent := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
 <title>Test Feed</title>
 <item>
  <title>Test Item 1</title>
  <link>http://example.com/item1</link>
  <description>Description 1</description>
 </item>
 <item>
  <title>Test Item 2</title>
  <link>http://example.com/item2</link>
  <description>Description 2</description>
 </item>
</channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rssContent))
	}))
	defer server.Close()

	ctx := context.Background()
	items, err := FetchRSS(ctx, server.URL)

	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Test Item 1", items[0].Title)
	assert.Equal(t, "http://example.com/item1", items[0].Link)
}
