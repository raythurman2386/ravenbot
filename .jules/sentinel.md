## 2025-01-31 - SSRF Vulnerability in Web Tools
**Vulnerability:** The web-fetching tools (`ScrapePage`, `FetchRSS`, `BrowseWeb`) were vulnerable to Server-Side Request Forgery (SSRF) as they accepted any URL and performed requests without validation.
**Learning:** Basic URL validation at the entry point is insufficient because of redirects and DNS rebinding. Standard Go `http.Client` follows redirects by default, and `net.LookupIP` can be bypassed by changing DNS records between validation and connection.
**Prevention:** Implement a robust URL and IP validation mechanism. For Go's `http.Client`, use a `CheckRedirect` policy and a custom `DialContext` to validate IPs at the moment of connection. For third-party libraries, ensure they use a secured `http.Client`. Use `TestMain` or environment overrides in tests to allow `httptest.NewServer` (which uses loopback) to function without compromising production security.

## 2025-02-04 - Shell Tool Hardening and Container Escape Prevention
**Vulnerability:** The `ShellExecute` tool allowed dangerous commands like `env` (leaking secrets) and `docker` (allowing host takeover via `/var/run/docker.sock`). Sanitization was also missing several dangerous characters.
**Learning:** Whitelisting commands is only half the battle; redundant tools (like `cat` or `ls` when a filesystem MCP exists) should be removed to reduce attack surface. Mapping the docker socket to an agent-controlled container is a critical risk that must be avoided.
**Prevention:** Use a minimal command whitelist, block all shell-sensitive characters in arguments (including backticks, quotes, and newlines), and never expose the host docker socket to the agent container.
