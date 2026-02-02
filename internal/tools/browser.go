package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowseWeb navigates to a URL and extracts the textual content using chromedp.
func BrowseWeb(ctx context.Context, url string) (string, error) {
	if err := ValidateURL(url); err != nil {
		return "", fmt.Errorf("security validation failed for browser URL: %w", err)
	}

	// Create a timeout for the browser action
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Use an allocator with no-sandbox (common for Docker/Alpine)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
	)

	// Use CHROME_BIN environment variable if set (for Docker/Alpine)
	if chromeBin := os.Getenv("CHROME_BIN"); chromeBin != "" {
		opts = append(opts, chromedp.ExecPath(chromeBin))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Create chromedp context
	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(format string, args ...interface{}) {
		slog.Debug(fmt.Sprintf(format, args...), "component", "chromedp")
	}))
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
