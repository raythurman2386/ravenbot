package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// MaxOutputSize is the maximum allowed output size for a command (100KB).
const MaxOutputSize = 100 * 1024

// limitWriter is a helper that wraps a buffer and enforces a maximum size.
type limitWriter struct {
	buf       *bytes.Buffer
	limit     int64
	n         int64
	truncated bool
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.n >= lw.limit {
		lw.truncated = true
		return len(p), nil
	}

	remaining := lw.limit - lw.n
	if int64(len(p)) > remaining {
		n, err := lw.buf.Write(p[:remaining])
		lw.n += int64(n)
		lw.truncated = true
		return len(p), err
	}

	n, err := lw.buf.Write(p)
	lw.n += int64(n)
	return n, err
}

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

	// Sanitize args (strict check for shell/command injection)
	for _, arg := range args {
		if strings.ContainsAny(arg, ";&|><$(){}[]\"'`\n\r\t\\") {
			return "", fmt.Errorf("invalid character in argument: %s", arg)
		}
	}

	var buf bytes.Buffer
	lw := &limitWriter{buf: &buf, limit: MaxOutputSize}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = lw
	cmd.Stderr = lw

	err := cmd.Run()
	output := buf.String()

	if lw.truncated {
		output += "\n\n[Output truncated due to size limit]"
	}

	if err != nil {
		return output, fmt.Errorf("execution failed: %w", err)
	}

	return output, nil
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
