package main

import (
	"testing"

	gzhcligitforge "github.com/gizzahub/gzh-cli-gitforge"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stamped string
		want    string
	}{
		{
			name:    "ldflags stamp wins",
			stamped: "0.8.0",
			want:    "0.8.0",
		},
		{
			name:    "unstamped build reports the embedded module version",
			stamped: "dev",
			want:    gzhcligitforge.Version,
		},
		{
			name:    "prerelease stamp is kept verbatim",
			stamped: "0.9.0-rc1",
			want:    "0.9.0-rc1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveVersion(tt.stamped); got != tt.want {
				t.Errorf("resolveVersion(%q) = %q, want %q", tt.stamped, got, tt.want)
			}
		})
	}
}
