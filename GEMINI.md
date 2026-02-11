# ravenbot: AI Agent Context & Architecture

This document provides structural and behavioral context for AI agents working on the **ravenbot** repository.

## 🛠 Tech Stack
- **Language**: Go 1.25.6
- **AI Framework**: Google Agent Development Kit (ADK) v0.4.0 `google.golang.org/adk`
- **AI Backend**: Pluggable via `AI_BACKEND` env var:
  - `gemini` (default): Google AI Studio using API Key.
  - `ollama`: Local/remote Ollama via OpenAI-compatible API.
- **Backend Factory**: `internal/backend` — creates `model.LLM` instances; ensures strict role alternation (User/Model) required by Gemini.
- **Generative AI**: `google.golang.org/genai` (**Gemini 2.5 Pro & Flash** for grounding support).
- **Database**: SQLite via `modernc.org/sqlite` (CGO-free).
- **Official MCP SDK**: Uses `github.com/modelcontextprotocol/go-sdk` for production-grade tool connections.

## 📁 Project Structure
```text
.
├── cmd/bot/                # Application entry point and main event loop
├── internal/
│   ├── agent/             # Core AI logic (Agent struct, Sub-agents, A2A delegation)
│   │   └── agent.go       # ADK Agent initialization, sub-agent tree, and tool registration
│   ├── handler/           # Unified message routing and command handling
│   ├── backend/           # Backend factory and Gemini role wrapper (SystemRoleWrapper)
│   ├── ollama/            # Ollama adapter (OpenAI-compatible API → model.LLM)
│   ├── tools/             # Native tool implementations (SSRF Validator, Jules API, Web Search)
│   ├── db/                # SQLite persistence (Briefings, Reminders, Session Summaries)
│   ├── notifier/          # Telegram & Discord delivery systems
│   ├── stats/             # Token usage and system statistics tracking
│   └── config/            # Environment and JSON configuration loading
├── config.json            # Bot settings, MCP servers, and scheduled jobs
└── Makefile               # Development workflow (build, test, lint)
```

## 🧠 Core Patterns

### 1. Flash-First Model Routing
RavenBot uses a two-stage routing pattern:
- **Classification**: User prompts are first sent to **Gemini 2.5 Flash** to classify them as "Simple" or "Complex".
- **Execution**:
    - "Simple" requests are handled by the **Gemini 2.5 Flash** (our main powerhouse model).
    - "Complex" requests (deep reasoning, technical tasks) are routed to the **Gemini 2.5 Pro** model.

> [!IMPORTANT]
> **Model Integrity**: AI agents MUST NOT modify or downgrade the configured Gemini models (currently 2.5 Flash and 2.5 Pro). These models are selected for optimal performance until Gemini 3 models reach General Availability.

### 2. Active Sub-Agents (A2A Architecture)
RavenBot utilizes specialized sub-agents to bypass model limitations and enhance accuracy:
- **ResearchAssistant**:
  - **Goal**: Deep technical research and data aggregation.
  - **Search Implementation**: Uses a custom `web_search` tool that performs a standalone Gemini API call with official Google Search grounding. This architecture bypasses the Gemini restriction that prevents mixing grounding with other function-calling toolsets (like MCP) in a single turn.
  - **Toolsets**: Uses all active MCP toolsets (weather, memory, filesystem) for context.
- **SystemManager**:
  - **Goal**: System health monitoring and diagnostics.
  - **Tools**: Official MCP toolsets (e.g., `sysmetrics`).
- **Jules**:
  - **Goal**: Coding, repository management, and GitHub interactions.
  - **Tools**: `JulesTask` and GitHub MCP.

### 3. Official MCP Toolsets
Integrated via `google.golang.org/adk/tool/mcptoolset`. 
- **Tool Namespacing**: Tools are automatically discovered and registered from `config.json`.
- **Transports**: Supports official `CommandTransport` (stdio) and `SSEClientTransport`.

### 4. Context Compression (Summarization)
- Long conversations are automatically summarized and persisted to SQLite.
- Summaries are injected into the system prompt to maintain long-term context while keeping the active window lean.

## 🛡 Security & Constraints
- **SSRF Protection**: All outbound requests must pass through `internal/tools/validator.go`. The `NewSafeClient` provides DNS rebinding protection and port blacklisting.
- **Role Requirements**: The `SystemRoleWrapper` in `internal/backend` ensures the message history follows the strict User -> Model alternation pattern required by the Gemini API. It merges consecutive same-role messages and normalizes roles like "assistant" or "system".
- **Pi Optimization**: Builds use `GOARCH=arm64` and `-ldflags="-s -w"` for Raspberry Pi 5 performance.

## 📝 Conventions
- **Structured Logging**: Use `slog` for structured JSON logs. All logs are persisted to `logs/combined.log`.
- **Error Handling**: Return errors with context using `fmt.Errorf("...: %w", err)`.
- **Dependencies**: Rely on official ADK and MCP SDK packages whenever possible.