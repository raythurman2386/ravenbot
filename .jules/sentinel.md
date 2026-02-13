## 2025-01-31 - SSRF Vulnerability in Web Tools
**Vulnerability:** The web-fetching tools (`ScrapePage`, `FetchRSS`, `BrowseWeb`) were vulnerable to Server-Side Request Forgery (SSRF) as they accepted any URL and performed requests without validation.
**Learning:** Basic URL validation at the entry point is insufficient because of redirects and DNS rebinding. Standard Go `http.Client` follows redirects by default, and `net.LookupIP` can be bypassed by changing DNS records between validation and connection.
**Prevention:** Implement a robust URL and IP validation mechanism. For Go's `http.Client`, use a `CheckRedirect` policy and a custom `DialContext` to validate IPs at the moment of connection. For third-party libraries, ensure they use a secured `http.Client`. Use `TestMain` or environment overrides in tests to allow `httptest.NewServer` (which uses loopback) to function without compromising production security.

## 2025-02-04 - Shell Tool Hardening and Container Escape Prevention
**Vulnerability:** The `ShellExecute` tool allowed dangerous commands like `env` (leaking secrets) and `docker` (allowing host takeover via `/var/run/docker.sock`). Sanitization was also missing several dangerous characters.
**Learning:** Whitelisting commands is only half the battle; redundant tools (like `cat` or `ls` when a filesystem MCP exists) should be removed to reduce attack surface. Mapping the docker socket to an agent-controlled container is a critical risk that must be avoided.
**Prevention:** Use a minimal command whitelist, block all shell-sensitive characters in arguments (including backticks, quotes, and newlines), and never expose the host docker socket to the agent container.

## 2025-02-10 - Consolidating SSRF Protection and Timeout Management
**Vulnerability:** Multiple tools (`WebSearch`, `MCP SSE Client`) were using default `http.Client` instances, bypassing the SSRF protections implemented in the centralized `NewSafeClient`. Additionally, `NewSafeClient` lacked a default timeout, leading to potential resource exhaustion.
**Learning:** Enforcing security at the factory level (`NewSafeClient`) is effective, but the factory must be flexible enough to handle different connection lifecycles. Applying a rigid timeout to all clients breaks streaming protocols like SSE.
**Prevention:** Centralize all HTTP client creation through a secured factory that accepts configuration (like timeouts). Ensure that all outbound network-accessing tools use this factory rather than `http.DefaultClient` or manual `&http.Client{}` instantiation.

## 2025-02-12 - Defensive Hardening: Input Limits and Port Filtering
**Vulnerability:** The bot lacked limits on user input length, creating a DoS risk. Additionally, SSRF protection focused on IP ranges but did not account for sensitive internal services on standard ports.
**Learning:** Defense-in-depth requires addressing both application-level resource exhaustion and network-level lateral movement. Even if an IP is "public," specific ports (like 3306 or 6379) should never be accessed by a web scraper tool.
**Prevention:** Enforce strict byte-length limits on all entry-point handlers. Enhance SSRF validation by implementing a port blacklist that blocks common internal infrastructure ports, even for otherwise valid hostnames.

## 2025-02-14 - Robust SSRF: CGNAT and DialContext Port Validation
**Vulnerability:** SSRF protection relied on Go's `IsPrivate()`, which omits the CGNAT range (`100.64.0.0/10`) and documentation IPs. Furthermore, the secure HTTP client only validated IPs at connection time, missing a final port-level check.
**Learning:** Standard library "private" checks are often incomplete for modern cloud and overlay network (e.g., Tailscale) environments. Security validation must be applied at both the high-level URL entry point and the low-level transport layer to be truly effective.
**Prevention:** Supplement `IsPrivate()` with a comprehensive list of non-routable CIDRs (CGNAT, Benchmarking, Documentation). Enforce the port blacklist within the `DialContext` of safe HTTP clients as a defense-in-depth measure against bypasses.

## 2025-02-18 - Expanding SSRF Protection and Chromedp Limitations
**Vulnerability:** SSRF protection missed several sensitive internal ports (SMB, etcd, etc.) and non-routable IPv6 ranges. Additionally, while `NewSafeClient` protects HTTP tools, `chromedp` (used in `BrowseWeb`) bypasses these protections for sub-resources and redirects after the initial `ValidateURL` check.
**Learning:** Blacklists must be regularly updated to account for common internal services. However, tool-specific network stacks (like Chrome's in `chromedp`) require specialized interception (e.g., `network.SetRequestPaused`) to be fully secured, as a simple entry-point URL check is insufficient against redirects or malicious sub-resources.
**Prevention:** Maintain a comprehensive and expanding list of blocked ports and non-routable CIDRs. For browser-based tools, investigate Chrome DevTools Protocol (CDP) level request interception to enforce SSRF policies consistently across the entire network stack.

## 2025-02-21 - Consolidating SSRF Protection Across All Outbound Clients
**Vulnerability:** Despite having a centralized `NewSafeClient`, several components (Ollama adapter, MCP SSE transport, and Gemini grounded search) were still using `http.DefaultClient` or default SDK configurations, leaving them vulnerable to SSRF.
**Learning:** Security primitives are only effective if they are universally applied. SDKs often hide the underlying HTTP client, making it easy to overlook.
**Prevention:** Explicitly configure all SDKs and adapters (GenAI, MCP, Ollama) to use the secured `NewSafeClient`. Use `TestMain` to enable `ALLOW_LOCAL_URLS` for tests that require `httptest.NewServer` without compromising production safety.

## 2025-02-25 - Hardened SSRF: Port Blacklist Expansion and GenAI Integration
**Vulnerability:** The SSRF port blacklist was missing several common internal service ports (MSSQL, Oracle, RabbitMQ, VNC). Additionally, the main Gemini GenAI client configuration in the backend was not yet integrated with the centralized `NewSafeClient`.
**Learning:** Security hardening is an iterative process. Even with a centralized "safe client," every new outbound connection (like a new LLM backend) must be explicitly opted into the protection. Blocking common development ports like 8080 and 8888 provides a necessary layer of defense against accidental exposure of internal administration or development tools.
**Prevention:** Maintain a comprehensive and regularly updated port blacklist. Ensure every new outbound HTTP client, including those used by LLM SDKs, is initialized via the centralized security factory (`NewSafeClient`).

## 2025-03-01 - SSRF Bypass via Port String Normalization
**Vulnerability:** SSRF protection using a string-based port blacklist could be bypassed by using leading zeros in the port (e.g., `:0022` instead of `:22`). The `isRestrictedIP` check was also missing some multicast ranges.
**Learning:** Canonicalization is critical for security checks. Numeric values like ports should be normalized to a standard representation before being compared against a blacklist. Relying on raw user-provided strings for security decisions is dangerous.
**Prevention:** Always normalize security-sensitive inputs. For ports, convert the string to an integer and back to a canonical string (or check as an integer). Broaden IP checks to include all multicast ranges via `IsMulticast()`.
