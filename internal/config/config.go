package config

import (
	"fmt"
	"log"
	"os"
)

type Config struct {
	GeminiAPIKey     string
	TelegramBotToken string
	TelegramChatID   int64
	DiscordBotToken  string
	DiscordChannelID string
	JulesAPIKey      string
}

func LoadConfig() (*Config, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	// Optional configurations for notifiers
	var chatID int64
	if cid := os.Getenv("TELEGRAM_CHAT_ID"); cid != "" {
		if _, err := fmt.Sscanf(cid, "%d", &chatID); err != nil {
			log.Printf("Warning: Invalid TELEGRAM_CHAT_ID: %v", err)
		}
	}

	return &Config{
		GeminiAPIKey:     apiKey,
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   chatID,
		DiscordBotToken:  os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
		JulesAPIKey:      os.Getenv("JULES_API_KEY"),
	}, nil
}
