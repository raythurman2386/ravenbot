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
		{"Carrier-Grade NAT (CGNAT)", "http://100.64.0.1", false, true},
		{"Documentation Range (TEST-NET-1)", "http://192.0.2.1", false, true},
		{"Benchmarking Range", "http://198.18.0.1", false, true},
		{"IPv6 Documentation", "http://[2001:db8::1]", false, true},
		{"6to4 Relay Anycast", "http://192.88.99.1", false, true},
		{"IPv6 Benchmarking", "http://[2001:2::1]", false, true},
		{"ORCHID", "http://[2001:10::1]", false, true},
		{"Blocked port 445 (SMB)", "http://example.com:445", false, true},
		{"Blocked port 9200 (Elasticsearch)", "http://example.com:9200", false, true},
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
