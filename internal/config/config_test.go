package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Run("success with GCP project", func(t *testing.T) {
		_ = os.Setenv("GCP_PROJECT", "test-project")
		defer func() { _ = os.Unsetenv("GCP_PROJECT") }()

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.Equal(t, "test-project", cfg.GCPProject)
		assert.Equal(t, "us-central1", cfg.GCPLocation, "should default location to us-central1")
	})

	t.Run("success with custom location", func(t *testing.T) {
		_ = os.Setenv("GCP_PROJECT", "test-project")
		_ = os.Setenv("GCP_LOCATION", "europe-west1")
		defer func() {
			_ = os.Unsetenv("GCP_PROJECT")
			_ = os.Unsetenv("GCP_LOCATION")
		}()

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.Equal(t, "test-project", cfg.GCPProject)
		assert.Equal(t, "europe-west1", cfg.GCPLocation)
	})

	t.Run("missing GCP project", func(t *testing.T) {
		_ = os.Unsetenv("GCP_PROJECT")

		cfg, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GCP_PROJECT environment variable is not set")
	})
}
