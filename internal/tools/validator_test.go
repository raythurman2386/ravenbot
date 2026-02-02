package tools

import (
	"context"
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_URLS", "false")
	tests := []struct {
		url     string
		wantErr bool
		errSub  string
	}{
		{"https://google.com", false, ""},
		{"http://example.com", false, ""},
		{"http://127.0.0.1", true, "private or local IP"},
		{"http://localhost", true, "private or local IP"},
		{"http://10.0.0.1", true, "private or local IP"},
		{"http://192.168.1.1", true, "private or local IP"},
		{"http://172.16.0.1", true, "private or local IP"},
		{"ftp://example.com", true, "invalid scheme"},
		{"file:///etc/passwd", true, "invalid scheme"},
		{"http://[::1]", true, "private or local IP"},
	}

	for _, tt := range tests {
		err := ValidateURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			continue
		}
		if tt.wantErr && !strings.Contains(err.Error(), tt.errSub) {
			t.Errorf("ValidateURL(%q) error = %v, want error containing %q", tt.url, err, tt.errSub)
		}
	}
}

func TestScrapePageSSRF(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_URLS", "false")
	ctx := context.Background()

	// Test localhost access
	_, err := ScrapePage(ctx, "http://127.0.0.1:12345")
	if err == nil {
		t.Fatal("Expected error for localhost access, got nil")
	}

	if !strings.Contains(err.Error(), "private or local IP") {
		t.Errorf("Error should indicate it was blocked by security validation, but got: %v", err)
	}
}

func TestFetchRSSSSRF(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_URLS", "false")
	ctx := context.Background()

	// Test private IP access
	_, err := FetchRSS(ctx, "http://10.0.0.1/feed.xml")
	if err == nil {
		t.Fatal("Expected error for private IP access, got nil")
	}

	if !strings.Contains(err.Error(), "private or local IP") {
		t.Errorf("Error should indicate it was blocked by security validation, but got: %v", err)
	}
}
