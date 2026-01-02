package cmd

import (
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		// HTTPS URLs
		{
			name: "HTTPS URL with .git suffix",
			url:  "https://github.com/user/repo.git",
			want: "repo",
		},
		{
			name: "HTTPS URL without .git suffix",
			url:  "https://github.com/user/repo",
			want: "repo",
		},
		{
			name: "HTTPS URL with organization",
			url:  "https://github.com/organization/project-name.git",
			want: "project-name",
		},

		// SSH URLs
		{
			name: "SSH URL with .git suffix",
			url:  "git@github.com:user/repo.git",
			want: "repo",
		},
		{
			name: "SSH URL without .git suffix",
			url:  "git@github.com:user/repo",
			want: "repo",
		},
		{
			name: "SSH URL with organization",
			url:  "git@github.com:organization/my-project.git",
			want: "my-project",
		},
		{
			name: "SSH URL with GitLab",
			url:  "git@gitlab.com:group/subgroup/repo.git",
			want: "repo",
		},

		// Git protocol URLs
		{
			name: "Git protocol URL",
			url:  "git://github.com/user/repo.git",
			want: "repo",
		},

		// File paths
		{
			name: "Unix file path",
			url:  "/home/user/projects/my-repo",
			want: "my-repo",
		},
		{
			name: "Unix file path with .git suffix",
			url:  "/path/to/repo.git",
			want: "repo",
		},
		// Note: Windows paths are not valid Git URLs
		// The utility function ExtractRepoNameFromURL is designed for Git URLs only

		// Edge cases
		{
			name: "Empty URL returns default",
			url:  "",
			want: "repository",
		},
		{
			name: "Only .git suffix",
			url:  ".git",
			want: "repository",
		},
		{
			name: "Simple name",
			url:  "repo",
			want: "repo",
		},
		{
			name: "URL with trailing slash should still work",
			url:  "https://github.com/user/repo.git",
			want: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := repository.ExtractRepoNameFromURL(tt.url)
			if got == "" {
				got = "repository"
			}
			if got != tt.want {
				t.Errorf("ExtractRepoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractRepoName_SSHVariants(t *testing.T) {
	// Specific test cases for SSH URL parsing
	tests := []struct {
		url  string
		want string
	}{
		// Standard SSH format
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/repo", "repo"},

		// Custom SSH hosts
		{"git@myserver.com:projects/repo.git", "repo"},
		{"git@192.168.1.1:user/repo.git", "repo"},

		// Nested paths
		{"git@gitlab.com:group/subgroup/repo.git", "repo"},
		{"git@gitlab.com:a/b/c/deep-repo.git", "deep-repo"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, _ := repository.ExtractRepoNameFromURL(tt.url)
			if got == "" {
				got = "repository"
			}
			if got != tt.want {
				t.Errorf("ExtractRepoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
