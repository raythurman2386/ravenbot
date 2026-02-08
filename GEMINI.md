# ravenbot: AI Agent Context & Architecture

This document provides structural and behavioral context for AI agents working on the **ravenbot** repository.

## 🛠 Tech Stack
- **Language**: Go 1.25.6
- **AI Framework**: Google Agent Development Kit (ADK) `google.golang.org/adk`
- **AI Backend**: Pluggable via `AI_BACKEND` env var:
  - `gemini` (default): Google AI (`genai.BackendGoogleAI`) using API Key.
  - `ollama`: Local/remote Ollama via OpenAI-compatible API (`internal/ollama`)
- **Backend Factory**: `internal/backend` — creates `model.LLM` instances based on config; the agent never imports `gemini` or `ollama` directly
- **Generative AI**: `google.golang.org/genai` (Gemini 3.0 Pro & Flash for Google AI, any compatible model for Ollama)
- **Database**: SQLite via `modernc.org/sqlite` (CGO-free)
- **Networking**: Custom safe HTTP client with SSRF protection in `internal/tools/validator.go`
- **Browsing**: Headless Chrome via `github.com/chromedp/chromedp`

## 📁 Project Structure
```text
.
├── cmd/bot/                # Application entry point and main event loop
├── internal/
│   ├── agent/             # Core AI logic (Agent struct, Routing, Sub-agents)
│   │   ├── agent.go       # ADK Agent initialization and model routing
│   │   ├── tools.go       # Tool registration (Core, Technical, MCP)
│   │   └── report.go      # Markdown report generation logic
│   ├── backend/           # Backend factory (Gemini in Google AI or Ollama) for model.LLM
│   ├── ollama/            # Ollama adapter (OpenAI-compatible API → model.LLM)
│   ├── mcp/               # Custom MCP client (Stdio & SSE transports)
│   ├── tools/             # Native tool implementations (Search, Browser, RSS, Scraper)
│   ├── db/                # SQLite persistence (Headlines, Briefings)
│   ├── notifier/          # Telegram & Discord delivery systems
│   └── config/            # Environment and JSON configuration loading
├── config.json            # Bot settings, MCP servers, and scheduled jobs
└── Makefile               # Development workflow (build, test, lint)
```

## 🧠 Core Patterns

### 1. Flash-First Model Routing
RavenBot uses a two-stage routing pattern with the classification prompt stored in `config.json`:
- **Classification**: Every user prompt is first sent to **Gemini 3 Flash** with the prompt defined in `bot.routingPrompt` to classify it as "Simple" or "Complex".
- **Execution**:
    - "Simple" requests are handled by the **Flash Runner** for low-latency chat.
    - "Complex" requests (reasoning, tool use, technical tasks) are routed to the **Pro Runner**.

### 2. Active Sub-Agents
RavenBot utilizes three active specialized sub-agents for distinct domains:
- **ResearchAssistant**:
  - **Goal**: Deep technical research, web search, and data aggregation.
  - **Tools**: `GoogleSearch`, `BrowseWeb`, `ScrapePage`, `FetchRSS`.
  - **Lifecycle**: Isolated session for each research mission.
- **SystemManager**:
  - **Goal**: System health monitoring and diagnostics.
  - **Tools**: `sysmetrics_*` (native system metrics via MCP).
  - **Usage**: Invoked by `/status` or when system issues are detected.
- **Jules**:
  - **Goal**: Coding, repository management, and GitHub interactions.
  - **Tools**: `github_*` and file operations.
  - **Usage**: Invoked explicitly via `/jules` or for complex coding requests.

### 3. Context Compression (Summarization)
To prevent context window overflow, the agent monitors token usage:
- When tokens exceed the `TokenThreshold` (defined in `config.json`), a summary mission is triggered.
- The summary is stored in the `Agent.summaries` map and injected into the system prompt for subsequent turns.
- The original session is cleared to reset the context window.

### 4. Active MCP Servers & Tool Namespacing
Tools discovered from the active MCP servers are dynamically registered and prefixed with the server name.
- **`memory_`**: Kept in the root agent to maintain personalized user context and history.
- **`sysmetrics_`**: Delegated to the **SystemManager**.
- **`github_`**: Delegated to **Jules** for repository operations.
- **`weather_` / `filesystem_`**: Generally available to the Pro Runner or specific sub-agents as needed.
- **`sequentialthinking_`**: Available to the Pro Runner to enhance reasoning capabilities.

## 🛡 Security & Constraints
- **AI Backend Auth**:
  - **Vertex AI**: Models authenticate via Application Default Credentials (ADC) — no static API keys. Set `GCP_PROJECT` and `GCP_LOCATION` env vars; credentials are managed by `gcloud auth application-default login` or a mounted service account key.
  - **Ollama**: No authentication required. Set `OLLAMA_BASE_URL` to point at your Ollama instance.
- **SSRF Protection**: All outbound web requests must pass through `internal/tools/validator.go`. Local/private IPs are blocked by default unless `ALLOW_LOCAL_URLS=true` is set.
- **Restricted Shell**: `ShellExecute` is limited to a whitelist of commands defined in `config.json`.
- **Tool Rule**: AI must output ONLY the tool call (no preamble) when invoking a function to avoid system crashes.

## 📝 Conventions
- **Error Handling**: Use `slog` for structured logging. Return errors with context using `fmt.Errorf("...: %w", err)`.
- **Testing**: Use `testify` for assertions. Prefer table-driven tests.
- **Dependency**: Do not add new direct dependencies unless absolutely necessary. Prefer standard library or established packages already in `go.mod`.
