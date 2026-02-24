package cliutil_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

func TestWriteJSON(t *testing.T) {
	data := map[string]string{"key": "value"}

	t.Run("compact json", func(t *testing.T) {
		var buf bytes.Buffer
		err := cliutil.WriteJSON(&buf, data, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := `{"key":"value"}`
		got := strings.TrimSpace(buf.String())
		if got != want {
			t.Errorf("WriteJSON (compact) = %q, want %q", got, want)
		}
	})

	t.Run("pretty json", func(t *testing.T) {
		var buf bytes.Buffer
		err := cliutil.WriteJSON(&buf, data, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := "{\n  \"key\": \"value\"\n}"
		got := strings.TrimSpace(buf.String())
		if got != want {
			t.Errorf("WriteJSON (pretty) = %q, want %q", got, want)
		}
	})
}

func TestWriteLLM(t *testing.T) {
	data := map[string]string{"hello": "world"}

	var buf bytes.Buffer
	err := cliutil.WriteLLM(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() == 0 {
		t.Errorf("WriteLLM produced empty output")
	}

	str := buf.String()
	if !strings.Contains(str, "hello") || !strings.Contains(str, "world") {
		t.Errorf("WriteLLM output didn't contain expected data: %q", str)
	}
}
