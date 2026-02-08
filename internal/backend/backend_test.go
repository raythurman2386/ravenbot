package backend

import (
	"context"
	"testing"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/ollama"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlashModel_Ollama(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.Config
		wantModelName string
	}{
		{
			name: "uses OllamaFlashModel override",
			cfg: &config.Config{
				AIBackend:        config.BackendOllama,
				OllamaBaseURL:    "http://localhost:11434/v1",
				OllamaModel:      "default-model",
				OllamaFlashModel: "flash-override",
			},
			wantModelName: "ollama/flash-override",
		},
		{
			name: "falls back to OllamaModel",
			cfg: &config.Config{
				AIBackend:     config.BackendOllama,
				OllamaBaseURL: "http://localhost:11434/v1",
				OllamaModel:   "shared-model",
			},
			wantModelName: "ollama/shared-model",
		},
		{
			name: "falls back to ollama.DefaultModel",
			cfg: &config.Config{
				AIBackend: config.BackendOllama,
			},
			wantModelName: "ollama/" + ollama.DefaultModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewFlashModel(context.Background(), tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantModelName, m.Name())
		})
	}
}

func TestNewProModel_Ollama(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.Config
		wantModelName string
	}{
		{
			name: "uses OllamaProModel override",
			cfg: &config.Config{
				AIBackend:      config.BackendOllama,
				OllamaModel:    "default-model",
				OllamaProModel: "pro-override",
			},
			wantModelName: "ollama/pro-override",
		},
		{
			name: "falls back to OllamaModel",
			cfg: &config.Config{
				AIBackend:   config.BackendOllama,
				OllamaModel: "shared-model",
			},
			wantModelName: "ollama/shared-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewProModel(context.Background(), tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantModelName, m.Name())
		})
	}
}

func TestNewFlashModel_UnknownBackend(t *testing.T) {
	cfg := &config.Config{AIBackend: "unknown"}
	m, err := NewFlashModel(context.Background(), cfg)
	assert.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "unsupported AI backend")
}

func TestNewProModel_UnknownBackend(t *testing.T) {
	cfg := &config.Config{AIBackend: "unknown"}
	m, err := NewProModel(context.Background(), cfg)
	assert.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "unsupported AI backend")
}
