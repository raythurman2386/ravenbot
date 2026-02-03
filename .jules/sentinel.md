## 2025-01-31 - SSRF Vulnerability in Web Tools
**Vulnerability:** The web-fetching tools (`ScrapePage`, `FetchRSS`, `BrowseWeb`) were vulnerable to Server-Side Request Forgery (SSRF) as they accepted any URL and performed requests without validation.
**Learning:** Basic URL validation at the entry point is insufficient because of redirects and DNS rebinding. Standard Go `http.Client` follows redirects by default, and `net.LookupIP` can be bypassed by changing DNS records between validation and connection.
**Prevention:** Implement a robust URL and IP validation mechanism. For Go's `http.Client`, use a `CheckRedirect` policy and a custom `DialContext` to validate IPs at the moment of connection. For third-party libraries, ensure they use a secured `http.Client`.
