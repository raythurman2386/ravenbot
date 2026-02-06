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
	FlashModel           string  `json:"flashModel"`
	ProModel             string  `json:"proModel"`
	SystemPrompt         string  `json:"systemPrompt"`
	ResearchSystemPrompt string  `json:"researchSystemPrompt"`
	SystemManagerPrompt  string  `json:"systemManagerPrompt"`
	JulesPrompt          string  `json:"julesPrompt"`
	TokenLimit           int64   `json:"tokenLimit"`
	TokenThreshold       float64 `json:"tokenThreshold"`
	SummaryPrompt        string  `json:"summaryPrompt"`
	HelpMessage          string  `json:"helpMessage"`
	StatusPrompt         string  `json:"statusPrompt"`
}

type Config struct {
	GeminiAPIKeys    []string
	TelegramBotToken string
	TelegramChatID   int64
	DiscordBotToken  string
	DiscordChannelID string
	JulesAPIKey      string
	Bot              BotConfig                  `json:"bot"`
	MCPServers       map[string]MCPServerConfig `json:"mcpServers"`
	Jobs             []JobConfig                `json:"jobs"`
	AllowedCommands  []string                   `json:"allowedCommands"`
}

func LoadConfig() (*Config, error) {
	apiKeyEnv := os.Getenv("GEMINI_API_KEY")
	if apiKeyEnv == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	var apiKeys []string
	for _, key := range strings.Split(apiKeyEnv, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			apiKeys = append(apiKeys, key)
		}
	}

	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is empty or invalid")
	}

	cfg := &Config{
		GeminiAPIKeys:    apiKeys,
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		DiscordBotToken:  os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
		JulesAPIKey:      os.Getenv("JULES_API_KEY"),
		Bot:              BotConfig{},
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
