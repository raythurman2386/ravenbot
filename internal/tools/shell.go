package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultAllowedCommands is the fallback whitelist if none is configured.
var DefaultAllowedCommands = []string{
	"df", "free", "uptime", "whoami", "date",
}

// ShellExecutor handles safe command execution with a configurable whitelist.
type ShellExecutor struct {
	allowedCommands map[string]bool
}

// NewShellExecutor creates a new ShellExecutor with the given allowed commands.
// If no commands are provided, it uses the default whitelist.
func NewShellExecutor(allowedCommands []string) *ShellExecutor {
	if len(allowedCommands) == 0 {
		allowedCommands = DefaultAllowedCommands
	}

	allowed := make(map[string]bool)
	for _, cmd := range allowedCommands {
		allowed[cmd] = true
	}

	return &ShellExecutor{
		allowedCommands: allowed,
	}
}

// Execute runs a restricted shell command if it's in the whitelist.
func (s *ShellExecutor) Execute(ctx context.Context, command string, args []string) (string, error) {
	if !s.allowedCommands[command] {
		return "", fmt.Errorf("command '%s' is not allowed", command)
	}

	// Sanitize args (basic check for shell injection)
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

// IsAllowed checks if a command is in the whitelist.
func (s *ShellExecutor) IsAllowed(command string) bool {
	return s.allowedCommands[command]
}

// AllowedList returns a slice of all allowed commands.
func (s *ShellExecutor) AllowedList() []string {
	list := make([]string, 0, len(s.allowedCommands))
	for cmd := range s.allowedCommands {
		list = append(list, cmd)
	}
	return list
}
