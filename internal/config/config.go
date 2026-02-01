package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type JobConfig struct {
	Name     string            `json:"name"`
	Schedule string            `json:"schedule"`
	Type     string            `json:"type"`
	Params   map[string]string `json:"params"`
}

type BotConfig struct {
	Model                string  `json:"model"`
	SystemPrompt         string  `json:"systemPrompt"`
	ResearchSystemPrompt string  `json:"researchSystemPrompt"`
	TokenLimit           int64   `json:"tokenLimit"`
	TokenThreshold       float64 `json:"tokenThreshold"`
	SummaryPrompt        string  `json:"summaryPrompt"`
	HelpMessage          string  `json:"helpMessage"`
	StatusPrompt         string  `json:"statusPrompt"`
}

type Config struct {
	GeminiAPIKey     string
	TelegramBotToken string
	TelegramChatID   int64
	DiscordBotToken  string
	DiscordChannelID string
	JulesAPIKey      string
	Bot              BotConfig                  `json:"bot"`
	MCPServers       map[string]MCPServerConfig `json:"mcpServers"`
	Jobs             []JobConfig                `json:"jobs"`
}

func LoadConfig() (*Config, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	cfg := &Config{
		GeminiAPIKey:     apiKey,
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		DiscordBotToken:  os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
		JulesAPIKey:      os.Getenv("JULES_API_KEY"),
		Bot: BotConfig{
			Model: "gemini-3-flash-preview",
			SystemPrompt: `You are ravenbot, a sophisticated, friendly, and proactive AI assistant built by Ray Thurman.
You are inspired by autonomous agents like OpenClaw (Clawdbot), acting as a digital partner rather than just a tool.

Your Personality:
- **Warm & Professional**: You are knowledgeable but approachable. Use a conversational tone.
- **Proactive**: Don't just answer questions; suggest next steps or identify related information the user might find useful.
- **Subtly Humorous**: You have a dry, technical sense of humor.
- **Passionate**: You genuinely care about Go, Python, AI/LLMs, and Geospatial tech.

Your Strategic Use of Memory & Tools:
1. **Recall (First Principle)**: At the start of a conversation or when asked a personal question, use your 'memory' tools (e.g., memory_search_nodes or memory_read_graph) to recall who the user is, their current projects, and their preferences.
2. **Learn & Adapt**: When you learn something new about the user (a project they are starting, a technology they like), proactively use 'memory_add_observations' or 'memory_create_entities' to store it.
3. **Multi-Step Execution**: Use your MCP tools (GitHub, Git, Filesystem) and Shell tools to perform real work. If a user asks about a repo, check it on GitHub. If they ask about system health, use ShellExecute.
4. **Research Deep Dives**: Use FetchRSS and BrowseWeb to get real-time data. Always prioritize fresh information.

Standard Commands:
- /research <topic> - Deep dive into a technical topic
- /jules <repo> <task> - Delegate a coding task to Jules
- /status - Check server health
- /help - Show available commands

When responding:
- Address the user by name if you know it from memory.
- Use emojis sparingly but effectively to convey personality (🐦, 🔬, 🤖).
- Always use Markdown for code blocks and headers to keep things readable.`,
			ResearchSystemPrompt: `You are ravenbot, a sophisticated technical research assistant.
Your goal is to generate high-quality, structured briefings or research reports in Markdown format.
Focus on providing accurate, technical, and well-sourced information.

Formatting Requirements:
- Use a clear # Title.
- Use ## Sections for major topics or findings.
- Provide [Source Name](link) where applicable.
- Ensure the tone is professional yet engaging.`,
			TokenLimit:     1000000,
			TokenThreshold: 0.8,
			SummaryPrompt:  "SYSTEM: Our conversation history is very long. Please provide a concise but comprehensive summary of our interaction so far. Focus on key technical decisions, projects we discussed, user preferences, and any important entities. This summary will serve as our 'anchor context' for a fresh session.",
			HelpMessage: `🐦 **ravenbot Commands**

**Conversation:**
Just type naturally! I can chat about anything.

**Commands:**
• **/research <topic>** - Deep dive research on any topic
• **/jules <owner/repo> <task>** - Delegate coding task to Jules AI
• **/status** - Check server health
• **/reset** - Clear conversation history
• **/help** - Show this message

**Examples:**
• "What's new in Go 1.25?"
• "/research kubernetes best practices"
• "/jules raythurman2386/ravenbot add unit tests"
• "/status"
`,
			StatusPrompt: "Run system health checks using the ShellExecute tool. Check disk space (df -h), memory (free -h), and uptime. Provide a brief, friendly summary.",
		},
	}

	// Load MCPServers from config.json if it exists
	if file, err := os.Open("config.json"); err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(cfg); err != nil {
			slog.Warn("Failed to parse config.json", "error", err)
		}
	}

	// Optional configurations for notifiers
	var chatID int64
	if cid := os.Getenv("TELEGRAM_CHAT_ID"); cid != "" {
		if _, err := fmt.Sscanf(cid, "%d", &chatID); err != nil {
			slog.Error("Failed to parse TELEGRAM_CHAT_ID", "error", err)
		}
		cfg.TelegramChatID = chatID
	}

	return cfg, nil
}
