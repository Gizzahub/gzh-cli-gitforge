//go:build parity

package parity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizerDurationDiagnostic(t *testing.T) {
	normalizer := newNormalizer(t.TempDir())
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "nanoseconds", input: "Duration=41ns", want: "Duration=<DURATION>"},
		{name: "microseconds ascii", input: "Duration=41us", want: "Duration=<DURATION>"},
		{name: "microseconds unicode", input: "Duration=41µs", want: "Duration=<DURATION>"},
		{name: "milliseconds", input: "Duration=41ms", want: "Duration=<DURATION>"},
		{name: "seconds", input: "Duration=41s", want: "Duration=<DURATION>"},
		{name: "minutes", input: "Duration=41m", want: "Duration=<DURATION>"},
		{name: "hours", input: "Duration=41h", want: "Duration=<DURATION>"},
		{name: "compound", input: "ready Duration=1h2m3.4s, next", want: "ready Duration=<DURATION>, next"},
		{name: "raw line break", input: "Duration=41ms\n", want: "Duration=<DURATION>\n"},
		{name: "escaped line break", input: `Duration=41ms\n`, want: `Duration=<DURATION>\n`},
		{name: "double encoded line break", input: `Duration=41ms\\n`, want: `Duration=<DURATION>\\n`},
		{name: "unrelated key", input: "duration=41ms", want: "duration=41ms"},
		{name: "prefixed key", input: "xDuration=41ms", want: "xDuration=41ms"},
		{name: "invalid duration", input: "Duration=not-a-duration", want: "Duration=not-a-duration"},
		{name: "suffixed duration", input: "Duration=41ms-extra", want: "Duration=41ms-extra"},
		{name: "escaped non line break", input: `Duration=41ms\path`, want: `Duration=41ms\path`},
		{name: "overflow duration", input: "Duration=999999999999999999999h", want: "Duration=999999999999999999999h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizer.norm(tt.input); got != tt.want {
				t.Fatalf("norm(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProviderMetadataPreservesProtocolText(t *testing.T) {
	raw := `{"provider":{"name":"gz-git","capabilities":{},"version":"gz-git version 0.9.1"},"reason":"gz-git version 0.9.1","argv":["gz-git version 0.9.1"],"receipt":{"provider":{"name":"gz-git","capabilities":{},"version":"gz-git version 0.9.1"}}}`
	wantBytes := strings.Replace(raw, `"version":"gz-git version 0.9.1"`, `"version":"gz-git version \u003cGZ_GIT_VERSION\u003e"`, 1)
	if got := canonicalGZGitMetadataJSON(raw); got != wantBytes {
		t.Fatalf("non-identity bytes changed: %s != %s", got, wantBytes)
	}
	var before, after map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(canonicalGZGitMetadataJSON(raw)), &after); err != nil {
		t.Fatal(err)
	}
	provider, ok := after["provider"].(map[string]interface{})
	if !ok || provider["version"] != gzGitVersionPlaceholder {
		t.Fatal("provider identity was not normalized")
	}
	for _, field := range []string{"reason", "argv", "receipt"} {
		want, _ := json.Marshal(before[field])
		got, _ := json.Marshal(after[field])
		if !bytes.Equal(got, want) {
			t.Fatalf("protocol field %s changed: %s != %s", field, got, want)
		}
	}
	if got := newNormalizer(t.TempDir()).norm(raw); got != raw {
		t.Fatalf("general normalization changed receipt/protocol text: %s", got)
	}
}

func TestProviderMetadataRejectsUnverifiedDocuments(t *testing.T) {
	for _, raw := range []string{
		`{"provider":{"name":"ce","capabilities":{},"version":"gz-git version 0.9.1"}}`,
		`{"provider":{"name":"gz-git","capabilities":true,"version":"gz-git version 0.9.1"}}`,
		`{"provider":{"name":"gz-git","capabilities":{},"version":"gz-git version 0.9.1"}} invalid`,
	} {
		if got := canonicalGZGitMetadataJSON(raw); got != raw {
			t.Fatalf("unverified metadata changed: %s != %s", got, raw)
		}
	}
}

