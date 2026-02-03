package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
)

// ValidateURL checks if a URL is safe for fetching by blocking restricted IP ranges.
func ValidateURL(ctx context.Context, urlStr string) error {
	if os.Getenv("ALLOW_LOCAL_URLS") == "true" {
		return nil
	}
	u, err := url.Parse(urlStr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid URL or scheme")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", u.Hostname())
	if err != nil {
		return fmt.Errorf("resolve failed: %w", err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("restricted IP: %s", ip)
		}
	}
	return nil
}

// SafeCheckRedirect is a redirect policy that validates the next URL.
func SafeCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("too many redirects")
	}
	return ValidateURL(req.Context(), req.URL.String())
}

// NewSafeClient returns an http.Client with SSRF protection (redirects and DNS rebinding).
func NewSafeClient() *http.Client {
	return &http.Client{
		CheckRedirect: SafeCheckRedirect,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, _ := net.SplitHostPort(addr)
				ips, _ := net.DefaultResolver.LookupIP(ctx, "ip", host)
				for _, ip := range ips {
					if !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified() || os.Getenv("ALLOW_LOCAL_URLS") == "true" {
						return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
					}
				}
				return nil, fmt.Errorf("connection to restricted IP blocked")
			},
		},
	}
}
