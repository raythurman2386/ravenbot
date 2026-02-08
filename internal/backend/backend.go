// Package backend provides a factory for creating model.LLM instances
// based on the configured AI backend (Google AI or Ollama).
package backend

import (
	"context"
	"fmt"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/ollama"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// geminiClientConfig builds the genai.ClientConfig for Google AI (Gemini).
func geminiClientConfig(cfg *config.Config) *genai.ClientConfig {
	return &genai.ClientConfig{
		APIKey: cfg.GeminiAPIKey,
	}
}

// resolveOllamaModel returns the model name to use, with fallback logic:
// specific override > OllamaModel > ollama.DefaultModel.
func resolveOllamaModel(override, fallback string) string {
	if override != "" {
		return override
	}
	if fallback != "" {
		return fallback
	}
	return ollama.DefaultModel
}

// resolveOllamaBaseURL returns the base URL, falling back to the Ollama default.
func resolveOllamaBaseURL(url string) string {
	if url != "" {
		return url
	}
	return ollama.DefaultBaseURL
}

// NewFlashModel creates a Flash-tier model.LLM based on the configured backend.
func NewFlashModel(ctx context.Context, cfg *config.Config) (model.LLM, error) {
	switch cfg.AIBackend {
	case config.BackendGemini:
		return gemini.NewModel(ctx, cfg.GeminiFlashModel, geminiClientConfig(cfg))
	case config.BackendOllama:
		modelName := resolveOllamaModel(cfg.OllamaFlashModel, cfg.OllamaModel)
		return ollama.New(
			ollama.WithBaseURL(resolveOllamaBaseURL(cfg.OllamaBaseURL)),
			ollama.WithModel(modelName),
		), nil
	default:
		return nil, fmt.Errorf("unsupported AI backend: %s", cfg.AIBackend)
	}
}

// NewProModel creates a Pro-tier model.LLM based on the configured backend.
func NewProModel(ctx context.Context, cfg *config.Config) (model.LLM, error) {
	switch cfg.AIBackend {
	case config.BackendGemini:
		return gemini.NewModel(ctx, cfg.GeminiProModel, geminiClientConfig(cfg))
	case config.BackendOllama:
		modelName := resolveOllamaModel(cfg.OllamaProModel, cfg.OllamaModel)
		return ollama.New(
			ollama.WithBaseURL(resolveOllamaBaseURL(cfg.OllamaBaseURL)),
			ollama.WithModel(modelName),
		), nil
	default:
		return nil, fmt.Errorf("unsupported AI backend: %s", cfg.AIBackend)
	}
}
