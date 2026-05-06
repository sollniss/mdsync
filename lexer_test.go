package mdsync

import (
	"bytes"
	"testing"
)

func TestLexerBasic(t *testing.T) {
	tests := []struct {
		input    string
		expected []token
	}{
		{
			input: `go from:file.go`,
			expected: []token{
				{kind: tokIdent, val: []byte("go")},
				{kind: tokIdent, val: []byte("from")},
				{kind: tokColon},
				{kind: tokIdent, val: []byte("file.go")},
				{kind: tokEOF},
			},
		},
		{
			input: `:go from:"path/to/file.go"`,
			expected: []token{
				{kind: tokColon},
				{kind: tokIdent, val: []byte("go")},
				{kind: tokIdent, val: []byte("from")},
				{kind: tokColon},
				{kind: tokString, val: []byte("path/to/file.go")},
				{kind: tokEOF},
			},
		},
		{
			input: `skip-match:"^//"`,
			expected: []token{
				{kind: tokIdent, val: []byte("skip-match")},
				{kind: tokColon},
				{kind: tokString, val: []byte("^//")},
				{kind: tokEOF},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := newLexer([]byte(tt.input))
			for i, expected := range tt.expected {
				tok, err := l.next()
				if err != nil {
					t.Fatalf("unexpected error at token %d: %v", i, err)
				}
				if tok.kind != expected.kind {
					t.Errorf("token %d: expected kind %v, got %v", i, expected.kind, tok.kind)
				}
				if !bytes.Equal(tok.val, expected.val) {
					t.Errorf("token %d: expected val %q, got %q", i, expected.val, tok.val)
				}
			}
		})
	}
}

func TestLexerUnterminatedString(t *testing.T) {
	l := newLexer([]byte(`"unterminated`))
	_, err := l.next()
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}
