package parity

import "testing"

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
