// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package hooks

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "single word",
			input:    "make",
			expected: []string{"make"},
		},
		{
			name:     "multiple words",
			input:    "make build",
			expected: []string{"make", "build"},
		},
		{
			name:     "extra whitespace",
			input:    "  make   build  ",
			expected: []string{"make", "build"},
		},
		{
			name:     "double quoted argument",
			input:    `echo "hello world"`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "single quoted argument",
			input:    "echo 'hello world'",
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "mixed quotes",
			input:    `cmd "arg1" 'arg2'`,
			expected: []string{"cmd", "arg1", "arg2"},
		},
		{
			name:     "tabs as separators",
			input:    "make\tbuild\ttest",
			expected: []string{"make", "build", "test"},
		},
		{
			name:     "path with spaces",
			input:    `mkdir -p "path with spaces"`,
			expected: []string{"mkdir", "-p", "path with spaces"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCommand(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseCommand(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasUnsafeCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "safe command",
			input:    "make build",
			expected: false,
		},
		{
			name:     "pipe",
			input:    "cat file | grep foo",
			expected: true,
		},
		{
			name:     "redirect",
			input:    "echo foo > file.txt",
			expected: true,
		},
		{
			name:     "append redirect",
			input:    "echo foo >> file.txt",
			expected: true,
		},
		{
			name:     "input redirect",
			input:    "cat < file.txt",
			expected: true,
		},
		{
			name:     "command substitution dollar",
			input:    "echo $(whoami)",
			expected: true,
		},
		{
			name:     "command substitution backtick",
			input:    "echo `whoami`",
			expected: true,
		},
		{
			name:     "and operator",
			input:    "cmd1 && cmd2",
			expected: true,
		},
		{
			name:     "or operator",
			input:    "cmd1 || cmd2",
			expected: true,
		},
		{
			name:     "semicolon",
			input:    "cmd1; cmd2",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasUnsafeCharacters(tt.input)
			if result != tt.expected {
				t.Errorf("HasUnsafeCharacters(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		hooks   *config.Hooks
		wantErr bool
	}{
		{
			name:    "nil hooks",
			hooks:   nil,
			wantErr: false,
		},
		{
			name:    "empty hooks",
			hooks:   &config.Hooks{},
			wantErr: false,
		},
		{
			name: "safe hooks",
			hooks: &config.Hooks{
				Before: []string{"mkdir -p logs"},
				After:  []string{"make test"},
			},
			wantErr: false,
		},
		{
			name: "unsafe before hook",
			hooks: &config.Hooks{
				Before: []string{"echo foo | tee file"},
			},
			wantErr: true,
		},
		{
			name: "unsafe after hook",
			hooks: &config.Hooks{
				After: []string{"cat > output.txt"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.hooks)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		global   *config.Hooks
		local    *config.Hooks
		expected *config.Hooks
	}{
		{
			name:     "both nil",
			global:   nil,
			local:    nil,
			expected: nil,
		},
		{
			name:   "global only",
			global: &config.Hooks{Before: []string{"cmd1"}, After: []string{"cmd2"}},
			local:  nil,
			expected: &config.Hooks{
				Before: []string{"cmd1"},
				After:  []string{"cmd2"},
			},
		},
		{
			name:   "local only",
			global: nil,
			local:  &config.Hooks{Before: []string{"cmd1"}, After: []string{"cmd2"}},
			expected: &config.Hooks{
				Before: []string{"cmd1"},
				After:  []string{"cmd2"},
			},
		},
		{
			name:   "both present",
			global: &config.Hooks{Before: []string{"global1"}, After: []string{"global2"}},
			local:  &config.Hooks{Before: []string{"local1"}, After: []string{"local2"}},
			expected: &config.Hooks{
				Before: []string{"global1", "local1"},
				After:  []string{"global2", "local2"},
			},
		},
		{
			name:     "both empty",
			global:   &config.Hooks{},
			local:    &config.Hooks{},
			expected: nil, // empty hooks return nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Merge(tt.global, tt.local)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Merge() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestExecuteCommands(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		commands []string
		workDir  string
		wantErr  bool
	}{
		{
			name:     "empty commands",
			commands: nil,
			workDir:  tmpDir,
			wantErr:  false,
		},
		{
			name:     "simple echo command",
			commands: []string{"echo hello"},
			workDir:  tmpDir,
			wantErr:  false,
		},
		{
			name:     "multiple commands",
			commands: []string{"echo one", "echo two"},
			workDir:  tmpDir,
			wantErr:  false,
		},
		{
			name:     "nonexistent workdir",
			commands: []string{"echo test"},
			workDir:  "/nonexistent/path",
			wantErr:  true,
		},
		{
			name:     "failing command",
			commands: []string{"false"},
			workDir:  tmpDir,
			wantErr:  true,
		},
		{
			name:     "nonexistent command",
			commands: []string{"nonexistent_command_xyz"},
			workDir:  tmpDir,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecuteCommands(context.Background(), tt.commands, tt.workDir, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteCommands() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		hooks   *config.Hooks
		phase   string
		wantErr bool
	}{
		{
			name:    "nil hooks",
			hooks:   nil,
			phase:   "before",
			wantErr: false,
		},
		{
			name: "before phase",
			hooks: &config.Hooks{
				Before: []string{"echo before"},
				After:  []string{"echo after"},
			},
			phase:   "before",
			wantErr: false,
		},
		{
			name: "after phase",
			hooks: &config.Hooks{
				Before: []string{"echo before"},
				After:  []string{"echo after"},
			},
			phase:   "after",
			wantErr: false,
		},
		{
			name: "invalid phase",
			hooks: &config.Hooks{
				Before: []string{"echo test"},
			},
			phase:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Execute(context.Background(), tt.hooks, tt.phase, tmpDir, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteCreatesFile(t *testing.T) {
	// Test that hooks can actually modify the filesystem
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	hooks := &config.Hooks{
		After: []string{"touch " + testFile},
	}

	err := Execute(context.Background(), hooks, "after", tmpDir, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Expected hook to create file, but it doesn't exist")
	}
}
