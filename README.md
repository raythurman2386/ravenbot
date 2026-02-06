# 🦅 ravenbot: Autonomous Technical Research Agent

ravenbot is a high-performance, self-hosted autonomous AI agent built in **Go 1.25.6** using the **Google Agent Development Kit (ADK)**. It functions as a proactive technical assistant that researches the latest trends in Golang, AI/LLM, and Geospatial Engineering, delivering high-quality briefings directly to your pocket.

Equipped with a **Gemini 3 Pro** brain and a **Gemini 3 Flash** router, ravenbot can browse the web, execute system commands, and delegate complex tasks to specialized sub-agents.

---

## 🚀 Key Features

### 🧠 Advanced Intelligence
- **Flash-First Routing**: Uses **Gemini 3 Flash** to intelligently classify prompts as "Simple" or "Complex", routing them to the optimal model to balance speed and reasoning depth.
- **Specialized Sub-Agents**:
  - **ResearchAssistant**: A dedicated **Gemini 3 Pro** powered sub-agent for deep technical research, web search, and data aggregation.
  - **SystemManager**: A specialized agent for system diagnostics and health monitoring using native system metrics.
  - **Jules**: An AI software engineer capable of managing repositories, performing coding tasks, and interacting with GitHub.
- **Smart Tools**: Equipped with a professional native toolbelt:
  - **GoogleSearch**: Integrated search tool for ground-truth verification.
  - **FetchRSS**: Real-time news gathering with automatic database deduplication.
  - **ScrapePage**: High-fidelity text extraction from technical articles.
  - **BrowseWeb**: A headless browser pilot (`chromedp`) for JS-heavy dynamic websites.
  - **Memory Tools**: Active context management to personalize interactions based on history.

### 🔌 Multi-Server MCP Integration
ravenbot supports the **Model Context Protocol (MCP)**, allowing it to seamlessly use tools from multiple servers:
- **Filesystem**: Safe file operations within allowed directories.
- **GitHub & Git**: Repository management, PR creation, and version control (via `Jules`).
- **Memory**: Personalized long-term context storage using a dedicated knowledge graph.
- **Weather**: Real-time environmental data.
- **System Metrics**: Real-time system health monitoring (CPU, Memory, Disk) via `sysmetrics`.
- **Sequential Thinking**: Enhanced reasoning for complex problem-solving.

### 💬 Multi-Channel & Interactive
- **Proactive Heartbeat**: Automated daily technical newsletters scheduled via `CronLib`.
- **Two-Way Comms**: Interactive listeners for **Telegram**, **Discord**, and **CLI**.
  - `/research <topic>` - Trigger a deep-dive research mission on any subject.
  - `/jules <repo> <task>` - Delegate complex coding or repository tasks to the **Jules Agent API**.
  - `/status` - Check system health (disk, memory, uptime) via **SystemManager**.
- **Secure by Design**: Restricted message processing to authorized Chat/Channel IDs and built-in SSRF protection.

### 💾 Persistence & Memory
- **SQLite Engine**: Tracks headlines to ensure you never receive duplicate news.
- **Context Compression**: Automatically summarizes long conversations when token thresholds are reached to maintain performance.

---

## 🛠 Tech Stack

- **Core**: Go 1.25.6
- **Framework**: [google.golang.org/adk](https://pkg.go.dev/google.golang.org/adk) (v0.4.0)
- **AI Models**: Gemini 3 Pro & Gemini 3 Flash (via `google.golang.org/genai` v1.44.0)
- **Scheduler**: [github.com/raythurman2386/cronlib](https://github.com/raythurman2386/cronlib)
- **Browser**: `chromedp` (v0.14.2)
- **Database**: `modernc.org/sqlite` (v1.44.3)
- **Infrastructure**: Docker & Docker Compose (Optimized for ARM64/Raspberry Pi 5)

---

## 📋 Getting Started

### 1. Prerequisites
- Docker & Docker Compose
- Google Gemini API Key
- (Optional) Telegram/Discord Bot Tokens
- (Optional) Jules Agent API Key
- (Optional) GitHub Personal Access Token (for MCP)

### 2. Deployment (Docker)
ravenbot is designed to run 24/7 in a lightweight Docker container.

```bash
# Clone the repository
git clone https://github.com/raythurman2386/ravenbot.git
cd ravenbot

# Set up your environment
cp .env.example .env
# Edit .env with your keys

# Launch the agent
docker compose up -d --build
```

### 3. Interactive Mode (CLI)
If you aren't using messaging apps, you can interact with ravenbot directly through the container terminal:

```bash
docker attach ravenbot-ravenbot-1
# Then type:
/research Go 1.26 performance
```

---

## ⚙️ Configuration

### Environment Variables (.env)
| Variable | Description |
|----------|-------------|
| `GEMINI_API_KEY` | **Required**. Your Google Gemini API key. |
| `TELEGRAM_BOT_TOKEN` | Token for the Telegram bot. |
| `TELEGRAM_CHAT_ID` | Authorized Telegram Chat ID. |
| `DISCORD_BOT_TOKEN` | Token for the Discord bot. |
| `DISCORD_CHANNEL_ID` | Authorized Discord Channel ID. |
| `JULES_API_KEY` | API Key for Jules Agent delegation. |
| `GITHUB_PERSONAL_ACCESS_TOKEN` | Token for GitHub MCP tools. |
| `ALLOW_LOCAL_URLS` | Set to `true` to allow access to local/private IPs (default: `false`). |

### MCP Servers (config.json)
MCP servers are defined in `config.json`. ravenbot automatically discovers and namespaces their tools (e.g., `github_create_issue`).

Current active servers:
- **filesystem**: `@modelcontextprotocol/server-filesystem`
- **sequential-thinking**: `@modelcontextprotocol/server-sequential-thinking`
- **git**: `@cyanheads/git-mcp-server`
- **github**: `@modelcontextprotocol/server-github`
- **memory**: `@modelcontextprotocol/server-memory`
- **weather**: `goweathermcp`
- **sysmetrics**: `sysmetrics-mcp`

---

## 📁 Project Structure

- `cmd/bot/`: Main application entry point and interactive loop.
- `internal/agent/`: Core agent logic, routing, sub-agents, and ADK integration.
- `internal/tools/`: Native tool implementations (Web, RSS, Shell, Browser, Jules).
- `internal/mcp/`: Custom MCP client for Stdio and SSE transports.
- `internal/db/`: Persistence layer (SQLite) for headlines and briefings.
- `internal/notifier/`: Messaging integrations (Telegram, Discord).
- `internal/config/`: Configuration and environment loading.
- `daily_logs/`: Local storage for generated Markdown reports.

---

## 📜 License
MIT – Build something great! 🦅
