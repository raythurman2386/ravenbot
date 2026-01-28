package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// JulesTask represents the payload for creating a Jules session.
type JulesTask struct {
	Source string `json:"source"`
	Prompt string `json:"prompt"`
}

// DelegateToJules calls the alpha Jules Agent API to perform a repository task.
func DelegateToJules(ctx context.Context, apiKey, repo, task string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("JULES_API_KEY is not set")
	}

	url := "https://jules.googleapis.com/v1alpha/sessions"
	payload := JulesTask{
		Source: repo,
		Prompt: task,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal jules payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create jules request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("jules api call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jules api returned status: %s", resp.Status)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode jules response: %w", err)
	}

	sessionID := result["name"].(string)
	return fmt.Sprintf("Jules task initiated successfully. Session ID: %s", sessionID), nil
}
