package tools

import (
	"context"
	"os"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		allowLocal bool
		wantErr    bool
	}{
		{"Valid public URL", "https://www.google.com", false, false},
		{"Loopback IP", "http://127.0.0.1", false, true},
		{"Loopback hostname", "http://localhost", false, true},
		{"Private IP range", "http://192.168.1.1", false, true},
		{"Invalid scheme", "ftp://google.com", false, true},
		{"Empty host", "http:///path", false, true},
		{"Blocked port 22", "http://example.com:22", false, true},
		{"Blocked port 3306", "http://example.com:3306", false, true},
		{"Allowed port 8080", "http://example.com:8080", false, false},
		{"IPv6 Loopback", "http://[::1]", false, true},
		{"Allow local URLs", "http://127.0.0.1", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.allowLocal {
				_ = os.Setenv("ALLOW_LOCAL_URLS", "true")
				defer func() { _ = os.Unsetenv("ALLOW_LOCAL_URLS") }()
			} else {
				_ = os.Unsetenv("ALLOW_LOCAL_URLS")
			}

			err := ValidateURL(context.Background(), tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
