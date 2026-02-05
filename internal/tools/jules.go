package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GithubRepoContext provides context for a GitHub repository.
type GithubRepoContext struct {
	StartingBranch *string `json:"startingBranch,omitempty"`
}

// SourceContext specifies the source repository for a Jules session.
type SourceContext struct {
	Source            string             `json:"source"`
	GithubRepoContext *GithubRepoContext `json:"githubRepoContext,omitempty"`
}

// JulesSessionRequest represents the payload for creating a Jules session.
type JulesSessionRequest struct {
	Prompt              string        `json:"prompt"`
	SourceContext       SourceContext `json:"sourceContext"`
	Title               string        `json:"title,omitempty"`
	RequirePlanApproval bool          `json:"requirePlanApproval,omitempty"`
	AutomationMode      string        `json:"automationMode,omitempty"`
}

// DelegateToJules calls the alpha Jules Agent API to perform a repository task.
// The repo should be in the format "owner/repo" (e.g., "raythurman2386/ravenbot").
// Note: The repository must be connected to Jules via https://jules.google first.
func DelegateToJules(ctx context.Context, apiKey, repo, task string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("JULES_API_KEY is not set")
	}

	// Parse owner/repo format
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo format, expected 'owner/repo', got: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	url := "https://jules.googleapis.com/v1alpha/sessions"

	// Build the proper request payload
	// Source format: sources/github/{owner}/{repo}
	payload := JulesSessionRequest{
		Prompt: task,
		SourceContext: SourceContext{
			Source:            fmt.Sprintf("sources/github/%s/%s", owner, repoName),
			GithubRepoContext: &GithubRepoContext{}, // omitempty handles nil StartingBranch
		},
		Title:               fmt.Sprintf("ravenbot Task: %s", truncateString(task, 50)),
		RequirePlanApproval: false, // Auto-approve for autonomous operation
		AutomationMode:      "AUTO_CREATE_PR",
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

	client := NewSafeClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("jules api call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body for error details
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode jules response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// Try to extract error message from response
		if errMsg, ok := result["error"].(map[string]any); ok {
			if msg, ok := errMsg["message"].(string); ok {
				// Provide helpful context for common errors
				if strings.Contains(msg, "not found") {
					return "", fmt.Errorf("jules api error: %s (Hint: Make sure the repo is connected at https://jules.google)", msg)
				}
				return "", fmt.Errorf("jules api error: %s", msg)
			}
		}
		return "", fmt.Errorf("jules api returned status: %s", resp.Status)
	}

	sessionName, ok := result["name"].(string)
	if !ok {
		return "", fmt.Errorf("jules response missing session name")
	}

	return fmt.Sprintf("Jules task initiated successfully. Session: %s", sessionName), nil
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
