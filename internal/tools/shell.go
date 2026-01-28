package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AllowedCommands is a whitelist of safe commands the agent can execute.
var AllowedCommands = map[string]bool{
	"df":     true,
	"free":   true,
	"uptime": true,
	"whoami": true,
	"date":   true,
	"ls":     true,
}

// ShellExecute runs a restricted set of shell commands.
func ShellExecute(ctx context.Context, command string, args []string) (string, error) {
	if !AllowedCommands[command] {
		return "", fmt.Errorf("command '%s' is not allowed", command)
	}

	// Sanitize args if needed (basic check for now)
	for _, arg := range args {
		if strings.ContainsAny(arg, ";&|><$(){}") {
			return "", fmt.Errorf("invalid character in argument: %s", arg)
		}
	}

	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("execution failed: %w", err)
	}

	return string(output), nil
}
