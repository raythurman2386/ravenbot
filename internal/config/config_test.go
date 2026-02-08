package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("default backend is vertex", func(t *testing.T) {
		_ = os.Setenv("GCP_PROJECT", "test-project")
		_ = os.Unsetenv("AI_BACKEND")
		defer func() { _ = os.Unsetenv("GCP_PROJECT") }()

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, BackendVertex, cfg.AIBackend)
		assert.Equal(t, "test-project", cfg.GCPProject)
		assert.Equal(t, "us-central1", cfg.GCPLocation, "should default location to us-central1")
	})

	t.Run("vertex with custom location", func(t *testing.T) {
		_ = os.Setenv("GCP_PROJECT", "test-project")
		_ = os.Setenv("GCP_LOCATION", "europe-west1")
		defer func() {
			_ = os.Unsetenv("GCP_PROJECT")
			_ = os.Unsetenv("GCP_LOCATION")
		}()

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, "test-project", cfg.GCPProject)
		assert.Equal(t, "europe-west1", cfg.GCPLocation)
	})

	t.Run("vertex requires GCP_PROJECT", func(t *testing.T) {
		_ = os.Unsetenv("GCP_PROJECT")
		_ = os.Unsetenv("AI_BACKEND")

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GCP_PROJECT")
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
		assert.Empty(t, cfg.GCPProject)
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
