package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_ = os.Setenv("GEMINI_API_KEY", "test-key")
		defer func() { _ = os.Unsetenv("GEMINI_API_KEY") }()

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.Len(t, cfg.GeminiAPIKeys, 1)
		assert.Equal(t, "test-key", cfg.GeminiAPIKeys[0])
	})

	t.Run("success multiple keys", func(t *testing.T) {
		_ = os.Setenv("GEMINI_API_KEY", "key1,key2, key3")
		defer func() { _ = os.Unsetenv("GEMINI_API_KEY") }()

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.Len(t, cfg.GeminiAPIKeys, 3)
		assert.Equal(t, "key1", cfg.GeminiAPIKeys[0])
		assert.Equal(t, "key2", cfg.GeminiAPIKeys[1])
		assert.Equal(t, "key3", cfg.GeminiAPIKeys[2])
	})

	t.Run("missing key", func(t *testing.T) {
		_ = os.Unsetenv("GEMINI_API_KEY")

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GEMINI_API_KEY environment variable is not set")
	})

	t.Run("empty key", func(t *testing.T) {
		_ = os.Setenv("GEMINI_API_KEY", "  , ,  ")
		defer func() { _ = os.Unsetenv("GEMINI_API_KEY") }()

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GEMINI_API_KEY environment variable is empty or invalid")
	})
}
