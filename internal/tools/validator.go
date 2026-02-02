package tools

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

// ValidateURL checks if a URL is safe to fetch (prevents SSRF).
// It ensures the scheme is http or https and that the host does not resolve to a private or local IP.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid scheme: %s (only http and https are allowed)", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	// Resolve the host to IP addresses to prevent DNS rebinding and handle numeric IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		// If we can't resolve it, it might be an invalid host or a problem with DNS
		return fmt.Errorf("failed to resolve host '%s': %w", host, err)
	}

	for _, ip := range ips {
		if isLocalOrPrivateIP(ip) {
			return fmt.Errorf("access to private or local IP address is not allowed: %s", ip.String())
		}
	}

	return nil
}

// isLocalOrPrivateIP returns true if the IP address is a loopback, link-local, or private address.
func isLocalOrPrivateIP(ip net.IP) bool {
	// Allow local URLs for testing if environment variable is set
	if os.Getenv("ALLOW_LOCAL_URLS") == "true" {
		return false
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate()
}
