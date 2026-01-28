package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func SaveReport(dir, content string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("Ravenwood_Updates_%s.md", date)
	path := filepath.Join(dir, filename)

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write report to %s: %w", path, err)
	}

	return path, nil
}
