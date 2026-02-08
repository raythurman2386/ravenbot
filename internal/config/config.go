package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

type JobConfig struct {
	Name     string            `json:"name"`
	Schedule string            `json:"schedule"`
	Type     string            `json:"type"`
	Params   map[string]string `json:"params"`
}

type BotConfig struct {
	SystemPrompt         string  `json:"systemPrompt"`
	ResearchSystemPrompt string  `json:"researchSystemPrompt"`
	SystemManagerPrompt  string  `json:"systemManagerPrompt"`
	JulesPrompt          string  `json:"julesPrompt"`
	FlashTokenLimit      int64   `json:"flashTokenLimit"`
	ProTokenLimit        int64   `json:"proTokenLimit"`
	TokenThreshold       float64 `json:"tokenThreshold"`
	SummaryPrompt        string  `json:"summaryPrompt"`
	HelpMessage          string  `json:"helpMessage"`
	StatusPrompt         string  `json:"statusPrompt"`
	RoutingPrompt        string  `json:"routingPrompt"`
}

// Supported AI backend values.
const (
	BackendGemini = "gemini"
	BackendOllama = "ollama"
)

type Config struct {
	// AI Backend selection ("gemini" or "ollama")
	AIBackend string

	// Google AI (Gemini) settings
	GeminiAPIKey     string
	GeminiFlashModel string
	GeminiProModel   string

	// Ollama settings (used when AIBackend == "ollama")
	OllamaBaseURL    string
	OllamaModel      string // Default model for both Flash and Pro
	OllamaFlashModel string // Optional override for Flash
	OllamaProModel   string // Optional override for Pro

	TelegramBotToken string
	TelegramChatID   int64
	DiscordBotToken  string
	DiscordChannelID string
	JulesAPIKey      string
	DBPath           string                     `json:"dbPath"`
	Bot              BotConfig                  `json:"bot"`
	MCPServers       map[string]MCPServerConfig `json:"mcpServers"`
	Jobs             []JobConfig                `json:"jobs"`
	AllowedCommands  []string                   `json:"allowedCommands"`
}

func LoadConfig() (*Config, error) {
	backend := strings.ToLower(os.Getenv("AI_BACKEND"))
	if backend == "" {
		backend = BackendGemini
	}
	if backend != BackendGemini && backend != BackendOllama {
		return nil, fmt.Errorf("unsupported AI_BACKEND %q: must be %q or %q", backend, BackendGemini, BackendOllama)
	}

	cfg := &Config{
		AIBackend:        backend,
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		DiscordBotToken:  os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
		JulesAPIKey:      os.Getenv("JULES_API_KEY"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		DBPath:           "data/ravenbot.db",
		Bot:              BotConfig{},
	}

	// Backend-specific configuration
	switch backend {
	case BackendGemini:
		if cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required when AI_BACKEND=%s", BackendGemini)
		}
		cfg.GeminiFlashModel = os.Getenv("GEMINI_FLASH_MODEL")
		if cfg.GeminiFlashModel == "" {
			cfg.GeminiFlashModel = os.Getenv("FLASH_MODEL")
		}
		if cfg.GeminiFlashModel == "" {
			cfg.GeminiFlashModel = "gemini-3.0-flash-preview"
		}
		cfg.GeminiProModel = os.Getenv("GEMINI_PRO_MODEL")
		if cfg.GeminiProModel == "" {
			cfg.GeminiProModel = os.Getenv("PRO_MODEL")
		}
		if cfg.GeminiProModel == "" {
			cfg.GeminiProModel = "gemini-3.0-pro-preview"
		}
	case BackendOllama:
		cfg.OllamaBaseURL = os.Getenv("OLLAMA_BASE_URL")
		cfg.OllamaModel = os.Getenv("OLLAMA_MODEL")
		cfg.OllamaFlashModel = os.Getenv("OLLAMA_FLASH_MODEL")
		cfg.OllamaProModel = os.Getenv("OLLAMA_PRO_MODEL")
	}

	// 2. Load Configuration from JSON file
	configPath := "config.json"
	if _, err := os.Stat(configPath); err == nil {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %w", err)
		}
		defer func() { _ = file.Close() }()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
		slog.Info("Loaded configuration from config.json")
	} else {
		slog.Warn("No config.json found, relying on environment variables only")
	}

	// Set defaults if still zero
	if cfg.Bot.FlashTokenLimit <= 0 {
		cfg.Bot.FlashTokenLimit = 1000000 // Default to 1M
	}
	if cfg.Bot.ProTokenLimit <= 0 {
		cfg.Bot.ProTokenLimit = 1000000 // Default to 1M
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
