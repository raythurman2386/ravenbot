package config

import (
	"fmt"
	"os"
)

type Config struct {
	GeminiAPIKey string
}

func LoadConfig() (*Config, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	return &Config{
		GeminiAPIKey: apiKey,
	}, nil
}
