package tools

import (
	"context"
	"strings"
	"testing"
)

func TestShellExecutor_IsAllowed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		allowedCommands []string
		command         string
		expected        bool
	}{
		{
			name:            "Default whitelist - allowed",
			allowedCommands: nil,
			command:         "df",
			expected:        true,
		},
		{
			name:            "Default whitelist - not allowed",
			allowedCommands: nil,
			command:         "rm",
			expected:        false,
		},
		{
			name:            "Custom whitelist - allowed",
			allowedCommands: []string{"ls", "grep"},
			command:         "ls",
			expected:        true,
		},
		{
			name:            "Custom whitelist - not allowed",
			allowedCommands: []string{"ls", "grep"},
			command:         "df",
			expected:        false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := NewShellExecutor(tt.allowedCommands)
			got := s.IsAllowed(tt.command)
			if got != tt.expected {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.command, got, tt.expected)
			}
		})
	}
}

func TestShellExecutor_Execute_Sanitization(t *testing.T) {
	t.Parallel()
	s := NewShellExecutor([]string{"echo"})
	ctx := context.Background()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "Safe arguments",
			args:    []string{"hello", "world"},
			wantErr: false,
		},
		{
			name:    "Semicolon injection",
			args:    []string{"hello;", "rm -rf /"},
			wantErr: true,
		},
		{
			name:    "Ampersand injection",
			args:    []string{"&", "ls"},
			wantErr: true,
		},
		{
			name:    "Pipe injection",
			args:    []string{"|", "cat /etc/passwd"},
			wantErr: true,
		},
		{
			name:    "Redirect injection",
			args:    []string{">", "file.txt"},
			wantErr: true,
		},
		{
			name:    "Subshell injection",
			args:    []string{"$(ls)"},
			wantErr: true,
		},
		{
			name:    "Parentheses injection",
			args:    []string{"(ls)"},
			wantErr: true,
		},
		{
			name:    "Braces injection",
			args:    []string{"{ls}"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := s.Execute(ctx, "echo", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), "invalid character in argument") {
				t.Errorf("Execute() error = %v, want error containing 'invalid character in argument'", err)
			}
		})
	}
}

func TestShellExecutor_Execute_NotAllowed(t *testing.T) {
	t.Parallel()
	s := NewShellExecutor([]string{"echo"})
	ctx := context.Background()

	_, err := s.Execute(ctx, "ls", []string{})
	if err == nil {
		t.Errorf("Execute() should have failed for non-whitelisted command")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("Execute() error = %v, want error containing 'not allowed'", err)
	}
}

func TestShellExecutor_AllowedList(t *testing.T) {
	t.Parallel()
	commands := []string{"ls", "grep"}
	s := NewShellExecutor(commands)
	list := s.AllowedList()

	if len(list) != len(commands) {
		t.Errorf("AllowedList() length = %d, want %d", len(list), len(commands))
	}

	for _, cmd := range commands {
		found := false
		for _, l := range list {
			if l == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllowedList() missing %q", cmd)
		}
	}
}
