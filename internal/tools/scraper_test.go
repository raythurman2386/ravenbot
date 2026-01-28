package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrapePage(t *testing.T) {
	htmlContent := `
	<html>
		<head><title>Test Page</title></head>
		<body>
			<header>Header content</header>
			<nav>Nav content</nav>
			<main>
				<h1>Main Title</h1>
				<p>This is the main content.</p>
			</main>
			<footer>Footer content</footer>
			<script>alert('bad');</script>
			<style>.css { color: red; }</style>
		</body>
	</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	ctx := context.Background()
	text, err := ScrapePage(ctx, server.URL)

	assert.NoError(t, err)
	assert.Contains(t, text, "Main Title")
	assert.Contains(t, text, "This is the main content.")

	// Should NOT contain header, nav, footer, script, or style content
	assert.NotContains(t, text, "Header content")
	assert.NotContains(t, text, "Nav content")
	assert.NotContains(t, text, "Footer content")
	assert.NotContains(t, text, "alert('bad')")
	assert.NotContains(t, text, ".css")
}
