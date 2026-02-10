# 🦅 ravenbot: Autonomous Technical Research Agent

ravenbot is a high-performance, self-hosted autonomous AI agent built in **Go 1.25.6** using the **Google Agent Development Kit (ADK)**. It functions as a proactive technical assistant that researches the latest trends in Golang, AI/LLM, and Geospatial Engineering, delivering high-quality briefings directly to your pocket.

Equipped with a pluggable AI backend supporting **Google Gemini** (Gemini 2.5/Flash) and **Ollama** (local models), ravenbot leverages native grounding and specialized sub-agents to deliver factual, real-time intelligence.

---

## 🚀 Key Features

### 🧠 Advanced Intelligence
- **Flash-First Routing**: Uses **Gemini 2.5 Flash** to intelligently classify prompts as "Simple" or "Complex", routing them to the optimal model to balance speed and reasoning depth.
- **Active Sub-Agents**:
  - **ResearchAssistant**: A specialized agent for deep technical research. It coordinates complex missions and delegates web searches to the `SearchAssistant`.
  - **SearchAssistant**: A dedicated sub-agent powered by **geminitool.GoogleSearch**. By isolating search, RavenBot leverages Gemini 2.5's official grounding while maintaining compatibility with other tools.
  - **SystemManager**: A specialized agent for system diagnostics and health monitoring using official MCP toolsets.
  - **Jules**: An AI software engineer capable of managing repositories and performing coding tasks.
- **Production Grounding**: Uses official **Google Search** grounding for high-accuracy, up-to-date technical research.

### 🔌 Official MCP Integration
ravenbot utilizes the official **Model Context Protocol (MCP)** SDK, allowing it to seamlessly use tools from multiple servers:
- **Filesystem**: Safe file operations within allowed directories.
- **Memory**: Personalized long-term context storage using a dedicated knowledge graph.
- **Weather**: Real-time environmental data.
- **System Metrics**: Real-time system health monitoring (CPU, Memory, Disk) via `sysmetrics`.
- **Sequential Thinking**: Enhanced reasoning for complex problem-solving.

### 💬 Multi-Channel & Interactive
- **Proactive Heartbeat**: Automated daily technical newsletters scheduled via `CronLib`.
- **Two-Way Comms**: Interactive listeners for **Telegram** and **Discord**.
  - `/research <topic>` - Trigger a deep-dive research mission on any subject.
  - `/jules <repo> <task>` - Delegate complex coding or repository tasks to the **Jules Agent API**.
  - `/status` - Check system health (disk, memory, uptime) via **SystemManager**.
- **Secure by Design**: Restricted message processing to authorized Chat/Channel IDs and built-in SSRF protection.

### 💾 Persistence & Memory
- **SQLite Engine**: Tracks headlines and briefings to ensure active knowledge management.
- **Context Compression**: Automatically summarizes long conversations when token thresholds are reached to maintain performance.

---

## 🛠 Tech Stack

- **Core**: Go 1.25.6
- **Framework**: [google.golang.org/adk](https://pkg.go.dev/google.golang.org/adk) (v0.4.0)
- **AI Backend**: Pluggable — **Google AI (Gemini)** or **Ollama** (selected via `AI_BACKEND` env var)
- **AI Models**: Gemini 2.5 Pro & Flash (Google AI) or any Ollama-compatible model.
- **Scheduler**: [github.com/raythurman2386/cronlib](https://github.com/raythurman2386/cronlib)
- **Database**: `modernc.org/sqlite` (v1.44.3)
- **Infrastructure**: Docker & Docker Compose (Optimized for ARM64/Raspberry Pi 5)

---

## 📋 Getting Started

### 1. Prerequisites
- Docker & Docker Compose
- **For Gemini backend**: Get an API Key from [Google AI Studio](https://aistudio.google.com/)
- **For Ollama backend**: A running [Ollama](https://ollama.com/) instance (local or remote)
- (Optional) Telegram/Discord Bot Tokens
- (Optional) Jules Agent API Key

### 2. Deployment (Docker)
ravenbot is designed to run 24/7 in a lightweight Docker container.

```bash
# Clone the repository
git clone https://github.com/raythurman2386/ravenbot.git
cd ravenbot

# Set up your environment
cp .env.example .env
# Edit .env — set AI_BACKEND to "gemini" or "ollama" and configure accordingly

# Launch the agent
docker compose up -d --build
```

---

## ⚙️ Configuration

### Environment Variables (.env)
| Variable | Description |
|----------|-------------|
| `AI_BACKEND` | AI backend to use: `gemini` (default) or `ollama`. |
| `GEMINI_API_KEY` | **Required for Gemini**. Your Google AI Studio API Key. |
| `OLLAMA_BASE_URL` | Ollama API URL (default: `http://localhost:11434/v1`). |
| `OLLAMA_MODEL` | Default Ollama model for both Flash and Pro tiers. |
| `TELEGRAM_BOT_TOKEN` | Token for the Telegram bot. |
| `TELEGRAM_CHAT_ID` | Authorized Telegram Chat ID. |
| `DISCORD_BOT_TOKEN` | Token for the Discord bot. |
| `DISCORD_CHANNEL_ID` | Authorized Discord Channel ID. |
| `JULES_API_KEY` | API Key for Jules Agent delegation. |
| `ALLOW_LOCAL_URLS` | Set to `true` to allow access to local/private IPs (default: `false`). |

---

## 📁 Project Structure

- `cmd/bot/`: Main application entry point and interactive loop.
- `internal/agent/`: Core agent logic, routing, sub-agents, and ADK integration.
- `internal/backend/`: Backend factory — creates `model.LLM` instances.
- `internal/ollama/`: Ollama adapter implementing `model.LLM`.
- `internal/tools/`: Custom tool implementations (Jules, Validator).
- `internal/db/`: Persistence layer (SQLite) for briefings and reminders.
- `internal/notifier/`: Messaging integrations (Telegram, Discord).
- `internal/config/`: Configuration and environment loading.
- `daily_logs/`: Local storage for generated Markdown reports.

---

## 📜 License
MIT – Build something great! 🦅