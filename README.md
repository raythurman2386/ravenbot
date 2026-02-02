# 🦅 ravenbot: Autonomous Technical Research Agent

ravenbot is a high-performance, self-hosted autonomous AI agent built in **Go 1.25+**. It functions as a proactive technical assistant that researches the latest trends in Golang, AI/LLM, and Geospatial Engineering, delivering high-quality briefings directly to your pocket.

Equipped with a **Gemini 3 Pro** brain, ravenbot can browse the web, execute system commands, and even delegate complex repository tasks to the **Gemini Jules Agent**.

---

## 🚀 Key Features

### 🧠 Advanced Intelligence
- **Native AI Power**: Driven by Google's `gemini-3-flash-preview` with native function calling and multi-turn reasoning.
- **Smart Tools**: Equipped with a professional toolbelt:
  - **FetchRSS**: Real-time news gathering from technical sources.
  - **ScrapePage**: High-fidelity text extraction from technical articles.
  - **BrowseWeb**: A headless browser pilot (`chromedp`) for JS-heavy dynamic websites.
  - **ShellExecute**: Restricted local execution for system monitoring (df, free, uptime).

### 💬 Multi-Channel & Interactive
- **Proactive Heartbeat**: Automated daily technical newsletters scheduled via `CronLib`.
- **Two-Way Comms**: Interactive listeners for **Telegram**, **Discord**, and **CLI**.
  - `/research <topic>` - Trigger a deep-dive research mission on any subject.
  - `/jules <repo> <task>` - Delegate complex coding or repository tasks to the **Jules Agent API**.
- **Secure by Design**: Restricted message processing to authorized IDs and SSRF protection for all web tools.

### 💾 Persistence & Memory
- **SQLite Engine**: Tracks headlines to ensure you never receive duplicate news.
- **RAG-Ready**: Persists daily briefings for historical reference and future trend analysis.

---

## 🛠 Tech Stack

- **Core**: Go 1.25+
- **Brain**: [google.golang.org/genai](https://github.com/googleapis/go-genai) (Gemini 3 Pro)
- **Scheduler**: [github.com/raythurman2386/cronlib](https://github.com/raythurman2386/cronlib)
- **Browser**: `chromedp`
- **Database**: `modernc.org/sqlite` (CGO-free)
- **Infrastructure**: Docker & Docker Compose (Optimized for ARM64/Raspberry Pi 5)

---

## 📋 Getting Started

### 1. Prerequisites
- Docker & Docker Compose
- Google Gemini API Key
- (Optional) Telegram Bot Token & Chat ID
- (Optional) Discord Bot Token & Channel ID
- (Optional) Jules Agent API Key

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

## 🔌 Extending with MCP (Model Context Protocol)

ravenbot supports the **Model Context Protocol (MCP)**, allowing you to easily add new tools without modifying the code. You can connect to any standard MCP server (e.g., Filesystem, GitHub, Postgres, Slack).

### Configuration
Create a `config.json` file in the root directory to define your MCP servers:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/files"]
    },
    "git": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "mcp/git"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"]
    }
  }
}
```

**Note:** For the GitHub server, you must set `GITHUB_PERSONAL_ACCESS_TOKEN` in your `.env` file.

ravenbot will automatically discover tools from these servers (e.g., `filesystem_read_file`, `git_diff`, `github_create_pull_request`) and make them available to the agent.

---

## 📁 Project Structure

- `cmd/bot/`: Main application entry point and interactive loop.
- `internal/agent/`: Core agent logic, function calling, and persona management.
- `internal/tools/`: Implementation of the Agent's toolset (Web, RSS, Shell, Browser, Jules).
- `internal/db/`: Persistence layer for headlines and briefings.
- `internal/notifier/`: Messaging integrations for Telegram and Discord.
- `daily_logs/`: Local storage for generated Markdown reports.

---

## 📜 License
MIT – Build something great! 🦅
