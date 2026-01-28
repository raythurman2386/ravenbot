package tools

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowseWeb navigates to a URL and extracts the textual content using chromedp.
func BrowseWeb(ctx context.Context, url string) (string, error) {
	// Create a timeout for the browser action
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Use an allocator with no-sandbox (common for Docker/Alpine)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Create chromedp context
	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer browserCancel()

	var content string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(url),
		// Wait for body to be visible or a small timeout
		chromedp.Sleep(2*time.Second),
		chromedp.Text("body", &content, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("chromedp failed for %s: %w", url, err)
	}

	return content, nil
}
