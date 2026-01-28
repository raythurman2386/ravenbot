package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		os.Setenv("GEMINI_API_KEY", "test-key")
		defer os.Unsetenv("GEMINI_API_KEY")

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.Equal(t, "test-key", cfg.GeminiAPIKey)
	})

	t.Run("missing key", func(t *testing.T) {
		os.Unsetenv("GEMINI_API_KEY")

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GEMINI_API_KEY environment variable is not set")
	})
}
