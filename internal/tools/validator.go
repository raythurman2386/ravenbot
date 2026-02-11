package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	// blockedPorts defines a list of common sensitive internal ports that should not be accessed by web tools.
	blockedPorts = map[string]bool{
		"21":    true, // FTP
		"22":    true, // SSH
		"23":    true, // Telnet
		"25":    true, // SMTP
		"53":    true, // DNS
		"110":   true, // POP3
		"143":   true, // IMAP
		"445":   true, // SMB
		"161":   true, // SNMP
		"2049":  true, // NFS
		"2379":  true, // etcd client
		"2380":  true, // etcd peer
		"3306":  true, // MySQL
		"4243":  true, // Docker API (alternative)
		"5000":  true, // Docker Registry
		"5432":  true, // PostgreSQL
		"5985":  true, // WinRM (HTTP)
		"5986":  true, // WinRM (HTTPS)
		"6379":  true, // Redis
		"7001":  true, // WebLogic
		"9000":  true, // PHP-FPM / FastCGI
		"9200":  true, // Elasticsearch (HTTP)
		"9300":  true, // Elasticsearch (Nodes)
		"2375":  true, // Docker API (unencrypted)
		"2376":  true, // Docker API (TLS)
		"2381":  true, // etcd metrics
		"6380":  true, // Redis (SSL)
		"6443":  true, // Kubernetes API
		"8001":  true, // Kubernetes API proxy
		"9090":  true, // Prometheus
		"9100":  true, // Node Exporter
		"10250": true, // Kubelet API
		"10255": true, // Kubelet (Read-only)
		"11211": true, // Memcached
		"16379": true, // Redis Cluster
		"26379": true, // Redis Sentinel
		"27017": true, // MongoDB
	}

	// additionalRestrictedRanges defines non-routable or documentation IP ranges not covered by Go's built-in checks.
	additionalRestrictedRanges []*net.IPNet
)

func init() {
	cidrs := []string{
		"100.64.0.0/10",   // CGNAT (RFC 6598)
		"192.0.0.0/24",    // IETF Protocol Assignments (RFC 6890)
		"192.0.2.0/24",    // Documentation TEST-NET-1 (RFC 5737)
		"192.88.99.0/24",  // 6to4 Relay Anycast (RFC 3068)
		"198.18.0.0/15",   // Benchmarking (RFC 2544)
		"198.51.100.0/24", // Documentation TEST-NET-2 (RFC 5737)
		"203.0.113.0/24",  // Documentation TEST-NET-3 (RFC 5737)
		"240.0.0.0/4",     // Reserved (RFC 1112)
		"100::/64",        // Discard-Only (RFC 6666)
		"2001:2::/48",     // IPv6 Benchmarking (RFC 5180)
		"2001:10::/28",    // ORCHID (RFC 4843)
		"2001:20::/28",    // ORCHIDv2 (RFC 7343)
		"2001:db8::/32",   // Documentation (RFC 3849)
	}
	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			additionalRestrictedRanges = append(additionalRestrictedRanges, block)
		}
	}
}

// isRestrictedIP returns true if the IP belongs to a restricted (private, loopback, etc.) range.
func isRestrictedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, block := range additionalRestrictedRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL checks if a URL is safe for fetching by blocking restricted IP ranges and ports.
func ValidateURL(ctx context.Context, urlStr string) error {
	if os.Getenv("ALLOW_LOCAL_URLS") == "true" {
		return nil
	}
	u, err := url.Parse(urlStr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid URL or scheme")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host in URL")
	}

	// Port-based SSRF protection
	if port := u.Port(); port != "" {
		if blockedPorts[port] {
			return fmt.Errorf("restricted port: %s", port)
		}
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve failed: %w", err)
	}
	for _, ip := range ips {
		if isRestrictedIP(ip) {
			return fmt.Errorf("restricted IP: %s", ip)
		}
	}
	return nil
}

// safeCheckRedirect is a redirect policy that validates the next URL.
func safeCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("too many redirects")
	}
	return ValidateURL(req.Context(), req.URL.String())
}

// NewSafeClient returns an http.Client with SSRF protection (redirects and DNS rebinding)
// and a configurable timeout.
func NewSafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: safeCheckRedirect,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, _ := net.SplitHostPort(addr)

				allowLocal := os.Getenv("ALLOW_LOCAL_URLS") == "true"

				// Defense-in-depth: check port against blacklist even in DialContext
				if blockedPorts[port] && !allowLocal {
					return nil, fmt.Errorf("restricted port: %s", port)
				}

				ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
				if err != nil {
					return nil, fmt.Errorf("DNS lookup failed for %s: %w", host, err)
				}

				// Try all non-restricted IPs, falling through on dial failure
				// so DNS round-robin provides resilience.
				var lastErr error
				for _, ip := range ips {
					if isRestrictedIP(ip) && !allowLocal {
						continue
					}
					conn, dialErr := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
					if dialErr != nil {
						lastErr = dialErr
						continue
					}
					return conn, nil
				}

				if lastErr != nil {
					return nil, fmt.Errorf("all resolved IPs failed for %s: %w", host, lastErr)
				}
				return nil, fmt.Errorf("connection to restricted IP blocked")
			},
		},
	}
}
