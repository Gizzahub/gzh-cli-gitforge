// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBranchList_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected BranchList
		wantErr  bool
	}{
		{
			name:     "single branch string",
			yaml:     "defaultBranch: develop",
			expected: BranchList{"develop"},
			wantErr:  false,
		},
		{
			name:     "comma-separated string",
			yaml:     "defaultBranch: develop,master",
			expected: BranchList{"develop", "master"},
			wantErr:  false,
		},
		{
			name:     "comma-separated with spaces",
			yaml:     "defaultBranch: develop, master, main",
			expected: BranchList{"develop", "master", "main"},
			wantErr:  false,
		},
		{
			name:     "yaml list format",
			yaml:     "defaultBranch:\n  - develop\n  - master",
			expected: BranchList{"develop", "master"},
			wantErr:  false,
		},
		{
			name:     "yaml inline list format",
			yaml:     "defaultBranch: [develop, master]",
			expected: BranchList{"develop", "master"},
			wantErr:  false,
		},
		{
			name:     "empty string",
			yaml:     "defaultBranch: \"\"",
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "empty list",
			yaml:     "defaultBranch: []",
			expected: BranchList{},
			wantErr:  false,
		},
		{
			name:     "comma-separated with extra commas",
			yaml:     "defaultBranch: develop,,master",
			expected: BranchList{"develop", "master"},
			wantErr:  false,
		},
		{
			name:     "comma-separated with leading/trailing spaces",
			yaml:     "defaultBranch: \"  develop , master  \"",
			expected: BranchList{"develop", "master"},
			wantErr:  false,
		},
		{
			name:     "single branch with trailing comma",
			yaml:     "defaultBranch: develop,",
			expected: BranchList{"develop"},
			wantErr:  false,
		},
		{
			name:     "only commas and spaces",
			yaml:     "defaultBranch: \", , ,\"",
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg struct {
				DefaultBranch BranchList `yaml:"defaultBranch"`
			}

			err := yaml.Unmarshal([]byte(tt.yaml), &cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(cfg.DefaultBranch) != len(tt.expected) {
					t.Errorf("UnmarshalYAML() got %v, want %v", cfg.DefaultBranch, tt.expected)
					return
				}
				for i, v := range cfg.DefaultBranch {
					if v != tt.expected[i] {
						t.Errorf("UnmarshalYAML() got[%d] = %q, want %q", i, v, tt.expected[i])
					}
				}
			}
		})
	}
}

func TestBranchList_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    BranchList
		expected string
	}{
		{
			name:     "nil list",
			input:    nil,
			expected: "defaultBranch: null\n",
		},
		{
			name:     "empty list",
			input:    BranchList{},
			expected: "defaultBranch: null\n",
		},
		{
			name:     "single branch",
			input:    BranchList{"develop"},
			expected: "defaultBranch: develop\n",
		},
		{
			name:     "two branches",
			input:    BranchList{"develop", "master"},
			expected: "defaultBranch: develop,master\n",
		},
		{
			name:     "three branches",
			input:    BranchList{"develop", "master", "main"},
			expected: "defaultBranch: develop,master,main\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := struct {
				DefaultBranch BranchList `yaml:"defaultBranch"`
			}{
				DefaultBranch: tt.input,
			}

			data, err := yaml.Marshal(&cfg)
			if err != nil {
				t.Errorf("MarshalYAML() error = %v", err)
				return
			}

			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() got %q, want %q", string(data), tt.expected)
			}
		})
	}
}

func TestBranchList_String(t *testing.T) {
	tests := []struct {
		name     string
		input    BranchList
		expected string
	}{
		{
			name:     "nil list",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty list",
			input:    BranchList{},
			expected: "",
		},
		{
			name:     "single branch",
			input:    BranchList{"develop"},
			expected: "develop",
		},
		{
			name:     "multiple branches",
			input:    BranchList{"develop", "master", "main"},
			expected: "develop,master,main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBranchList_First(t *testing.T) {
	tests := []struct {
		name     string
		input    BranchList
		expected string
	}{
		{
			name:     "nil list",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty list",
			input:    BranchList{},
			expected: "",
		},
		{
			name:     "single branch",
			input:    BranchList{"develop"},
			expected: "develop",
		},
		{
			name:     "multiple branches",
			input:    BranchList{"develop", "master", "main"},
			expected: "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.First(); got != tt.expected {
				t.Errorf("First() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBranchList_RoundTrip(t *testing.T) {
	// Test that marshaling and unmarshaling preserves the data
	tests := []struct {
		name  string
		input BranchList
	}{
		{
			name:  "single branch",
			input: BranchList{"develop"},
		},
		{
			name:  "multiple branches",
			input: BranchList{"develop", "master"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := struct {
				DefaultBranch BranchList `yaml:"defaultBranch"`
			}{
				DefaultBranch: tt.input,
			}

			// Marshal
			data, err := yaml.Marshal(&cfg)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}

			// Unmarshal
			var result struct {
				DefaultBranch BranchList `yaml:"defaultBranch"`
			}
			if err := yaml.Unmarshal(data, &result); err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}

			// Compare
			if len(result.DefaultBranch) != len(tt.input) {
				t.Errorf("RoundTrip: got %v, want %v", result.DefaultBranch, tt.input)
				return
			}
			for i, v := range result.DefaultBranch {
				if v != tt.input[i] {
					t.Errorf("RoundTrip: got[%d] = %q, want %q", i, v, tt.input[i])
				}
			}
		})
	}
}
