package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserManager manages a shared Chrome allocator context.
type BrowserManager struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// NewBrowserManager creates a new BrowserManager with a shared allocator.
func NewBrowserManager(ctx context.Context) *BrowserManager {
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

	return &BrowserManager{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
	}
}

// Close cleans up the shared allocator.
func (bm *BrowserManager) Close() {
	if bm.allocCancel != nil {
		bm.allocCancel()
	}
}

// Browse navigates to a URL and extracts the textual content using the shared allocator.
func (bm *BrowserManager) Browse(ctx context.Context, url string) (string, error) {
	if err := ValidateURL(ctx, url); err != nil {
		return "", err
	}

	// Create chromedp context from the shared allocator context
	browserCtx, browserCancel := chromedp.NewContext(bm.allocCtx, chromedp.WithLogf(func(format string, args ...interface{}) {
		slog.Debug(fmt.Sprintf(format, args...), "component", "chromedp")
	}))
	defer browserCancel()

	// Create a timeout for the browser action derived from the browser context
	// This ensures the action is cancelled if it takes too long.
	// Note: We cannot easily inherit from the passed 'ctx' because chromedp requires the context
	// chain to flow from the allocator. We rely on the 30s timeout as the primary safeguard.
	timeoutCtx, cancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer cancel()

	var content string
	err := chromedp.Run(timeoutCtx,
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

// BrowseWeb navigates to a URL and extracts the textual content using chromedp.
// Deprecated: Use BrowserManager.Browse instead for better performance.
func BrowseWeb(ctx context.Context, url string) (string, error) {
	// Create a temporary manager for backward compatibility
	bm := NewBrowserManager(ctx)
	defer bm.Close()
	return bm.Browse(ctx, url)
}
