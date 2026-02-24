package cliutil_test

import (
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

func TestValidateFormat(t *testing.T) {
	allowed := cliutil.CoreFormats

	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"valid format default", "default", false},
		{"valid format json", "json", false},
		{"invalid format xml", "xml", true},
		{"empty format", "", true},
		{"invalid format with space", " json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cliutil.ValidateFormat(tt.format, allowed)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsMachineFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   bool
	}{
		{"json is machine format", "json", true},
		{"llm is machine format", "llm", true},
		{"csv is machine format", "csv", true},
		{"default is not machine format", "default", false},
		{"table is not machine format", "table", false},
		{"uppercase JSON is machine format", "JSON", true},
		{"uppercase LLM is machine format", "LLM", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cliutil.IsMachineFormat(tt.format); got != tt.want {
				t.Errorf("IsMachineFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