func TestProviderVersionDiagnosticScope(t *testing.T) {
	for _, tc := range []struct {
		name, command, args, version string
		exit                         int
		change                       bool
	}{
		{"valid", "gz-git", `["--version"]`, "gz-git version 0.9.2\n", 0, true},
		{"failed", "gz-git", `["--version"]`, "gz-git version 0.9.2\n", 1, false},
		{"other-command", "ce", `["--version"]`, "gz-git version 0.9.2\n", 0, false},
		{"extra-argument", "gz-git", `["--version","--help"]`, "gz-git version 0.9.2\n", 0, false},
		{"help", "gz-git", `["integrate","--help"]`, "gz-git version 0.9.2\n", 0, false},
		{"invalid-version", "gz-git", `["--version"]`, "gz-git version missing\n", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]interface{}{"command": tc.command, "args": json.RawMessage(tc.args), "exitCode": tc.exit, "stdout": tc.version, "stderr": tc.version}
			data, err := json.Marshal(map[string]interface{}{"diagnostics": []interface{}{entry}})
			if err != nil {
				t.Fatal(err)
			}
			got := canonicalGZGitMetadataJSON(string(data))
			if !tc.change {
				if got != string(data) {
					t.Fatalf("non-identity diagnostic changed: %s", got)
				}
				return
			}
			entry["stdout"] = gzGitVersionPlaceholder + "\n"
			want, _ := json.Marshal(map[string]interface{}{"diagnostics": []interface{}{entry}})
			if got != string(want) {
				t.Fatalf("valid diagnostic mismatch: %s != %s", got, want)
			}
		})
	}
}

func TestProviderMetadataCanonicalizesOnlyGZGitVersion(t *testing.T) {
	first := canonicalGZGitMetadataJSON(`{"provider":{"name":"gz-git","version":"gz-git version 0.9.1","capabilities":{"integrate-check":true},"exitCode":0}}`)
	second := canonicalGZGitMetadataJSON(`{"provider":{"name":"gz-git","version":"gz-git version 0.9.2","capabilities":{"integrate-check":true},"exitCode":0}}`)
	if first != second {
		t.Fatalf("provider version drift was retained:\nfirst:  %s\nsecond: %s", first, second)
	}
	if want := `"version":"gz-git version \u003cGZ_GIT_VERSION\u003e"`; !strings.Contains(first, want) {
		t.Fatalf("normalized provider version missing %q: %s", want, first)
	}

	capabilityChanged := canonicalGZGitMetadataJSON(`{"provider":{"name":"gz-git","version":"gz-git version 0.9.2","capabilities":{"integrate-check":false},"exitCode":0}}`)
	if capabilityChanged == first {
		t.Fatal("provider capability change was hidden by version normalization")
	}
	if !strings.Contains(capabilityChanged, `"integrate-check":false`) || !strings.Contains(capabilityChanged, `"exitCode":0`) {
		t.Fatalf("normalization changed non-version provider facts: %s", capabilityChanged)
	}

	invalid := canonicalGZGitMetadataJSON(`{"command":"gz-git","args":["--version"],"exitCode":0,"stdout":"gz-git version missing\\n","reason":"gz-git version 0.9.2"}`)
	if strings.Contains(invalid, gzGitVersionPlaceholder) {
		t.Fatalf("invalid diagnostic or reason was normalized: %s", invalid)
	}
}

func TestIntegrateRunVersionIsMasked(t *testing.T) {
	n := newNormalizer(t.TempDir())
	for raw, want := range map[string]string{
		"integrate: run device=mbp gz-git=0.10.6\n":                  "integrate: run device=mbp gz-git=<VER>\n",
		`"stderr":"integrate: run device=mbp gz-git=0.11.0+dirty\n"`: `"stderr":"integrate: run device=mbp gz-git=<VER>\n"`,
		`\"integrate: run device=mbp gz-git=0.9.5-0.2026\\n\"`:       `\"integrate: run device=mbp gz-git=<VER>\\n\"`,
	} {
		if got := n.norm(raw); got != want {
			t.Errorf("norm(%q) = %q, want %q", raw, got, want)
		}
	}
	for _, raw := range []string{`{"gz-git":"0.10.6"}`, "gz-git version 0.10.6", "ungz-git=0.10.6"} {
		if got := n.norm(raw); got != raw {
			t.Errorf("non-provenance text changed: %q -> %q", raw, got)
		}
	}
}
