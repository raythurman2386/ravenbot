package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveReport(t *testing.T) {
	content := "# Daily Report\nTest content"

	// Use a cleanup to remove the test logs directory
	defer func() { _ = os.RemoveAll("daily_logs") }()

	path, err := SaveReport("daily_logs", content)
	assert.NoError(t, err)
	assert.NotEmpty(t, path)

	// Verify file exists and content matches
	savedContent, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, content, string(savedContent))

	// Verify filename format
	_, filename := filepath.Split(path)
	assert.Contains(t, filename, "Ravenwood_Updates_")
	assert.Contains(t, filename, ".md")
}
