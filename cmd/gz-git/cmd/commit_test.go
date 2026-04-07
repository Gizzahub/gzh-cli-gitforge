package cmd

import (
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

func TestParseYAMLMessages(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid single repo",
			input: "gzh-cli: 'feat: add feature'\n",
			want:  map[string]string{"gzh-cli": "feat: add feature"},
		},
		{
			name: "valid multiple repos",
			input: `
gzh-cli: "feat: add feature"
gzh-cli-gitforge: "fix: bug fix"
gzh-cli-quality: "docs: update README"
`,
			want: map[string]string{
				"gzh-cli":          "feat: add feature",
				"gzh-cli-gitforge": "fix: bug fix",
				"gzh-cli-quality":  "docs: update README",
			},
		},
		{
			name:    "invalid YAML",
			input:   "not valid yaml : : :",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseYAMLMessages(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseYAMLMessages(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseYAMLMessages(%q) got %d entries, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseYAMLMessages(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

func TestParseJSONOrYAMLMessages(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid JSON",
			input: `{"gzh-cli":"feat: add feature"}`,
			want:  map[string]string{"gzh-cli": "feat: add feature"},
		},
		{
			name: "valid YAML",
			input: `
gzh-cli: "feat: add feature from yaml"
`,
			want: map[string]string{"gzh-cli": "feat: add feature from yaml"},
		},
		{
			name:    "invalid both",
			input:   "just some gibberish text without any mapping",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJSONOrYAMLMessages(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSONOrYAMLMessages(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseJSONOrYAMLMessages(%q) got %d entries, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseJSONOrYAMLMessages(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}
