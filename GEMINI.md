# ravenbot: AI Agent Context & Architecture

This document provides structural and behavioral context for AI agents working on the **ravenbot** repository.

## 🛠 Tech Stack
- **Language**: Go 1.25.6
- **AI Framework**: Google Agent Development Kit (ADK) `google.golang.org/adk`
- **Generative AI**: `google.golang.org/genai` (Gemini 3 Pro & Flash)
- **Database**: SQLite via `modernc.org/sqlite` (CGO-free)
- **Networking**: Custom safe HTTP client with SSRF protection in `internal/tools/validator.go`
- **Browsing**: Headless Chrome via `github.com/chromedp/chromedp`

## 📁 Project Structure
```text
.
├── cmd/bot/                # Application entry point (main loop, interactive mode)
├── internal/
│   ├── agent/             # Core AI logic (Agent struct, Routing, Sub-agents)
│   │   ├── agent.go       # ADK Agent initialization and model routing
│   │   ├── tools.go       # Tool registration and MCP-to-ADK conversion
│   │   └── report.go      # Markdown report generation logic
│   ├── mcp/               # Custom MCP client (Stdio & SSE transports)
│   ├── tools/             # Native tool implementations (Search, Browser, RSS, Shell)
│   ├── db/                # SQLite persistence (Headlines, Briefings)
│   ├── notifier/          # Telegram & Discord delivery systems
│   └── config/            # Environment and JSON configuration loading
├── config.json            # Bot settings, MCP servers, and scheduled jobs
└── Makefile               # Development workflow (build, test, lint)
```

## 🧠 Core Patterns

### 1. Flash-First Model Routing
RavenBot uses a two-stage routing pattern implemented in `internal/agent/agent.go`:
- **Classification**: Every user prompt is first sent to **Gemini 3 Flash** with a system prompt to classify it as "Simple" or "Complex".
- **Execution**:
    - "Simple" requests are handled by the **Flash Runner** for low-latency chat.
    - "Complex" requests (reasoning, tool use, technical tasks) are routed to the **Pro Runner**.

### 2. ResearchAssistant Sub-Agent
For deep-dive missions, RavenBot utilizes a specialized sub-agent:
- **Lifecycle**: Created as a `llmagent` within the main Agent. It is also wrapped as an ADK tool (`ResearchAssistant`) that the root agent can call.
- **Tools**: It has access to the full technical toolbelt (Search, Browse, Shell, MCP).
- **Isolation**: Each mission run is isolated with its own session lifecycle.

### 3. Context Compression (Summarization)
To prevent context window overflow, the agent monitors token usage:
- When tokens exceed the `TokenThreshold` (defined in `config.json`), a summary mission is triggered.
- The summary is stored in the `Agent.summaries` map and injected into the system prompt for subsequent turns.
- The original session is cleared to reset the context window.

### 4. MCP Tool Namespacing
Tools discovered from MCP servers are dynamically registered and prefixed with the server name (e.g., `github_create_pull_request`).
- `memory_` tools are kept in the root agent to maintain personalized context.
- Most other MCP tools are delegated to the `ResearchAssistant`.

## 🛡 Security & Constraints
- **SSRF Protection**: All outbound web requests must pass through `internal/tools/validator.go`. Local/private IPs are blocked by default unless `ALLOW_LOCAL_URLS=true` is set.
- **Restricted Shell**: `ShellExecute` is limited to a whitelist of commands defined in `config.json`.
- **Tool Rule**: AI must output ONLY the tool call (no preamble) when invoking a function to avoid system crashes.

## 📝 Conventions
- **Error Handling**: Use `slog` for structured logging. Return errors with context using `fmt.Errorf("...: %w", err)`.
- **Testing**: Use `testify` for assertions. Prefer table-driven tests.
- **Dependency**: Do not add new direct dependencies unless absolutely necessary. Prefer standard library or established packages already in `go.mod`.
