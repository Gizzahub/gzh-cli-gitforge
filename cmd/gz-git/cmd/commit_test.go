package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJSONMessages(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid single repo",
			input: `{"gzh-cli":"feat: add feature"}`,
			want:  map[string]string{"gzh-cli": "feat: add feature"},
		},
		{
			name:  "valid multiple repos",
			input: `{"gzh-cli":"feat: add feature","gzh-cli-gitforge":"fix: bug fix","gzh-cli-quality":"docs: update README"}`,
			want: map[string]string{
				"gzh-cli":          "feat: add feature",
				"gzh-cli-gitforge": "fix: bug fix",
				"gzh-cli-quality":  "docs: update README",
			},
		},
		{
			name:  "message with special characters",
			input: `{"repo":"feat(core): add \"quoted\" support & more"}`,
			want:  map[string]string{"repo": `feat(core): add "quoted" support & more`},
		},
		{
			name:    "invalid JSON",
			input:   `{not valid json}`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
		{
			name:  "empty object",
			input: `{}`,
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJSONMessages(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSONMessages(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseJSONMessages(%q) got %d entries, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseJSONMessages(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

func TestParseRepoMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		repo    string
		message string
		wantErr bool
	}{
		{
			name:    "valid message",
			input:   "gzh-cli:feat: add new feature",
			repo:    "gzh-cli",
			message: "feat: add new feature",
		},
		{
			name:    "no colon",
			input:   "gzh-cli feat add feature",
			wantErr: true,
		},
		{
			name:    "empty repo",
			input:   ":feat: add feature",
			wantErr: true,
		},
		{
			name:    "empty message",
			input:   "gzh-cli:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, message, err := parseRepoMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepoMessage(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if repo != tt.repo {
				t.Errorf("parseRepoMessage(%q) repo = %q, want %q", tt.input, repo, tt.repo)
			}
			if message != tt.message {
				t.Errorf("parseRepoMessage(%q) message = %q, want %q", tt.input, message, tt.message)
			}
		})
	}
}

func TestLoadMessagesFile(t *testing.T) {
	t.Run("valid JSON file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "messages.json")
		content := `{"gzh-cli":"feat: add feature","gzh-cli-gitforge":"fix: bug"}`
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := loadMessagesFile(filePath)
		if err != nil {
			t.Fatalf("loadMessagesFile() error = %v", err)
		}
		if got["gzh-cli"] != "feat: add feature" {
			t.Errorf("got[gzh-cli] = %q, want %q", got["gzh-cli"], "feat: add feature")
		}
		if got["gzh-cli-gitforge"] != "fix: bug" {
			t.Errorf("got[gzh-cli-gitforge] = %q, want %q", got["gzh-cli-gitforge"], "fix: bug")
		}
	})

	t.Run("invalid JSON file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "bad.json")
		if err := os.WriteFile(filePath, []byte("{bad json}"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := loadMessagesFile(filePath)
		if err == nil {
			t.Error("loadMessagesFile() expected error for invalid JSON")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := loadMessagesFile("/nonexistent/path/to/messages.json")
		if err == nil {
			t.Error("loadMessagesFile() expected error for nonexistent file")
		}
	})
}
