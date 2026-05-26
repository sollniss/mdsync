package mdsync

import (
	"bytes"
	"testing"
)

func TestParseDirectiveBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *snippetRef
	}{
		{
			name:  "simple from",
			input: `from:file.go`,
			expected: &snippetRef{
				sourceFile: "file.go",
			},
		},
		{
			name:  "with language tag (colon prefix)",
			input: `:go from:file.go`,
			expected: &snippetRef{
				lang:       []byte("go"),
				sourceFile: "file.go",
			},
		},
		{
			name:  "with language tag (space separated)",
			input: `go from:file.go`,
			expected: &snippetRef{
				lang:       []byte("go"),
				sourceFile: "file.go",
			},
		},
		{
			name:  "quoted path",
			input: `from:"path/to/file.go"`,
			expected: &snippetRef{
				sourceFile: "path/to/file.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLexer([]byte(tt.input))
			r, err := parseDirective(l)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(r.lang, tt.expected.lang) {
				t.Errorf("expected lang %q, got %q", tt.expected.lang, r.lang)
			}
			if r.sourceFile != tt.expected.sourceFile {
				t.Errorf("expected sourceFile %q, got %q", tt.expected.sourceFile, r.sourceFile)
			}
		})
	}
}

func TestParseDirectiveFilters(t *testing.T) {
	input := `from:file.go skip-match:"^//" skip-after:"^import" count:2 start-at:"func main" start-offset:1 end-at:"^}" end-offset:0`
	l := newLexer([]byte(input))
	r, err := parseDirective(l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.sourceFile != "file.go" {
		t.Errorf("expected sourceFile 'file.go', got %q", r.sourceFile)
	}
	if len(r.skipMatch) != 1 {
		t.Fatalf("expected 1 skip-match, got %d", len(r.skipMatch))
	}
	if len(r.skipAfter) != 1 {
		t.Fatalf("expected 1 skip-after, got %d", len(r.skipAfter))
	}
	if r.skipAfter[0].count != 2 {
		t.Errorf("expected skip-after count 2, got %d", r.skipAfter[0].count)
	}
	if !bytes.Equal(r.startAt.literal, []byte("func main")) {
		t.Errorf("expected start-at 'func main', got %q", r.startAt.literal)
	}
	if r.startOffset != 1 {
		t.Errorf("expected start-offset 1, got %d", r.startOffset)
	}
	if !bytes.Equal(r.endAt.literal, []byte("^}")) {
		t.Errorf("expected end-at '^}', got %q", r.endAt.literal)
	}
	if r.endOffset != 0 {
		t.Errorf("expected end-offset 0, got %d", r.endOffset)
	}
}

func TestParseDirectiveSkipBetween(t *testing.T) {
	input := `from:file.go skip-between:"START":"END"`
	l := newLexer([]byte(input))
	r, err := parseDirective(l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.skipBetween) != 1 {
		t.Fatalf("expected 1 skip-between, got %d", len(r.skipBetween))
	}
	if !bytes.Equal(r.skipBetween[0].start.literal, []byte("START")) {
		t.Errorf("expected skip-between start 'START', got %q", r.skipBetween[0].start.literal)
	}
	if !bytes.Equal(r.skipBetween[0].end.literal, []byte("END")) {
		t.Errorf("expected skip-between end 'END', got %q", r.skipBetween[0].end.literal)
	}
}

func TestParseRefAttributeValueContainsGT(t *testing.T) {
	// Bug: IndexByte('>') found '>' inside a quoted attribute value and incorrectly
	// stripped the body before the closing '-->', leaving '-->' visible to the lexer.
	comment := []byte(`<!-- MDSYNC:go from:file.go skip-match:"ptr->field" -->`)
	r, err := parseRef(comment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.sourceFile != "file.go" {
		t.Errorf("expected sourceFile 'file.go', got %q", r.sourceFile)
	}
	if len(r.skipMatch) != 1 {
		t.Fatalf("expected 1 skip-match, got %d", len(r.skipMatch))
	}
	if !bytes.Equal(r.skipMatch[0].literal, []byte("ptr->field")) {
		t.Errorf("expected skip-match literal 'ptr->field', got %q", r.skipMatch[0].literal)
	}
}

func TestParseDirectiveTabSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		tabSize int
		wantErr bool
	}{
		{
			name:    "explicit 4",
			input:   `from:file.go tab-size:4`,
			tabSize: 4,
		},
		{
			name:    "zero strips",
			input:   `from:file.go tab-size:0`,
			tabSize: 0,
		},
		{
			name:    "negative disables",
			input:   `from:file.go tab-size:-1`,
			tabSize: -1,
		},
		{
			name:    "default when absent",
			input:   `from:file.go`,
			tabSize: 2,
		},
		{
			name:    "combined with other attributes",
			input:   `from:file.go start-at:"func main" tab-size:4 end-at:"^}"`,
			tabSize: 4,
		},
		{
			name:    "malformed value",
			input:   `from:file.go tab-size:abc`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLexer([]byte(tt.input))
			r, err := parseDirective(l)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.tabSize != tt.tabSize {
				t.Errorf("expected tabSize %d, got %d", tt.tabSize, r.tabSize)
			}
		})
	}
}

func TestParseDirectiveRegion(t *testing.T) {
	input := `from:file.go region:myFunc`
	l := newLexer([]byte(input))
	r, err := parseDirective(l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.startAt == nil || string(r.startAt.literal) != `#region myFunc\s*$` {
		t.Errorf("expected startAt '#region myFunc\\s*$', got %v", r.startAt)
	}
	if r.endAt == nil || string(r.endAt.literal) != `#endregion\s*$` {
		t.Errorf("expected endAt '#endregion\\s*$', got %v", r.endAt)
	}
}

func TestCompiledPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		line    string
		matches bool
	}{
		{
			name:    "literal match",
			pattern: "func main",
			line:    "func main() {",
			matches: true,
		},
		{
			name:    "literal no match",
			pattern: "func test",
			line:    "func main() {",
			matches: false,
		},
		{
			name:    "regex match start anchor",
			pattern: "^func",
			line:    "func main() {",
			matches: true,
		},
		{
			name:    "regex no match start anchor",
			pattern: "^func",
			line:    "  func main() {",
			matches: false,
		},
		{
			name:    "regex match end anchor",
			pattern: "}$",
			line:    "}",
			matches: true,
		},
		{
			name:    "regex character class",
			pattern: "[0-9]+",
			line:    "line 123",
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := compilePattern([]byte(tt.pattern))
			result := cp.matches([]byte(tt.line))
			if result != tt.matches {
				t.Errorf("expected matches=%v, got %v", tt.matches, result)
			}
		})
	}
}
