package tools

import (
	"strings"
	"testing"
)

func TestExtractDDGURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		href     string
		expected string
	}{
		{
			name:     "Regular URL",
			href:     "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "DDG Redirect URL",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath%3Fq%3D1",
			expected: "https://example.com/path?q=1",
		},
		{
			name:     "DDG Redirect URL with multiple params",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&other=1",
			expected: "https://example.com",
		},
		{
			name:     "Invalid URL in uddg",
			href:     "//duckduckgo.com/l/?uddg=%GH",
			expected: "//duckduckgo.com/l/?uddg=%GH", // falls back to href if uddg cannot be parsed
		},
		{
			name:     "No uddg param",
			href:     "//duckduckgo.com/l/?nothing=here",
			expected: "//duckduckgo.com/l/?nothing=here",
		},
		{
			name:     "Malformed URL",
			href:     "://malformed",
			expected: "://malformed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractDDGURL(tt.href)
			if got != tt.expected {
				t.Errorf("extractDDGURL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatSearchResults(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		results  []SearchResult
		contains []string
	}{
		{
			name:    "Empty results",
			results: []SearchResult{},
			contains: []string{
				"No results found.",
			},
		},
		{
			name: "Single result",
			results: []SearchResult{
				{
					Title:   "Example Title",
					URL:     "https://example.com",
					Snippet: "This is a snippet.",
				},
			},
			contains: []string{
				"**1. Example Title**",
				"URL: https://example.com",
				"This is a snippet.",
			},
		},
		{
			name: "Multiple results",
			results: []SearchResult{
				{
					Title:   "First",
					URL:     "https://first.com",
					Snippet: "Snippet 1",
				},
				{
					Title:   "Second",
					URL:     "https://second.com",
					Snippet: "",
				},
			},
			contains: []string{
				"**1. First**",
				"URL: https://first.com",
				"Snippet 1",
				"**2. Second**",
				"URL: https://second.com",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatSearchResults(tt.results)
			for _, substr := range tt.contains {
				if !strings.Contains(got, substr) {
					t.Errorf("FormatSearchResults() does not contain %q\nGot:\n%s", substr, got)
				}
			}
		})
	}
}
