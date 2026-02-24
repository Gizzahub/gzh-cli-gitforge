package cliutil

import (
	"encoding/json"
	"io"

	"github.com/gizzahub/gzh-cli-core/cli"
)

// WriteJSON writes the given value as JSON to the writer.
// If verbose is true, it pretty-prints with indentation.
func WriteJSON(w io.Writer, v any, verbose bool) error {
	encoder := json.NewEncoder(w)
	if verbose {
		encoder.SetIndent("", "  ")
	}
	// Avoid escaping HTML characters if not strictly necessary,
	// but default is typically fine. Let's keep it standard.
	return encoder.Encode(v)
}

// WriteLLM writes the given value as LLM-formatted structure.
// This uses gzh-cli-core/cli's Output formatter set to "llm".
func WriteLLM(w io.Writer, v any) error {
	out := cli.NewOutput().SetWriter(w).SetFormat("llm")
	return out.Print(v)
}
