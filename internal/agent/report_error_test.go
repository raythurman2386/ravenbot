package agent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveReport_Errors(t *testing.T) {
	t.Run("mkdir error", func(t *testing.T) {
		// Create a file where the directory should be to force a mkdir error
		tempFile := "daily_logs_mkdir_fail"
		err := os.WriteFile(tempFile, []byte("file"), 0644)
		assert.NoError(t, err)
		defer os.Remove(tempFile)

		_, err = SaveReport(tempFile+"/subdir", "content")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create logs directory")
	})

	t.Run("write file error", func(t *testing.T) {
		// This can be triggered by making the directory read-only
		dir := "daily_logs_readonly"
		err := os.MkdirAll(dir, 0555) // Read-only
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		_, err = SaveReport(dir, "content")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write report to")
	})
}
