package tools

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIsRestrictedIP_Coverage(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"Loopback v4", "127.0.0.1", true},
		{"Loopback v6", "::1", true},
		{"Private 10.x", "10.0.0.1", true},
		{"Private 172.16.x", "172.16.0.1", true},
		{"Private 192.168.x", "192.168.1.1", true},
		{"CGNAT", "100.64.0.1", true},
		{"TEST-NET-1", "192.0.2.1", true},
		{"Benchmarking", "198.18.0.1", true},
		{"Public IP", "8.8.8.8", false},
		{"Public IP 2", "142.250.80.46", false},
		{"Link-local", "169.254.1.1", true},
		{"Unspecified v4", "0.0.0.0", true},
		{"IPv6 documentation", "2001:db8::1", true},
		{"IPv6 benchmarking", "2001:2::1", true},
		{"IPv6 ORCHID", "2001:10::1", true},
		{"IPv6 public", "2607:f8b0:4004:800::200e", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %s", tt.ip)
			}
			got := isRestrictedIP(ip)
			if got != tt.want {
				t.Errorf("isRestrictedIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestNewSafeClient_ReachesServer(t *testing.T) {
	// ALLOW_LOCAL_URLS is set by TestMain, so local httptest servers work.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	client := NewSafeClient(5 * time.Second)
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("NewSafeClient GET failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", body)
	}
}

func TestNewSafeClient_BlockedPortInDialContext(t *testing.T) {
	// Temporarily disable ALLOW_LOCAL_URLS so blocked port logic fires
	_ = os.Setenv("ALLOW_LOCAL_URLS", "false")
	defer func() { _ = os.Setenv("ALLOW_LOCAL_URLS", "true") }()

	client := NewSafeClient(5 * time.Second)
	// Port 3306 (MySQL) should be blocked at the DialContext level
	_, err := client.Get("http://example.com:3306/test")
	if err == nil {
		t.Fatal("expected error for blocked port 3306, got nil")
	}
	if !strings.Contains(err.Error(), "restricted port") {
		t.Errorf("expected 'restricted port' error, got: %v", err)
	}
}

func TestNewSafeClient_FollowsRedirectSafely(t *testing.T) {
	// ALLOW_LOCAL_URLS is set by TestMain
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("final"))
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	client := NewSafeClient(5 * time.Second)
	resp, err := client.Get(redirectServer.URL)
	if err != nil {
		t.Fatalf("redirect request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "final" {
		t.Errorf("expected body 'final', got %q", body)
	}
}

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
		{"Blocked port 10250 (Kubelet)", "http://example.com:10250", false, true},
		{"Blocked port 16379 (Redis Cluster)", "http://example.com:16379", false, true},
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
