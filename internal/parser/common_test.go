package parser

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// TestParseError tests ParseError type
func TestParseError(t *testing.T) {
	tests := []struct {
		name    string
		err     *ParseError
		wantMsg string
	}{
		{
			name: "basic error",
			err: &ParseError{
				Line:    5,
				Content: "invalid line",
				Reason:  "unexpected format",
			},
			wantMsg: `parse error at line 5: unexpected format (content: "invalid line")`,
		},
		{
			name: "error with cause",
			err: &ParseError{
				Line:   2,
				Reason: "parsing failed",
				Cause:  errors.New("underlying error"),
			},
			wantMsg: "parse error at line 2: parsing failed: underlying error",
		},
		{
			name: "error without content",
			err: &ParseError{
				Line:   0,
				Reason: "empty line",
			},
			wantMsg: "parse error at line 0: empty line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// TestParseErrorIs tests ParseError.Is method
func TestParseErrorIs(t *testing.T) {
	err1 := &ParseError{Line: 1, Reason: "test"}
	err2 := &ParseError{Line: 2, Reason: "test2"}

	if !err1.Is(err2) {
		t.Error("ParseError.Is() should return true for another ParseError")
	}

	if err1.Is(errors.New("other error")) {
		t.Error("ParseError.Is() should return false for non-ParseError")
	}
}

// TestSplitLines tests line splitting
func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "empty string",
			text: "",
			want: []string{},
		},
		{
			name: "single line",
			text: "line 1",
			want: []string{"line 1"},
		},
		{
			name: "multiple lines (LF)",
			text: "line 1\nline 2\nline 3",
			want: []string{"line 1", "line 2", "line 3"},
		},
		{
			name: "multiple lines (CRLF)",
			text: "line 1\r\nline 2\r\nline 3",
			want: []string{"line 1", "line 2", "line 3"},
		},
		{
			name: "trailing newline",
			text: "line 1\nline 2\n",
			want: []string{"line 1", "line 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitLines(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseKeyValue tests key-value parsing
func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		separator string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "colon separator",
			line:      "key: value",
			separator: ":",
			wantKey:   "key",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name:      "equals separator",
			line:      "key=value",
			separator: "=",
			wantKey:   "key",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name:      "with whitespace",
			line:      "  key  :  value  ",
			separator: ":",
			wantKey:   "key",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name:      "value with separator",
			line:      "url: https://example.com",
			separator: ":",
			wantKey:   "url",
			wantValue: "https://example.com",
			wantErr:   false,
		},
		{
			name:      "no separator",
			line:      "invalid line",
			separator: ":",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, err := ParseKeyValue(tt.line, tt.separator)

			if tt.wantErr {
				if err == nil {
					t.Error("ParseKeyValue() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseKeyValue() unexpected error: %v", err)
				return
			}

			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}

			if value != tt.wantValue {
				t.Errorf("value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

// TestParseInt tests integer parsing
func TestParseInt(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"positive number", "42", 42},
		{"negative number", "-10", -10},
		{"zero", "0", 0},
		{"with whitespace", "  123  ", 123},
		{"invalid", "abc", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInt(tt.s)
			if got != tt.want {
				t.Errorf("ParseInt(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

// TestParseBool tests boolean parsing
func TestParseBool(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"true", "true", true},
		{"True", "True", true},
		{"yes", "yes", true},
		{"YES", "YES", true},
		{"1", "1", true},
		{"false", "false", false},
		{"no", "no", false},
		{"0", "0", false},
		{"empty", "", false},
		{"invalid", "maybe", false},
		{"with whitespace", "  true  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBool(tt.s)
			if got != tt.want {
				t.Errorf("ParseBool(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// TestParseTimestamp tests timestamp parsing
func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want time.Time
	}{
		{
			name: "valid timestamp",
			s:    "1609459200",
			want: time.Unix(1609459200, 0),
		},
		{
			name: "zero timestamp",
			s:    "0",
			want: time.Unix(0, 0),
		},
		{
			name: "invalid timestamp",
			s:    "invalid",
			want: time.Time{},
		},
		{
			name: "empty string",
			s:    "",
			want: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTimestamp(tt.s)
			if !got.Equal(tt.want) {
				t.Errorf("ParseTimestamp(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// TestParseDate tests date parsing
func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		wantErr bool
	}{
		{
			name:    "Unix timestamp",
			s:       "1609459200",
			wantErr: false,
		},
		{
			name:    "RFC3339",
			s:       "2021-01-01T00:00:00Z",
			wantErr: false,
		},
		{
			name:    "Date only",
			s:       "2021-01-01",
			wantErr: false,
		},
		{
			name:    "Git format",
			s:       "Fri Jan 1 00:00:00 2021 +0000",
			wantErr: false,
		},
		{
			name:    "invalid date",
			s:       "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDate(tt.s)

			if tt.wantErr {
				if err == nil {
					t.Error("ParseDate() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDate() unexpected error: %v", err)
				return
			}

			if got.IsZero() {
				t.Error("ParseDate() returned zero time")
			}
		})
	}
}

// TestParseRef tests reference parsing
func TestParseRef(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantRef    string
		wantTarget string
	}{
		{
			name:       "simple ref",
			line:       "refs/heads/main",
			wantRef:    "refs/heads/main",
			wantTarget: "",
		},
		{
			name:       "symbolic ref",
			line:       "HEAD -> refs/heads/main",
			wantRef:    "HEAD",
			wantTarget: "refs/heads/main",
		},
		{
			name:       "ref with whitespace",
			line:       "  refs/heads/develop  ->  refs/remotes/origin/develop  ",
			wantRef:    "refs/heads/develop",
			wantTarget: "refs/remotes/origin/develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, target := ParseRef(tt.line)

			if ref != tt.wantRef {
				t.Errorf("ref = %q, want %q", ref, tt.wantRef)
			}

			if target != tt.wantTarget {
				t.Errorf("target = %q, want %q", target, tt.wantTarget)
			}
		})
	}
}

// TestParseCommitHash tests commit hash parsing
func TestParseCommitHash(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    string
		wantErr bool
	}{
		{
			name:    "full SHA-1 (40 chars)",
			s:       "abcdef0123456789abcdef0123456789abcdef01",
			want:    "abcdef0123456789abcdef0123456789abcdef01",
			wantErr: false,
		},
		{
			name:    "abbreviated (7 chars)",
			s:       "abcdef0",
			want:    "abcdef0",
			wantErr: false,
		},
		{
			name:    "abbreviated (10 chars)",
			s:       "abcdef0123",
			want:    "abcdef0123",
			wantErr: false,
		},
		{
			name:    "with whitespace",
			s:       "  abc1234  ",
			want:    "abc1234",
			wantErr: false,
		},
		{
			name:    "uppercase",
			s:       "ABCDEF0",
			want:    "ABCDEF0",
			wantErr: false,
		},
		{
			name:    "too short (6 chars)",
			s:       "abcdef",
			wantErr: true,
		},
		{
			name:    "non-hex characters",
			s:       "ghijklm",
			wantErr: true,
		},
		{
			name:    "empty string",
			s:       "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommitHash(tt.s)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCommitHash(%q) expected error, got nil", tt.s)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseCommitHash(%q) unexpected error: %v", tt.s, err)
				return
			}

			if got != tt.want {
				t.Errorf("ParseCommitHash(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

// TestParseFileMode tests file mode parsing
func TestParseFileMode(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    int
		wantErr bool
	}{
		{
			name:    "regular file",
			s:       "100644",
			want:    0o100644,
			wantErr: false,
		},
		{
			name:    "executable file",
			s:       "100755",
			want:    0o100755,
			wantErr: false,
		},
		{
			name:    "directory",
			s:       "040000",
			want:    0o040000,
			wantErr: false,
		},
		{
			name:    "with whitespace",
			s:       "  100644  ",
			want:    0o100644,
			wantErr: false,
		},
		{
			name:    "invalid mode",
			s:       "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFileMode(tt.s)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFileMode(%q) expected error, got nil", tt.s)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseFileMode(%q) unexpected error: %v", tt.s, err)
				return
			}

			if got != tt.want {
				t.Errorf("ParseFileMode(%q) = %o, want %o", tt.s, got, tt.want)
			}
		})
	}
}

// TestIsEmptyLine tests empty line detection
func TestIsEmptyLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"empty string", "", true},
		{"spaces only", "   ", true},
		{"tabs only", "\t\t", true},
		{"mixed whitespace", " \t \n ", true},
		{"with content", "content", false},
		{"content with spaces", "  content  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEmptyLine(tt.line)
			if got != tt.want {
				t.Errorf("IsEmptyLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// TestTrimPrefix tests prefix trimming
func TestTrimPrefix(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		prefix string
		want   string
	}{
		{
			name:   "with prefix",
			s:      "prefix: value",
			prefix: "prefix:",
			want:   "value",
		},
		{
			name:   "without prefix",
			s:      "value",
			prefix: "prefix:",
			want:   "value",
		},
		{
			name:   "with whitespace",
			s:      "prefix:   value  ",
			prefix: "prefix:",
			want:   "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimPrefix(tt.s, tt.prefix)
			if got != tt.want {
				t.Errorf("TrimPrefix(%q, %q) = %q, want %q", tt.s, tt.prefix, got, tt.want)
			}
		})
	}
}

// TestSplitFields tests field splitting
func TestSplitFields(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "simple fields",
			line: "field1 field2 field3",
			want: []string{"field1", "field2", "field3"},
		},
		{
			name: "quoted field",
			line: `field1 "quoted field" field3`,
			want: []string{"field1", "quoted field", "field3"},
		},
		{
			name: "escaped quote",
			line: `field1 "quoted \"field\"" field3`,
			want: []string{"field1", `quoted "field"`, "field3"},
		},
		{
			name: "multiple spaces",
			line: "field1    field2    field3",
			want: []string{"field1", "field2", "field3"},
		},
		{
			name: "tabs",
			line: "field1\tfield2\tfield3",
			want: []string{"field1", "field2", "field3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitFields(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitFields(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// TestExtractBetween tests text extraction
func TestExtractBetween(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		start string
		end   string
		want  string
	}{
		{
			name:  "basic extraction",
			s:     "before [content] after",
			start: "[",
			end:   "]",
			want:  "content",
		},
		{
			name:  "no start delimiter",
			s:     "before content] after",
			start: "[",
			end:   "]",
			want:  "",
		},
		{
			name:  "no end delimiter",
			s:     "before [content after",
			start: "[",
			end:   "]",
			want:  "",
		},
		{
			name:  "empty content",
			s:     "before [] after",
			start: "[",
			end:   "]",
			want:  "",
		},
		{
			name:  "multi-character delimiters",
			s:     "before <<content>> after",
			start: "<<",
			end:   ">>",
			want:  "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBetween(tt.s, tt.start, tt.end)
			if got != tt.want {
				t.Errorf("ExtractBetween(%q, %q, %q) = %q, want %q",
					tt.s, tt.start, tt.end, got, tt.want)
			}
		})
	}
}
