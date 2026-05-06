package mdsync

import (
	"fmt"
	"strconv"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokString
	tokColon
)

type token struct {
	kind tokenKind
	val  []byte
	pos  int
}

type lexer struct {
	input []byte
	pos   int
}

func newLexer(input []byte) *lexer {
	return &lexer{input: input}
}

func isDelimiter(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == ':' || ch == '"'
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		switch ch {
		case ' ', '\t', '\n', '\r':
			l.pos++
		case ':':
			tok := token{kind: tokColon, pos: l.pos}
			l.pos++
			return tok, nil
		case '"':
			start := l.pos
			l.pos++
			j := l.pos
			for j < len(l.input) && l.input[j] != '"' {
				j++
			}
			if j >= len(l.input) {
				return token{}, fmt.Errorf("unterminated string at position %d", start)
			}
			tok := token{kind: tokString, val: l.input[l.pos:j], pos: start}
			l.pos = j + 1
			return tok, nil
		default:
			start := l.pos
			for l.pos < len(l.input) && !isDelimiter(l.input[l.pos]) {
				l.pos++
			}
			return token{kind: tokIdent, val: l.input[start:l.pos], pos: start}, nil
		}
	}
	return token{kind: tokEOF}, nil
}

func (l *lexer) peek() (token, error) {
	savedPos := l.pos
	tok, err := l.next()
	l.pos = savedPos
	return tok, err
}

func (l *lexer) expectValue() ([]byte, error) {
	tok, err := l.next()
	if err != nil {
		return nil, err
	}
	if tok.kind == tokEOF {
		return nil, fmt.Errorf("expected value, got end of input")
	}
	if tok.kind == tokIdent || tok.kind == tokString {
		return tok.val, nil
	}
	return nil, fmt.Errorf("expected value at position %d, got ':'", tok.pos)
}

func (l *lexer) expectString() ([]byte, error) {
	tok, err := l.next()
	if err != nil {
		return nil, err
	}
	if tok.kind == tokEOF {
		return nil, fmt.Errorf("expected quoted string, got end of input")
	}
	if tok.kind == tokString {
		return tok.val, nil
	}
	if tok.kind == tokIdent {
		return nil, fmt.Errorf("expected quoted string at position %d, got unquoted %q (wrap in double quotes)", tok.pos, tok.val)
	}
	return nil, fmt.Errorf("expected quoted string at position %d", tok.pos)
}

func (l *lexer) expectInt() (int, error) {
	tok, err := l.next()
	if err != nil {
		return 0, err
	}
	if tok.kind == tokEOF {
		return 0, fmt.Errorf("expected integer, got end of input")
	}
	if tok.kind != tokIdent {
		return 0, fmt.Errorf("expected integer at position %d", tok.pos)
	}
	n, err := strconv.Atoi(string(tok.val))
	if err != nil {
		return 0, fmt.Errorf("expected integer at position %d, got %q", tok.pos, tok.val)
	}
	return n, nil
}
