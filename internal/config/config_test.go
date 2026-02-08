package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("default backend is gemini", func(t *testing.T) {
		_ = os.Setenv("GEMINI_API_KEY", "test-key")
		_ = os.Unsetenv("AI_BACKEND")
		defer func() { _ = os.Unsetenv("GEMINI_API_KEY") }()

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, BackendGemini, cfg.AIBackend)
		assert.Equal(t, "test-key", cfg.GeminiAPIKey)
	})

	t.Run("gemini requires GEMINI_API_KEY", func(t *testing.T) {
		_ = os.Unsetenv("GEMINI_API_KEY")
		_ = os.Unsetenv("AI_BACKEND")

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GEMINI_API_KEY")
	})

	t.Run("ollama backend loads without GCP_PROJECT", func(t *testing.T) {
		_ = os.Setenv("AI_BACKEND", "ollama")
		_ = os.Setenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")
		_ = os.Setenv("OLLAMA_MODEL", "llama3.2")
		_ = os.Unsetenv("GCP_PROJECT")
		defer func() {
			_ = os.Unsetenv("AI_BACKEND")
			_ = os.Unsetenv("OLLAMA_BASE_URL")
			_ = os.Unsetenv("OLLAMA_MODEL")
		}()

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, BackendOllama, cfg.AIBackend)
		assert.Equal(t, "http://localhost:11434/v1", cfg.OllamaBaseURL)
		assert.Equal(t, "llama3.2", cfg.OllamaModel)
		assert.Empty(t, cfg.GeminiAPIKey)
	})

	t.Run("ollama with flash and pro model overrides", func(t *testing.T) {
		_ = os.Setenv("AI_BACKEND", "ollama")
		_ = os.Setenv("OLLAMA_FLASH_MODEL", "qwen3:1.7b")
		_ = os.Setenv("OLLAMA_PRO_MODEL", "qwen3:8b")
		defer func() {
			_ = os.Unsetenv("AI_BACKEND")
			_ = os.Unsetenv("OLLAMA_FLASH_MODEL")
			_ = os.Unsetenv("OLLAMA_PRO_MODEL")
		}()

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, "qwen3:1.7b", cfg.OllamaFlashModel)
		assert.Equal(t, "qwen3:8b", cfg.OllamaProModel)
	})

	t.Run("invalid backend value returns error", func(t *testing.T) {
		_ = os.Setenv("AI_BACKEND", "openai")
		defer func() { _ = os.Unsetenv("AI_BACKEND") }()

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "unsupported AI_BACKEND")
	})

	t.Run("backend value is case insensitive", func(t *testing.T) {
		_ = os.Setenv("AI_BACKEND", "OLLAMA")
		defer func() { _ = os.Unsetenv("AI_BACKEND") }()

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, BackendOllama, cfg.AIBackend)
	})
}
