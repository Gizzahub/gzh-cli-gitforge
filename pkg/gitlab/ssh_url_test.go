// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package gitlab

import (
	"testing"
)

func TestExtractHostFromURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		wantHost string
	}{
		{
			name:     "standard HTTPS URL",
			baseURL:  "https://gitlab.polypia.net",
			wantHost: "gitlab.polypia.net",
		},
		{
			name:     "HTTPS with port (API endpoint)",
			baseURL:  "https://gitlab.polypia.net:8443",
			wantHost: "gitlab.polypia.net",
		},
		{
			name:     "gitlab.com",
			baseURL:  "https://gitlab.com",
			wantHost: "gitlab.com",
		},
		{
			name:     "HTTPS with path",
			baseURL:  "https://gitlab.com/api/v4",
			wantHost: "gitlab.com",
		},
		{
			name:     "empty URL",
			baseURL:  "",
			wantHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHostFromURL(tt.baseURL)
			if got != tt.wantHost {
				t.Errorf("extractHostFromURL() = %v, want %v", got, tt.wantHost)
			}
		})
	}
}

func TestBuildSSHURL(t *testing.T) {
	tests := []struct {
		name        string
		sshHost     string
		sshPort     int
		projectPath string
		want        string
	}{
		{
			name:        "custom port",
			sshHost:     "gitlab.polypia.net",
			sshPort:     2224,
			projectPath: "archmagece1/iac/devenv",
			want:        "ssh://git@gitlab.polypia.net:2224/archmagece1/iac/devenv.git",
		},
		{
			name:        "custom port with .git suffix",
			sshHost:     "gitlab.polypia.net",
			sshPort:     2224,
			projectPath: "archmagece1/iac/devenv.git",
			want:        "ssh://git@gitlab.polypia.net:2224/archmagece1/iac/devenv.git",
		},
		{
			name:        "standard port 22",
			sshHost:     "gitlab.com",
			sshPort:     22,
			projectPath: "group/project",
			want:        "git@gitlab.com:group/project.git",
		},
		{
			name:        "no port specified (default)",
			sshHost:     "gitlab.com",
			sshPort:     0,
			projectPath: "group/project",
			want:        "git@gitlab.com:group/project.git",
		},
		{
			name:        "empty host",
			sshHost:     "",
			sshPort:     2224,
			projectPath: "group/project",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				sshHost: tt.sshHost,
				sshPort: tt.sshPort,
			}
			got := p.buildSSHURL(tt.projectPath)
			if got != tt.want {
				t.Errorf("buildSSHURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
